package platform

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

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
