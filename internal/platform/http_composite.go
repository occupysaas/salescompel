package platform

import (
	"encoding/json"
	"io"
	"regexp"
	"salescompel/internal/metadata"
	"strings"
)

var compositeRefID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_]{0,79}$`)
var compositeRecordID = regexp.MustCompile(`^[A-Za-z0-9]{18}$`)
var compositeReference = regexp.MustCompile(`^@\{([A-Za-z0-9][A-Za-z0-9_]{0,79})\.id\}$`)
var compositeRoute = regexp.MustCompile(`^/services/data/v65\.0/sobjects/([A-Za-z][A-Za-z0-9_]*)(?:/([^/]+))?$`)

func compositeInvalid() error {
	return invalid("INVALID_COMPOSITE_REQUEST", "Invalid or unsupported Composite request")
}
func compositeJSONError() error { return invalid("JSON_PARSER_ERROR", "Invalid Composite JSON body") }

// Raw maps preserve exact property spelling and distinguish absent and null bodies.
// Field business validation intentionally remains in the transaction executor.
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
	if string(top["allOrNone"]) != "true" {
		return fail()
	}
	req.AllOrNone = true
	if b, ok := top["collateSubrequests"]; ok {
		if string(b) != "false" {
			return fail()
		}
		v := false
		req.CollateSubrequests = &v
	}
	var subs []map[string]json.RawMessage
	if json.Unmarshal(top["compositeRequest"], &subs) != nil || len(subs) < 1 || len(subs) > 25 {
		return fail()
	}
	for _, rawSub := range subs {
		if rawSub == nil {
			return fail()
		}
		for k := range rawSub {
			if k != "method" && k != "url" && k != "referenceId" && k != "body" {
				return fail()
			}
		}
		sub := CompositeSubrequest{}
		for key, target := range map[string]*string{"method": &sub.Method, "url": &sub.URL, "referenceId": &sub.ReferenceID} {
			if json.Unmarshal(rawSub[key], target) != nil || *target == "" {
				return fail()
			}
		}
		b, present := rawSub["body"]
		if sub.Method == "DELETE" {
			if present {
				return fail()
			}
		} else {
			if !present || json.Unmarshal(b, &sub.Body) != nil || sub.Body == nil {
				return fail()
			}
		}
		req.CompositeRequest = append(req.CompositeRequest, sub)
	}
	if err := validateComposite(req); err != nil {
		return CompositeRequest{}, err
	}
	return req, nil
}
func validateComposite(req CompositeRequest) error {
	if !req.AllOrNone || (req.CollateSubrequests != nil && *req.CollateSubrequests) || len(req.CompositeRequest) < 1 || len(req.CompositeRequest) > 25 {
		return compositeInvalid()
	}
	earlier := map[string]CompositeSubrequest{}
	for _, sub := range req.CompositeRequest {
		if !compositeRefID.MatchString(sub.ReferenceID) {
			return compositeInvalid()
		}
		if _, ok := earlier[sub.ReferenceID]; ok {
			return compositeInvalid()
		}
		route := compositeRoute.FindStringSubmatch(sub.URL)
		if route == nil || strings.ContainsAny(sub.URL, "%?#\\") {
			return compositeInvalid()
		}
		object, id := route[1], route[2]
		if object != "Account" && object != "Contact" && !metadata.ValidAPIName(object) {
			return compositeInvalid()
		}
		checkRef := func(value string, account bool) bool {
			match := compositeReference.FindStringSubmatch(value)
			if match == nil {
				return false
			}
			prev, ok := earlier[match[1]]
			return ok && prev.Method == "POST" && (!account || prev.URL == apiBase+"sobjects/Account")
		}
		switch sub.Method {
		case "POST":
			if id != "" || sub.Body == nil {
				return compositeInvalid()
			}
		case "PATCH", "DELETE":
			if !compositeRecordID.MatchString(id) && !checkRef(id, false) {
				return compositeInvalid()
			}
			if (sub.Method == "PATCH" && sub.Body == nil) || (sub.Method == "DELETE" && sub.Body != nil) {
				return compositeInvalid()
			}
		default:
			return compositeInvalid()
		}
		for key, value := range sub.Body {
			if strings.Contains(key, "@{") {
				return compositeInvalid()
			}
			if text, ok := value.(string); ok && strings.Contains(text, "@{") {
				if object != "Contact" || !strings.EqualFold(key, "AccountId") || !checkRef(text, true) {
					return compositeInvalid()
				}
			} else if containsCompositeMarker(value) {
				return compositeInvalid()
			}
		}
		earlier[sub.ReferenceID] = sub
	}
	return nil
}
func containsCompositeMarker(value any) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(v, "@{")
	case map[string]any:
		for k, x := range v {
			if strings.Contains(k, "@{") || containsCompositeMarker(x) {
				return true
			}
		}
	case []any:
		for _, x := range v {
			if containsCompositeMarker(x) {
				return true
			}
		}
	}
	return false
}
