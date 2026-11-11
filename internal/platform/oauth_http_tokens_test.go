package platform

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestRevokeAndIntrospectHTTP(t *testing.T) {
	s, u, id, sec, n := httpApp(t, []string{"client_credentials"})
	v, e := s.IssueTokens(context.Background(), TokenGrant{ClientID: id, User: u, Scope: "api"}, n, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	w, m := oauthHTTP(t, s, n, "/services/oauth2/introspect", url.Values{"token": {v.Access}})
	if m["error"] != "invalid_client" {
		t.Fatal(w.Code, m)
	}
	w, m = oauthHTTP(t, s, n, "/services/oauth2/introspect", url.Values{"token": {v.Access}}, id, sec)
	if w.Code != 200 || m["active"] != true || m["client_id"] != id || m["username"] != u.Username || m["exp"] == nil || m["iat"] == nil {
		t.Fatal(w.Code, m)
	}
	w, _ = oauthHTTP(t, s, n, "/services/oauth2/revoke", url.Values{"token": {v.Access}})
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("GET", apiBase+"sobjects", nil)
	r.Header.Set("Authorization", "Bearer "+v.Access)
	w = httptest.NewRecorder()
	NewHandler(s, "").ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	w, m = oauthHTTP(t, s, n, "/services/oauth2/revoke", url.Values{"token": {v.Access}})
	if w.Code != 400 || m["error"] != "invalid_token" {
		t.Fatal(w.Code, m)
	}
}

func TestTokenClientCredentialsScope(t *testing.T) {
	s, _, id, secret, n := httpApp(t, []string{"client_credentials"})
	for _, tc := range []struct{ scope, want string }{{"api", "api"}, {"refresh_token", ""}, {"openid", "invalid_scope"}, {"web", "invalid_scope"}} {
		w, m := oauthHTTP(t, s, n, "/services/oauth2/token", url.Values{"grant_type": {"client_credentials"}, "client_id": {id}, "client_secret": {secret}, "scope": {tc.scope}})
		if tc.want == "invalid_scope" {
			if w.Code != 400 || m["error"] != tc.want {
				t.Fatal(w.Code, m)
			}
		} else if w.Code != 200 || m["scope"] != tc.want {
			t.Fatal(w.Code, m)
		}
	}
}

func TestTokenJWTReplayWindow(t *testing.T) {
	s, g, n := tokenFixture(t)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert := jwtTestCert(t, key, n)
	id, _, e := s.CreateApp(context.Background(), AppInput{Tenant: g.User.Tenant, Name: "jwt-window", Grants: []string{jwtGrant}, Scopes: []string{"api"}, JWTUsers: []string{g.User.Username}, JWTCertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))}, n)
	if e != nil {
		t.Fatal(e)
	}
	for i, at := range []time.Time{n, n.Add(6 * time.Minute)} {
		assertion := jwtTestSign(t, key, "RS256", map[string]any{"iss": id, "sub": g.User.Username, "aud": "https://crm.example.com", "exp": at.Unix() + 60, "jti": "reusable-after-expiry"})
		w, m := oauthHTTP(t, s, at, "/services/oauth2/token", url.Values{"grant_type": {jwtGrant}, "assertion": {assertion}})
		if w.Code != 200 {
			t.Fatal(i, w.Code, m)
		}
	}
}

func TestTokenInvalidScopeDoesNotConsumeGrant(t *testing.T) {
	s, u, id, secret, n := httpApp(t, []string{"authorization_code", "refresh_token"})
	code, e := s.IssueCode(context.Background(), CodeInput{ClientID: id, UserID: u.ID, RedirectURI: "https://callback.example.com", Scope: "api refresh_token"}, n)
	if e != nil {
		t.Fatal(e)
	}
	values := url.Values{"grant_type": {"authorization_code"}, "client_id": {id}, "client_secret": {secret}, "redirect_uri": {"https://callback.example.com"}, "code": {code}}
	for _, scope := range []string{"openid", "web"} {
		values.Set("scope", scope)
		w, m := oauthHTTP(t, s, n, "/services/oauth2/token", values)
		if w.Code != 400 || m["error"] != "invalid_scope" {
			t.Fatalf("code scope %s: %d %v", scope, w.Code, m)
		}
	}
	values.Set("scope", "api")
	w, m := oauthHTTP(t, s, n, "/services/oauth2/token", values)
	if w.Code != 200 || m["scope"] != "api refresh_token" {
		t.Fatal(w.Code, m)
	}
	// An allowed requested scope cannot widen the scope bound to the code.
	narrowCode, err := s.IssueCode(context.Background(), CodeInput{ClientID: id, UserID: u.ID, RedirectURI: "https://callback.example.com", Scope: "api"}, n)
	if err != nil {
		t.Fatal(err)
	}
	narrowValues := url.Values{"grant_type": {"authorization_code"}, "client_id": {id}, "client_secret": {secret}, "redirect_uri": {"https://callback.example.com"}, "code": {narrowCode}, "scope": {"refresh_token"}}
	narrowW, narrowM := oauthHTTP(t, s, n, "/services/oauth2/token", narrowValues)
	if narrowW.Code != 200 || narrowM["scope"] != "api" || narrowM["refresh_token"] != nil {
		t.Fatal(narrowW.Code, narrowM)
	}
	refresh := m["refresh_token"].(string)
	values = url.Values{"grant_type": {"refresh_token"}, "client_id": {id}, "client_secret": {secret}, "refresh_token": {refresh}}
	for _, scope := range []string{"openid", "web"} {
		values.Set("scope", scope)
		w, m = oauthHTTP(t, s, n, "/services/oauth2/token", values)
		if w.Code != 400 || m["error"] != "invalid_scope" {
			t.Fatalf("refresh scope %s: %d %v", scope, w.Code, m)
		}
	}
	values.Set("scope", "api")
	w, m = oauthHTTP(t, s, n, "/services/oauth2/token", values)
	if w.Code != 200 || m["scope"] != "api refresh_token" {
		t.Fatal(w.Code, m)
	}
}

func TestTokenInvalidScopeDoesNotConsumeJWT(t *testing.T) {
	s, g, n := tokenFixture(t)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert := jwtTestCert(t, key, n)
	id, _, e := s.CreateApp(context.Background(), AppInput{Tenant: g.User.Tenant, Name: "jwt-scope", Grants: []string{jwtGrant}, Scopes: []string{"api"}, JWTUsers: []string{g.User.Username}, JWTCertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))}, n)
	if e != nil {
		t.Fatal(e)
	}
	assertion := jwtTestSign(t, key, "RS256", map[string]any{"iss": id, "sub": g.User.Username, "aud": "https://crm.example.com", "exp": n.Unix() + 60, "jti": "scope-retry"})
	values := url.Values{"grant_type": {jwtGrant}, "assertion": {assertion}, "scope": {"openid"}}
	w, m := oauthHTTP(t, s, n, "/services/oauth2/token", values)
	if w.Code != 400 || m["error"] != "invalid_scope" {
		t.Fatal(w.Code, m)
	}
	values.Set("scope", "api")
	w, m = oauthHTTP(t, s, n, "/services/oauth2/token", values)
	if w.Code != 200 {
		t.Fatal(w.Code, m)
	}
}

func TestTokenJWTMillisecondReplayBoundary(t *testing.T) {
	s, g, n := tokenFixture(t)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert := jwtTestCert(t, key, n)
	id, _, e := s.CreateApp(context.Background(), AppInput{Tenant: g.User.Tenant, Name: "jwt-boundary", Grants: []string{jwtGrant}, Scopes: []string{"api"}, JWTUsers: []string{g.User.Username}, JWTCertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))}, n)
	if e != nil {
		t.Fatal(e)
	}
	assertion := jwtTestSign(t, key, "RS256", map[string]any{"iss": id, "sub": g.User.Username, "aud": "https://crm.example.com", "iat": n.Unix(), "exp": n.UnixMilli(), "jti": "boundary"})
	values := url.Values{"grant_type": {jwtGrant}, "assertion": {assertion}}
	w, m := oauthHTTP(t, s, n, "/services/oauth2/token", values)
	if w.Code != 200 {
		t.Fatal(w.Code, m)
	}
	var expires time.Time
	if e = s.pool.QueryRow(context.Background(), "SELECT expires_at FROM oauth_jwt_ids WHERE client_id=$1 AND jti='boundary'", id).Scan(&expires); e != nil {
		t.Fatal(e)
	}
	if !expires.Equal(n.Add(300 * time.Second)) {
		t.Fatalf("JWT ID retention ends %v, want %v", expires, n.Add(300*time.Second))
	}
	// A fresh assertion is accepted at the exact five-minute boundary.
	fresh := jwtTestSign(t, key, "RS256", map[string]any{"iss": id, "sub": g.User.Username, "aud": "https://crm.example.com", "iat": n.Unix(), "exp": n.UnixMilli(), "jti": "exact-boundary-fresh"})
	w, m = oauthHTTP(t, s, n.Add(300*time.Second), "/services/oauth2/token", url.Values{"grant_type": {jwtGrant}, "assertion": {fresh}})
	if w.Code != 200 {
		t.Fatal(w.Code, m)
	}
	// Once used it is rejected at the exact boundary; fractional time past
	// the boundary is invalid even for an assertion without an identifier.
	for _, at := range []time.Time{n.Add(300 * time.Second), n.Add(300*time.Second + 500*time.Millisecond)} {
		w, m = oauthHTTP(t, s, at, "/services/oauth2/token", values)
		if w.Code != 400 || m["error"] != "invalid_grant" {
			t.Fatal(at, w.Code, m)
		}
	}
	withoutID := jwtTestSign(t, key, "RS256", map[string]any{"iss": id, "sub": g.User.Username, "aud": "https://crm.example.com", "iat": n.Unix(), "exp": n.UnixMilli()})
	w, m = oauthHTTP(t, s, n.Add(300*time.Second+500*time.Millisecond), "/services/oauth2/token", url.Values{"grant_type": {jwtGrant}, "assertion": {withoutID}})
	if w.Code != 400 || m["error"] != "invalid_grant" {
		t.Fatal(w.Code, m)
	}

}
