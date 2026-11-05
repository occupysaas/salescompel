package platform

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestResolvePublicURL(t *testing.T) {
	for _, tc := range []struct {
		v, a string
		tls  bool
		want string
	}{{"", "127.0.0.1:8080", false, "http://127.0.0.1:8080"}, {"", "127.0.0.1:8080", true, "https://127.0.0.1:8080"}, {"https://crm.example.com/", "0.0.0.0:443", true, "https://crm.example.com"}, {"http://example.com", "127.0.0.1:8080", false, ""}, {"https://h/path", "", true, ""}, {"https://h?q=x", "", true, ""}, {"https://h#f", "", true, ""}, {"", "0.0.0.0:8080", true, ""}} {
		got, e := ResolvePublicURL(tc.v, tc.a, tc.tls)
		if got != tc.want || (e != nil) != (tc.want == "") {
			t.Fatalf("%+v: %q %v", tc, got, e)
		}
	}
}
func oauthHTTP(t *testing.T, s *Store, n time.Time, path string, v url.Values, basic ...string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest("POST", path, strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if len(basic) > 0 {
		r.SetBasicAuth(basic[0], basic[1])
	}
	w := httptest.NewRecorder()
	NewHandlerWithOptions(s, "", Options{PublicURL: "https://crm.example.com", Now: func() time.Time { return n }}).ServeHTTP(w, r)
	m := map[string]any{}
	json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}
func httpApp(t *testing.T, grants []string) (*Store, User, string, string, time.Time) {
	t.Helper()
	s, g, n := tokenFixture(t)
	id, sec, e := s.CreateApp(context.Background(), AppInput{Tenant: g.User.Tenant, Name: "http", Grants: grants, Scopes: []string{"api", "refresh_token"}, RedirectURIs: []string{"https://callback.example.com"}, RunAs: g.User.Username}, n)
	if e != nil {
		t.Fatal(e)
	}
	return s, g.User, id, sec, n
}
func TestTokenAuthorizationCode(t *testing.T) {
	s, u, id, sec, n := httpApp(t, []string{"authorization_code"})
	code, e := s.IssueCode(context.Background(), CodeInput{ClientID: id, UserID: u.ID, RedirectURI: "https://callback.example.com", Scope: "api refresh_token"}, n)
	if e != nil {
		t.Fatal(e)
	}
	v := url.Values{"grant_type": {"authorization_code"}, "client_id": {id}, "code": {code}, "redirect_uri": {"https://callback.example.com"}}
	w, m := oauthHTTP(t, s, n, "/services/oauth2/token", v)
	if w.Code != 400 || m["error"] != "invalid_client" {
		t.Fatal(w.Code, m)
	}
	v.Set("client_secret", sec)
	w, m = oauthHTTP(t, s, n, "/services/oauth2/token", v)
	if w.Code != 200 || m["instance_url"] != "https://crm.example.com" || m["id"] != "https://crm.example.com/id/"+orgID(u.Tenant)+"/"+u.ID || m["refresh_token"] == nil || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, m)
	}
	mac := hmac.New(sha256.New, []byte(sec))
	mac.Write([]byte(m["id"].(string) + m["issued_at"].(string)))
	if m["signature"] != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal(m)
	}
}
func TestTokenCLIShapes(t *testing.T) {
	s, g, n := tokenFixture(t)
	ver := strings.Repeat("a", 171)
	hash := sha256.Sum256([]byte(ver))
	code, e := s.IssueCode(context.Background(), CodeInput{ClientID: PlatformCLIClientID, UserID: g.User.ID, RedirectURI: "http://localhost:1717/OauthRedirect", Scope: "api", Challenge: base64.RawURLEncoding.EncodeToString(hash[:])}, n)
	if e != nil {
		t.Fatal(e)
	}
	w, m := oauthHTTP(t, s, n, "//services/oauth2/token", url.Values{"grant_type": {"authorization_code"}, "client_id": {PlatformCLIClientID}, "code": {code}, "code_verifier": {ver}, "redirect_uri": {"http://localhost:1717/OauthRedirect"}})
	if w.Code != 200 || m["signature"] != nil {
		t.Fatal(w.Code, m)
	}
	code, e = s.IssueCode(context.Background(), CodeInput{ClientID: PlatformCLIClientID, UserID: g.User.ID, RedirectURI: "http://localhost:1717/OauthRedirect", Scope: "api"}, n)
	if e != nil {
		t.Fatal(e)
	}
	w, m = oauthHTTP(t, s, n, "/services/oauth2/token", url.Values{"grant_type": {"authorization_code"}, "client_id": {PlatformCLIClientID}, "code": {code}, "redirect_uri": {"http://localhost:1717/OauthRedirect"}})
	if w.Code != 400 || m["error"] != "invalid_grant" {
		t.Fatal(w.Code, m)
	}

}
func TestTokenClientCredentials(t *testing.T) {
	s, u, id, sec, n := httpApp(t, []string{"client_credentials"})
	for _, basic := range []bool{false, true} {
		v := url.Values{"grant_type": {"client_credentials"}}
		var creds []string
		if basic {
			creds = []string{id, sec}
		} else {
			v.Set("client_id", id)
			v.Set("client_secret", sec)
		}
		w, m := oauthHTTP(t, s, n, "/services/oauth2/token", v, creds...)
		if w.Code != 200 || m["refresh_token"] != nil || m["scope"] != "api" {
			t.Fatal(w.Code, m)
		}
		p, e := s.AuthenticateAt(context.Background(), m["access_token"].(string), n)
		if e != nil || p.User != u.ID {
			t.Fatal(p, e)
		}
	}
	for _, secret := range []string{"", "wrong"} {
		w, m := oauthHTTP(t, s, n, "/services/oauth2/token", url.Values{"grant_type": {"client_credentials"}, "client_id": {id}, "client_secret": {secret}})
		if w.Code != 400 || m["error"] != "invalid_client" {
			t.Fatal(w.Code, m)
		}
	}
	w, m := oauthHTTP(t, s, n, "/services/oauth2/token", url.Values{"grant_type": {"client_credentials"}, "client_id": {id}, "client_secret": {sec}}, id, "different")
	if w.Code != 400 || m["error"] != "invalid_request" {
		t.Fatal(w.Code, m)
	}
	s.DeactivateUser(context.Background(), u.Username)
	w, m = oauthHTTP(t, s, n, "/services/oauth2/token", url.Values{"grant_type": {"client_credentials"}, "client_id": {id}, "client_secret": {sec}})
	if m["error"] != "invalid_grant" {
		t.Fatal(w.Code, m)
	}
}
func TestTokenJWT(t *testing.T) {
	s, g, n := tokenFixture(t)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert := jwtTestCert(t, key, n)
	id, _, e := s.CreateApp(context.Background(), AppInput{Tenant: g.User.Tenant, Name: "jwt", Grants: []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"}, Scopes: []string{"api", "refresh_token"}, JWTUsers: []string{g.User.Username}, JWTCertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))}, n)
	if e != nil {
		t.Fatal(e)
	}
	for index, jti := range []string{"once", "once", "", ""} {
		assertion := jwtTestSign(t, key, "RS256", map[string]any{"iss": id, "sub": g.User.Username, "aud": "https://crm.example.com/", "iat": n.Unix(), "exp": n.UnixMilli(), "jti": jti})
		w, m := oauthHTTP(t, s, n, "/services/oauth2/token", url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}})
		if index == 1 {
			if w.Code != 400 || m["error"] != "invalid_grant" {
				t.Fatal(w.Code, m)
			}
			continue
		}
		if w.Code != 200 || m["refresh_token"] != nil || m["scope"] != "api" {
			t.Fatal(w.Code, m)
		}
	}
	ctx := context.Background()
	otherName := "other-" + g.User.Username
	other, err := s.CreateUser(ctx, g.User.Tenant, otherName, "long-password", "admin")
	if err != nil {
		t.Fatal(err)
	}
	foreignName := "foreign-" + g.User.Username
	foreign, err := s.CreateUser(ctx, g.User.Tenant+"-foreign", foreignName, "long-password", "admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, subject string
		prepare       func()
	}{
		{"unlisted", otherName, func() {}},
		{"foreign", foreignName, func() {
			_, err = s.pool.Exec(ctx, "INSERT INTO oauth_app_jwt_users VALUES($1,$2)", id, foreign)
			if err != nil {
				t.Fatal(err)
			}
		}},
		{"inactive", otherName, func() {
			_, err = s.pool.Exec(ctx, "INSERT INTO oauth_app_jwt_users VALUES($1,$2)", id, other)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.DeactivateUser(ctx, otherName); err != nil {
				t.Fatal(err)
			}
		}},
		{"disabled", g.User.Username, func() {
			if err = s.DisableApp(ctx, id); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.prepare()
			assertion := jwtTestSign(t, key, "RS256", map[string]any{"iss": id, "sub": tc.subject, "aud": "https://crm.example.com", "exp": n.Unix() + 60})
			w, m := oauthHTTP(t, s, n, "/services/oauth2/token", url.Values{"grant_type": {jwtGrant}, "assertion": {assertion}})
			if w.Code != 400 || m["error"] != "invalid_grant" || m["error_description"] != "Invalid authorization grant" || strings.Contains(w.Body.String(), assertion) {
				t.Fatal(w.Code, m)
			}
		})
	}
	noGrant, _, err := s.CreateApp(ctx, AppInput{Tenant: g.User.Tenant, Name: "no-grant", Scopes: []string{"api"}, JWTCertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))}, n)
	if err != nil {
		t.Fatal(err)
	}
	assertion := jwtTestSign(t, key, "RS256", map[string]any{"iss": noGrant, "sub": g.User.Username, "aud": "https://crm.example.com", "exp": n.Unix() + 60})
	w, m := oauthHTTP(t, s, n, "/services/oauth2/token", url.Values{"grant_type": {jwtGrant}, "assertion": {assertion}})
	if w.Code != 400 || m["error"] != "unsupported_grant_type" {
		t.Fatal(w.Code, m)
	}

}
func TestTokenRejects(t *testing.T) {
	s, _, n := tokenFixture(t)
	for _, tc := range []struct {
		v    url.Values
		want string
	}{{url.Values{}, "invalid_request"}, {url.Values{"grant_type": {"password"}}, "unsupported_grant_type"}, {url.Values{"grant_type": {"unknown"}}, "unsupported_grant_type"}, {url.Values{"grant_type": {"a", "b"}}, "invalid_request"}} {
		w, m := oauthHTTP(t, s, n, "/services/oauth2/token", tc.v)
		if w.Code != 400 || m["error"] != tc.want {
			t.Fatal(w.Code, m)
		}
	}
	h := NewHandlerWithOptions(s, "", Options{PublicURL: "https://crm.example.com"})
	for _, verb := range []string{"GET", "POST"} {
		r := httptest.NewRequest(verb, "/services/oauth2/token", strings.NewReader(`{"secret":"do-not-echo"}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 400
		if verb == "GET" {
			want = 405
		}
		if w.Code != want || strings.Contains(w.Body.String(), "do-not-echo") {
			t.Fatal(w.Code, w.Body)
		}
	}
}
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
