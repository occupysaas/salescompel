package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	apicontract "salescompel/api"
	"strings"
	"testing"
)

func TestHTTPServedOpenAPI(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	restricted := a
	restricted.Role = "restricted"
	restricted.User = "contract-reader"
	rt := "restricted-contract-" + a.Tenant
	if err := s.Provision(context.Background(), rt, restricted); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir()) // The binary serves no files from the checkout or asset root.
	server := httptest.NewServer(NewHandler(s, ""))
	defer server.Close()
	request := func(method, path, token string) *http.Response {
		t.Helper()
		r, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Forwarded-Host", "PRIVATE_MARKER.invalid")
		res, err := server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	for _, token := range []string{"token-" + a.Tenant, rt, "token-" + b.Tenant} {
		res := request("GET", apicontract.CandidateOpenAPIPath, token)
		data, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || string(data) != apicontract.CandidateOpenAPI() {
			t.Fatalf("contract status/bytes: %d", res.StatusCode)
		}
		for k, want := range map[string]string{"Content-Type": "application/yaml; charset=utf-8", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer"} {
			if res.Header.Get(k) != want {
				t.Fatalf("%s: %q", k, res.Header.Get(k))
			}
		}
		if res.Header.Get("Content-Security-Policy") == "" {
			t.Fatal("missing CSP")
		}
		for _, marker := range []string{token, a.Tenant, b.Tenant, restricted.User, "PRIVATE_MARKER"} {
			if strings.Contains(string(data), marker) {
				t.Fatal("dynamic value leaked into static contract")
			}
		}
	}
	for _, tc := range []struct {
		method, path, token string
		status              int
		allow               string
	}{
		{"GET", apicontract.CandidateOpenAPIPath, "", 401, ""},
		{"GET", apicontract.CandidateOpenAPIPath, "bad", 401, ""},
		{"POST", apicontract.CandidateOpenAPIPath, "token-" + a.Tenant, 405, "GET"},
		{"HEAD", apicontract.CandidateOpenAPIPath, "token-" + a.Tenant, 405, "GET"},
		{"GET", "/services/data/v99.0/openapi.yaml", "token-" + a.Tenant, 404, ""},
	} {
		res := request(tc.method, tc.path, tc.token)
		data, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != tc.status || res.Header.Get("Allow") != tc.allow {
			t.Fatalf("%s %s: status %d Allow %q", tc.method, tc.path, res.StatusCode, res.Header.Get("Allow"))
		}
		if tc.method == "HEAD" {
			if len(data) != 0 {
				t.Fatal("HEAD returned body")
			}
			continue
		}
		var errors []map[string]any
		if json.Unmarshal(data, &errors) != nil || len(errors) != 1 || errors[0]["errorCode"] == nil {
			t.Fatal("missing JSON error shape")
		}
	}
	res := request("GET", "/services/data/v65.0", "token-"+a.Tenant)
	defer res.Body.Close()
	var resources map[string]string
	if json.NewDecoder(res.Body).Decode(&resources) != nil {
		t.Fatal("bad discovery")
	}
	if len(resources) != 4 || resources["openapi"] != apicontract.CandidateOpenAPIPath {
		t.Fatal("missing or widened resource discovery")
	}
}
