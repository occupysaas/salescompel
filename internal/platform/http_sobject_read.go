package platform

import (
	"context"
	"net/http"
)

// serveRecord handles GET, PATCH and DELETE on /sobjects/{object}/{id}.
func serveRecord(s *Store, ctx context.Context, p Principal, w http.ResponseWriter, r *http.Request, object, id string) {
	if !method(w, r, "GET", "PATCH", "DELETE") {
		return
	}
	switch r.Method {
	case "GET":
		row, e := s.Read(ctx, p, object, id)
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
