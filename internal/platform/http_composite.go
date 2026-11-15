package platform

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

var compositeRefID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_]{0,79}$`)

const (
	maxCompositeSubrequests = 25
	maxCompositeQueries     = 5
)

func compositeInvalid() error {
	return invalid("INVALID_COMPOSITE_REQUEST", "Invalid or unsupported Composite request")
}
func compositeJSONError() error { return invalid("JSON_PARSER_ERROR", "Invalid Composite JSON body") }
func compositeRefIDError() error {
	return invalid("JSON_PARSER_ERROR", "Invalid referenceId: use letters, numbers and underscores")
}

// Raw maps preserve exact property spelling and distinguish absent and null.
// Field business validation stays in the transaction executor.
func decodeComposite(r io.Reader) (CompositeRequest, error) {
	var raw json.RawMessage
	d := json.NewDecoder(r)
	if err := d.Decode(&raw); err != nil {
		return CompositeRequest{}, compositeJSONError()
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return CompositeRequest{}, compositeJSONError()
	}
	fail := func() (CompositeRequest, error) { return CompositeRequest{}, compositeInvalid() }
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil || top == nil {
		return fail()
	}
	for k := range top {
		if k != "allOrNone" && k != "collateSubrequests" && k != "compositeRequest" {
			return fail()
		}
	}
	var req CompositeRequest
	if b, ok := top["allOrNone"]; ok {
		if !decodeBool(b, &req.AllOrNone) {
			return fail()
		}
	}
	if b, ok := top["collateSubrequests"]; ok {
		var v bool
		if !decodeBool(b, &v) {
			return fail()
		}
		req.CollateSubrequests = &v
	}
	var subs []map[string]json.RawMessage
	if json.Unmarshal(top["compositeRequest"], &subs) != nil || len(subs) < 1 || len(subs) > maxCompositeSubrequests {
		return fail()
	}
	for _, rawSub := range subs {
		var ref string
		if rawSub != nil && (json.Unmarshal(rawSub["referenceId"], &ref) != nil || ref == "") {
			return CompositeRequest{}, compositeRefIDError()
		}
		sub, ok := decodeSubrequest(rawSub)
		if !ok {
			return fail()
		}
		req.CompositeRequest = append(req.CompositeRequest, sub)
	}
	if _, err := planComposite(req); err != nil {
		return CompositeRequest{}, err
	}
	return req, nil
}

func decodeBool(b json.RawMessage, v *bool) bool {
	switch string(b) {
	case "true":
		*v = true
	case "false":
		*v = false
	default:
		return false
	}
	return true
}

func decodeSubrequest(raw map[string]json.RawMessage) (CompositeSubrequest, bool) {
	sub := CompositeSubrequest{}
	if raw == nil {
		return sub, false
	}
	for k := range raw {
		switch k {
		case "method", "url", "referenceId", "body", "httpHeaders":
		default:
			return sub, false
		}
	}
	for key, target := range map[string]*string{"method": &sub.Method, "url": &sub.URL, "referenceId": &sub.ReferenceID} {
		if json.Unmarshal(raw[key], target) != nil || *target == "" {
			return sub, false
		}
	}
	if h, ok := raw["httpHeaders"]; ok {
		if json.Unmarshal(h, &sub.HTTPHeaders) != nil || sub.HTTPHeaders == nil {
			return sub, false
		}
	}
	b, present := raw["body"]
	switch sub.Method {
	case "GET", "DELETE":
		return sub, !present
	}
	if !present || decodeExact(b, &sub.Body) != nil || sub.Body == nil {
		return sub, false
	}
	return sub, true
}

// subPlan is the prevalidated form of one subrequest.
type subPlan struct {
	route             subRoute
	refs              []*reference // every reference in URL and body
	headerUnsupported bool
}

func validateComposite(req CompositeRequest) error {
	_, err := planComposite(req)
	return err
}

// planComposite checks the whole request before any transaction begins.
// spec: cmp-01, cmp-03, cmp-04
func planComposite(req CompositeRequest) ([]subPlan, error) {
	if len(req.CompositeRequest) < 1 || len(req.CompositeRequest) > maxCompositeSubrequests {
		return nil, compositeInvalid()
	}
	plans := make([]subPlan, len(req.CompositeRequest))
	earlier := map[string]bool{}
	queries := 0
	for i, sub := range req.CompositeRequest {
		if !compositeRefID.MatchString(sub.ReferenceID) {
			return nil, compositeRefIDError()
		}
		if earlier[sub.ReferenceID] {
			return nil, compositeInvalid()
		}
		switch sub.Method {
		case "POST", "PATCH":
			if sub.Body == nil {
				return nil, compositeInvalid()
			}
		case "GET", "DELETE":
			if sub.Body != nil {
				return nil, compositeInvalid()
			}
		default:
			return nil, compositeInvalid()
		}
		for name := range sub.HTTPHeaders {
			switch strings.ToLower(name) {
			case "accept", "authorization", "content-type":
				return nil, compositeInvalid()
			}
			plans[i].headerUnsupported = true
		}
		route, ok := parseSubrequestURL(sub.Method, sub.URL)
		if !ok {
			return nil, compositeInvalid()
		}
		if route.countsTowardQueryLimit() {
			if queries++; queries > maxCompositeQueries {
				return nil, compositeInvalid()
			}
		}
		refs, ok := bodyRefs(map[string]any(sub.Body))
		if !ok {
			return nil, compositeInvalid()
		}
		refs = append(refs, route.refs()...)
		for _, ref := range refs {
			if !earlier[ref.id] {
				return nil, compositeInvalid()
			}
		}
		if err := validateCollectionsSubrequest(route, sub.Body); err != nil {
			return nil, err
		}
		plans[i].route, plans[i].refs = route, refs
		earlier[sub.ReferenceID] = true
	}
	return plans, nil
}

// decodeExactOnly is decodeExact that also rejects trailing data.
func decodeExactOnly(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(v); err != nil {
		return err
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// decodeExact decodes JSON keeping numbers as json.Number, so field values
// never pass through binary floating point.
func decodeExact(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	return d.Decode(v)
}
