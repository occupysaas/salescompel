package platform

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// sObject Collections routes and request validation. spec: col-01, col-02

const (
	maxCollectionRecords   = 200
	maxCollectionChunks    = 10
	maxRetrievePostIDs     = 2000
	maxRetrieveGetIDs      = 800
	collectionsDefaultPage = 2000
)

// collectionsRoute classifies /composite/sobjects... for the standalone API.
func collectionsRoute(method string, parts []string) (subRoute, []string) {
	route := subRoute{kind: routeUnsupported}
	switch {
	case len(parts) == 2:
		route.kind = map[string]routeKind{"POST": routeCollCreate, "PATCH": routeCollUpdate, "DELETE": routeCollDelete}[method]
		return route, []string{"POST", "PATCH", "DELETE"}
	case len(parts) == 3:
		route.object = parts[2]
		route.kind = map[string]routeKind{"GET": routeCollRetrieveGet, "POST": routeCollRetrievePost}[method]
		return route, []string{"GET", "POST"}
	case len(parts) == 4 && parts[2] == "Account" && strings.EqualFold(parts[3], "Compat_Key__c"):
		route.object = "Account"
		route.kind = map[string]routeKind{"PATCH": routeCollUpsert}[method]
		return route, []string{"PATCH"}
	}
	return route, nil
}

// serveCollections handles /services/data/v65.0/composite/sobjects...
func serveCollections(s *Store, ctx context.Context, p Principal, w http.ResponseWriter, r *http.Request, parts []string) {
	route, methods := collectionsRoute(r.Method, parts)
	if methods == nil {
		writeError(w, problem(404, "NOT_FOUND", "Resource not implemented by this candidate"))
		return
	}
	if route.kind == routeUnsupported {
		method(w, r, methods...)
		return
	}
	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, invalid("INVALID_QUERY_PARAMETER", "Invalid query parameter encoding"))
		return
	}
	var data map[string]any
	switch route.kind {
	case routeCollDelete, routeCollRetrieveGet:
		raw, _ := io.ReadAll(r.Body)
		if strings.TrimSpace(string(raw)) != "" {
			writeError(w, compositeInvalid())
			return
		}
	default:
		raw, readErr := io.ReadAll(r.Body)
		if readErr != nil || decodeExact(raw, &data) != nil || data == nil {
			writeError(w, compositeJSONError())
			return
		}
	}
	for k := range params {
		if !collectionsParamAllowed(route.kind, k) {
			writeError(w, compositeInvalid())
			return
		}
	}
	if err := validateCollectionsRequest(route, data, params); err != nil {
		writeError(w, err)
		return
	}
	var status int
	var out any
	err = s.within(ctx, p, func(st *Store, p Principal) error {
		var e error
		status, out, e = st.collections(ctx, p, route, params, data, false)
		return e
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, status, out)
}

func collectionsParamAllowed(kind routeKind, name string) bool {
	switch kind {
	case routeCollDelete:
		return name == "ids" || name == "allOrNone"
	case routeCollRetrieveGet:
		return name == "ids" || name == "fields"
	}
	return false
}

// validateCollectionsSubrequest prevalidates a Collections subrequest inside
// Composite. Values that hold references are checked after substitution.
func validateCollectionsSubrequest(route subRoute, body map[string]any) error {
	if !route.isCollections() {
		return nil
	}
	return validateCollectionsRequest(route, body, nil)
}

// validateCollectionsRequest checks the request shape. params is nil inside
// Composite, where query values are checked at run time.
func validateCollectionsRequest(route subRoute, data map[string]any, params url.Values) error {
	switch route.kind {
	case routeCollCreate, routeCollUpdate, routeCollUpsert:
		return validateCollectionsWrite(route.kind, data)
	case routeCollRetrievePost:
		for k := range data {
			if k != "ids" && k != "fields" {
				return compositeInvalid()
			}
		}
		ids, ok1 := data["ids"].([]any)
		fields, ok2 := data["fields"].([]any)
		if !ok1 || !ok2 || len(ids) < 1 || len(ids) > maxRetrievePostIDs || len(fields) < 1 {
			return compositeInvalid()
		}
		for _, v := range append(append([]any{}, ids...), fields...) {
			if _, ok := v.(string); !ok {
				return compositeInvalid()
			}
		}
	case routeCollRetrieveGet:
		if params != nil {
			return checkRetrieveGetParams(params.Get("ids"), params.Get("fields"), params.Has("ids"), params.Has("fields"))
		}
	case routeCollDelete:
		if params != nil {
			return checkDeleteParams(params.Get("ids"), params.Get("allOrNone"), params.Has("ids"), params.Has("allOrNone"))
		}
	}
	return nil
}

func checkRetrieveGetParams(ids, fields string, hasIDs, hasFields bool) error {
	n := len(splitIDs(ids))
	if !hasIDs || !hasFields || n < 1 || n > maxRetrieveGetIDs || len(splitFields(fields)) < 1 {
		return compositeInvalid()
	}
	return nil
}

func checkDeleteParams(ids, allOrNone string, hasIDs, hasAllOrNone bool) error {
	n := len(splitIDs(ids))
	if !hasIDs || n < 1 || n > maxCollectionRecords {
		return compositeInvalid()
	}
	if hasAllOrNone && allOrNone != "true" && allOrNone != "false" {
		return compositeInvalid()
	}
	return nil
}

func splitIDs(raw string) []string {
	out := []string{}
	for _, id := range strings.Split(raw, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func validateCollectionsWrite(kind routeKind, data map[string]any) error {
	for k := range data {
		if k != "allOrNone" && k != "records" {
			return compositeInvalid()
		}
	}
	if v, ok := data["allOrNone"]; ok {
		if _, isBool := v.(bool); !isBool {
			return compositeInvalid()
		}
	}
	records, ok := data["records"].([]any)
	if !ok || len(records) < 1 || len(records) > maxCollectionRecords {
		return compositeInvalid()
	}
	chunks, previous := 0, ""
	for _, item := range records {
		record, ok := item.(map[string]any)
		if !ok {
			return compositeInvalid()
		}
		typ, ok := recordType(record)
		if !ok {
			return compositeInvalid()
		}
		if !strings.EqualFold(typ, previous) {
			chunks++
			previous = typ
		}
		idKeys := 0
		for k := range record {
			if strings.EqualFold(k, "id") {
				idKeys++
			}
		}
		switch kind {
		case routeCollCreate:
			if idKeys != 0 {
				return compositeInvalid()
			}
		case routeCollUpdate:
			if idKeys != 1 {
				return compositeInvalid()
			}
		case routeCollUpsert:
			if idKeys != 0 || !strings.EqualFold(typ, "Account") {
				return compositeInvalid()
			}
		}
	}
	if kind != routeCollUpsert && chunks > maxCollectionChunks {
		return compositeInvalid()
	}
	return nil
}

// recordType returns attributes.type, which must be a literal object name.
func recordType(record map[string]any) (string, bool) {
	attributes, ok := record["attributes"].(map[string]any)
	if !ok {
		return "", false
	}
	typ, ok := attributes["type"].(string)
	return typ, ok && compositeName.MatchString(typ)
}
