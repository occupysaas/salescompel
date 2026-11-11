package platform

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHTTPScopeEnforced(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	h := NewHandler(s, "")
	raw := "scope-provisioned-" + g.User.Tenant
	if e := s.Provision(ctx, raw, Principal{Tenant: g.User.Tenant, User: "admin", Role: "admin"}); e != nil {
		t.Fatal(e)
	}
	for _, scope := range []string{"id", "web", "api", "", "provisioned"} {
		t.Run(scope, func(t *testing.T) {
			token := raw
			if scope != "provisioned" {
				g.Scope = scope
				g.Refresh = false
				token = issueTest(t, s, g, n).Access
			}
			for _, path := range []string{"/services/data/v65.0/sobjects/Account/describe", "/services/data/v65.0/openapi.yaml", "/services/oauth2/userinfo"} {
				want := 200
				if scope != "api" && scope != "provisioned" && (path != "/services/oauth2/userinfo" || scope != "id") {
					want = 403
				}
				r := httptest.NewRequest("GET", path, nil)
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != want {
					t.Fatalf("%s got %d: %s", path, w.Code, w.Body.String())
				}
				if want == 403 {
					var envelope []struct {
						ErrorCode string `json:"errorCode"`
					}
					if e := json.Unmarshal(w.Body.Bytes(), &envelope); e != nil || len(envelope) != 1 || envelope[0].ErrorCode != "INSUFFICIENT_ACCESS" {
						t.Fatal(w.Body.String(), e)
					}
				}
			}
		})
	}
}
