package platform

import (
	"context"
	"net/http"
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

// requireAPIEnabled is the API Enabled gate. The handler applies it to every
// REST and Composite route under /services/data; the OAuth endpoints and the
// identity URL are not gated.
func requireAPIEnabled(p Principal, r *http.Request) error {
	if p.Perms.System().APIEnabled {
		return nil
	}
	if ui, _ := r.Context().Value(uiSessionKey).(bool); ui {
		return nil
	}
	// D33-07
	return problem(403, "API_CURRENTLY_DISABLED", "API is not enabled for this User")
}
