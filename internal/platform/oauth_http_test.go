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
	}{{"https://CRM.EXAMPLE.COM:443", "", true, "https://crm.example.com"}, {"http://LOCALHOST:80/", "", false, "http://localhost"}, {"https://[::1]:443", "", true, "https://[::1]"}, {"https://CRM.EXAMPLE.COM:8443", "", true, "https://crm.example.com:8443"}, {"", "127.0.0.1:8080", false, "http://127.0.0.1:8080"}, {"", "127.0.0.1:8080", true, "https://127.0.0.1:8080"}, {"https://crm.example.com/", "0.0.0.0:443", true, "https://crm.example.com"}, {"http://example.com", "127.0.0.1:8080", false, ""}, {"https://h/path", "", true, ""}, {"https://h?q=x", "", true, ""}, {"https://h#f", "", true, ""}, {"", "0.0.0.0:8080", true, ""}} {
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

func identityGET(t *testing.T, s *Store, n time.Time, path, token, assets string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.Host = "spoof.invalid"
	r.Header.Set("X-Forwarded-Host", "spoof.invalid")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	NewHandlerWithOptions(s, assets, Options{PublicURL: "https://crm.example.com", Now: func() time.Time { return n }}).ServeHTTP(w, r)
	m := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}
func TestDiscovery(t *testing.T) {
	w, m := identityGET(t, nil, time.Now(), "/.well-known/openid-configuration", "", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, key := range []string{"issuer", "authorization_endpoint", "token_endpoint", "revocation_endpoint", "introspection_endpoint", "userinfo_endpoint"} {
		v, ok := m[key].(string)
		if !ok || !strings.HasPrefix(v, "https://crm.example.com") {
			t.Fatal(key, m)
		}
	}
	for _, key := range []string{"response_types_supported", "scopes_supported", "code_challenge_methods_supported", "token_endpoint_auth_methods_supported"} {
		if m[key] == nil {
			t.Fatal(key, m)
		}
	}
	grants, _ := json.Marshal(m["grant_types_supported"])
	if string(grants) != `["authorization_code","refresh_token","client_credentials","urn:ietf:params:oauth:grant-type:jwt-bearer"]` {
		t.Fatal(string(grants))
	}
	for _, key := range []string{"jwks_uri", "id_token_signing_alg_values_supported"} {
		if m[key] != nil {
			t.Fatal(m)
		}
	}
	scopes, _ := json.Marshal(m["scopes_supported"])
	if strings.Contains(string(scopes), "openid") {
		t.Fatal(m)
	}
	w = httptest.NewRecorder()
	NewHandler(nil, "").ServeHTTP(w, httptest.NewRequest("GET", "/.well-known/openid-configuration", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestIdentityURL(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	path := "/id/" + orgID(g.User.Tenant) + "/" + g.User.ID
	w, m := identityGET(t, s, n, path, v.Access, t.TempDir())
	if w.Code != 200 || m["id"] != "https://crm.example.com"+path || m["user_id"] != g.User.ID || m["organization_id"] != orgID(g.User.Tenant) || m["username"] != g.User.Username {
		t.Fatal(w.Code, m, w.Body.String())
	}
	for _, tc := range []struct {
		path, token string
		status      int
	}{{path, "", 401}, {path + "x", v.Access, 403}, {"/id/" + orgID("other") + "/" + g.User.ID, v.Access, 403}} {
		w, _ = identityGET(t, s, n, tc.path, tc.token, "")
		if w.Code != tc.status {
			t.Fatal(tc, w.Code)
		}
	}
	otherID, err := s.CreateUser(context.Background(), g.User.Tenant, "other-"+g.User.Username, "long-password", "admin")
	if err != nil {
		t.Fatal(err)
	}
	otherUser, err := s.VerifyLogin(context.Background(), "other-"+g.User.Username, "long-password", n)
	if err != nil || otherUser.ID != otherID {
		t.Fatal(err)
	}
	otherGrant := g
	otherGrant.User = otherUser
	otherToken := issueTest(t, s, otherGrant, n)
	w, _ = identityGET(t, s, n, path, otherToken.Access, "")
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	g.Scope = "web"
	v = issueTest(t, s, g, n)
	w, _ = identityGET(t, s, n, path, v.Access, "")
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	g.Scope = "id"
	v = issueTest(t, s, g, n)
	w, _ = identityGET(t, s, n, path, v.Access, "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	w, _ = identityGET(t, s, n.Add(time.Hour), path, v.Access, "")
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestUserInfoForOAuth(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	w, m := identityGET(t, s, n, "/services/oauth2/userinfo", v.Access, "")
	if w.Code != 200 || m["user_id"] != g.User.ID || m["organization_id"] != orgID(g.User.Tenant) || m["username"] != g.User.Username || m["preferred_username"] != g.User.Username || m["sub"] != "https://crm.example.com/id/"+orgID(g.User.Tenant)+"/"+g.User.ID {
		t.Fatal(w.Code, m)
	}
	if e := s.Provision(context.Background(), "legacy-identity-token", Principal{Tenant: g.User.Tenant, User: "legacy", Role: "admin"}); e != nil {
		t.Fatal(e)
	}
	w, _ = identityGET(t, s, n, "/services/oauth2/userinfo", "legacy-identity-token", "")
	want, _ := json.Marshal(map[string]any{"user_id": "legacy", "organization_id": g.User.Tenant, "preferred_username": "legacy", "username": "legacy"})
	if w.Body.String() != string(want)+"\n" {
		t.Fatal(w.Body.String())
	}
}
func TestUserRouteForOAuth(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	for _, version := range []string{"v51.0", "v65.0"} {
		path := "/services/data/" + version + "/sobjects/User/"
		w, m := identityGET(t, s, n, path+g.User.ID, v.Access, "")
		if w.Code != 200 || m["Username"] != g.User.Username {
			t.Fatal(w.Code, m)
		}
		w, _ = identityGET(t, s, n, path+"other", v.Access, "")
		if w.Code != 404 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
func TestOrganizationBridge(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	q := "SELECT Name, InstanceName, IsSandbox, TrialExpirationDate, NamespacePrefix, OrganizationType FROM Organization"
	for _, query := range []string{q, strings.ToLower(q), " SELECT\tName , InstanceName,IsSandbox, TrialExpirationDate ,NamespacePrefix,OrganizationType\nFROM  Organization "} {
		w, m := identityGET(t, s, n, apiBase+"query?q="+url.QueryEscape(query), v.Access, "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		rows := m["records"].([]any)
		row := rows[0].(map[string]any)
		if m["totalSize"] != float64(1) || m["done"] != true || row["Name"] != g.User.Tenant || row["IsSandbox"] != false || row["OrganizationType"] != "SalesCompel" {
			t.Fatal(m)
		}
		for _, key := range []string{"InstanceName", "TrialExpirationDate", "NamespacePrefix"} {
			val, ok := row[key]
			if !ok || val != nil {
				t.Fatal(row)
			}
		}
	}
	for _, query := range []string{"SELECT Name FROM Organization", q + " WHERE Name = 'x'"} {
		w, _ := identityGET(t, s, n, apiBase+"query?q="+url.QueryEscape(query), v.Access, "")
		if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_TYPE") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
