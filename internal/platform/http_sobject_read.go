package platform

import (
	"context"
	"net/http"
	"strings"
)

// fieldsParam returns nil when the request has no fields parameter, and the
// comma-separated names otherwise (an empty value gives an empty list, which
// the record layer rejects).
// Malformed parameters other than fields are ignored, as before this change.
func fieldsParam(r *http.Request) ([]string, error) {
	raw, ok := r.URL.Query()["fields"]
	if !ok {
		return nil, nil
	}
	return splitFields(strings.Join(raw, ",")), nil
}

func splitFields(raw string) []string {
	out := []string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	return append(out, strings.Split(raw, ",")...)
}

// serveRecord handles GET, PATCH and DELETE on /sobjects/{object}/{id}.
func serveRecord(s *Store, ctx context.Context, p Principal, w http.ResponseWriter, r *http.Request, object, id string) {
	if !method(w, r, "GET", "PATCH", "DELETE") {
		return
	}
	switch r.Method {
	case "GET":
		fields, e := fieldsParam(r)
		if e != nil {
			writeError(w, e)
			return
		}
		row, e := s.ReadFields(ctx, p, object, id, fields)
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, 200, row)
	case "PATCH":
		data, e := body(r)
		if e != nil {
			writeError(w, e)
			return
		}
		if e = s.Update(ctx, p, object, id, data); e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, 204, nil)
	case "DELETE":
		if e := s.Delete(ctx, p, object, id); e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, 204, nil)
	}
}
