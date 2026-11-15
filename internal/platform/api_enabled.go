package platform

import (
	"context"
	"net/http"
	"strings"
)

type uiSessionKeyType struct{}

// uiSessionKey marks a request as coming from the first-party web UI
// session, which API Enabled does not restrict (D33-07). Only withUISession
// sets it. Nothing in production calls withUISession yet: Plan 36's sign-in
// creates the real session and replaces this stand-in. Tests set it through
// withUISession.
var uiSessionKey = uiSessionKeyType{}

// withUISession is the UI session middleware stand-in: it marks r as a
// first-party UI request.
func withUISession(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), uiSessionKey, true))
}

// requireAPIEnabled gates every REST and Composite route under
// /services/data. The OAuth endpoints and the identity URL are not gated.
func requireAPIEnabled(p Principal, r *http.Request) error {
	path := strings.TrimSuffix("/"+strings.TrimLeft(r.URL.EscapedPath(), "/"), "/")
	if path != "/services/data" && !strings.HasPrefix(path, "/services/data/") {
		return nil
	}
	if p.Perms.System().APIEnabled {
		return nil
	}
	if ui, _ := r.Context().Value(uiSessionKey).(bool); ui {
		return nil
	}
	// D33-07
	return problem(403, "API_CURRENTLY_DISABLED", "API is not enabled for this User")
}

// readFields is the REST record read's ?fields= list, comma separated.
func readFields(r *http.Request) []string {
	v := r.URL.Query().Get("fields")
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}
