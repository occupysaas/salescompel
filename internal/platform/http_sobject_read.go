package platform

import (
	"context"
	"net/http"
	"strings"
)

// restFields splits a REST ?fields= value the way the standalone record GET
// does (D33-03): nil when absent or empty, otherwise the comma-separated names.
func restFields(v string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
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
		row, e := s.Read(ctx, p, object, id, restFields(r.URL.Query().Get("fields"))...)
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
