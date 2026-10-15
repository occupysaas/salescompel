package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPBoundary(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	h := NewHandler(s, "")
	request := func(method, path, body, token string) (int, any) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Tenant-ID", b.Tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var result any
		if w.Body.Len() > 0 {
			if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
				t.Fatalf("non-json response %s", w.Body.String())
			}
		}
		return w.Code, result
	}
	base := "/services/data/v65.0/"
	token := "token-" + a.Tenant
	for _, tt := range []struct {
		method, path, body, token string
		status                    int
	}{
		{"GET", base + "sobjects/Account/describe", "", "", 401},
		{"GET", base + "sobjects/Account/describe", "", "bad", 401},
		{"GET", base + "sobjects/Account/describe", "", token, 200},
		{"GET", "//services/data", "", token, 200},
		{"POST", base + "sobjects/Account", `{"Name":"A"} {}`, token, 400},
		{"POST", base + "sobjects/Account", `null`, token, 400},
		{"POST", base + "sobjects/Account", `{"Name":3}`, token, 400},
		{"GET", "/services/data/v99.0/sobjects", "", token, 404},
		{"POST", base + "query", "{}", token, 405},
		{"POST", base + "composite", "{}", token, 404},
	} {
		code, _ := request(tt.method, tt.path, tt.body, tt.token)
		if code != tt.status {
			t.Errorf("%s %s: %d want %d", tt.method, tt.path, code, tt.status)
		}
	}
	code, result := request("POST", base+"sobjects/Account", `{"Name":"HTTP"}`, token)
	if code != 201 {
		t.Fatal(code, result)
	}
	id := result.(map[string]any)["id"].(string)
	code, row := request("GET", base+"sobjects/Account/"+id, "", token)
	if code != 200 || row.(map[string]any)["Name"] != "HTTP" {
		t.Fatal(code, row)
	}
	if code, _ = request("GET", base+"sobjects/Account/"+id, "", "token-"+b.Tenant); code != 404 {
		t.Fatal("tenant leak", code)
	}
	if code, _ = request("PATCH", base+"sobjects/Account/"+id, `{"Name":"Updated"}`, token); code != 204 {
		t.Fatal(code)
	}
	if code, _ = request("DELETE", base+"sobjects/Account/"+id, "", token); code != 204 {
		t.Fatal(code)
	}
	if code, _ = request("GET", base+"sobjects/Account/"+id, "", token); code != 404 {
		t.Fatal(code)
	}
	p := a
	p.Role = "restricted"
	p.User = "reader"
	restricted := "restricted-" + a.Tenant
	if err := s.Provision(context.Background(), restricted, p); err != nil {
		t.Fatal(err)
	}
	code, result = request("GET", base+"sobjects/Account/describe", "", restricted)
	if code != http.StatusOK || strings.Contains(stringJSON(result), "Compat_Text__c") {
		t.Fatal(code, result)
	}
}
func stringJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestHTTPExternalIDPreservesEscapedSlashes(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	h := NewHandler(s, "")
	original, _, err := s.Upsert(ctx, a, "key", map[string]any{"Name": "Original"})
	if err != nil {
		t.Fatal(err)
	}
	for _, encoded := range []string{"key%2F", "a%2Fb", "key%252F"} {
		r := httptest.NewRequest("PATCH", apiBase+"sobjects/Account/Compat_Key__c/"+encoded, strings.NewReader(`{"Name":"Distinct"}`))
		r.Header.Set("Authorization", "Bearer token-"+a.Tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 201 {
			t.Errorf("%s: status %d, %s", encoded, w.Code, w.Body.String())
		}
	}
	row, err := s.Read(ctx, a, "Account", original)
	if err != nil || row["Name"] != "Original" {
		t.Fatal("changed different external ID", row, err)
	}
	q, _ := ParseQuery("SELECT Id,Compat_Key__c FROM Account")
	rows, err := s.Select(ctx, a, q)
	if err != nil || len(rows) != 4 {
		t.Fatal(rows, err)
	}
}

func TestHTTPRejectsUndeliverablePathsAndListings(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	assets := t.TempDir()
	if err := os.MkdirAll(filepath.Join(assets, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "assets/app.js"} {
		if err := os.WriteFile(filepath.Join(assets, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(s, assets)
	for _, tt := range []struct {
		path   string
		status int
	}{
		{"/", 200},
		{"/assets/app.js", 200},
		{"/assets/", 404},
		{apiBase + "sobjects/Account/%FF", 400},
		{apiBase + "sobjects/Account/a%00", 400},
		{apiBase + "query?q=SELECT%20Id%20FROM%20Account%20WHERE%20Name%20%3D%20%27%FF%27", 400},
	} {
		r := httptest.NewRequest("GET", tt.path, nil)
		r.Header.Set("Authorization", "Bearer token-"+a.Tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tt.status {
			t.Errorf("%s: %d want %d (%s)", tt.path, w.Code, tt.status, w.Body.String())
		}
	}
}
