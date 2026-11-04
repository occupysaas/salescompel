package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func compositeJSON(subs string) string { return `{"allOrNone":true,"compositeRequest":[` + subs + `]}` }

const compositeCreate = `{"method":"POST","url":"/services/data/v65.0/sobjects/Account","referenceId":"a","body":{"Name":"Synthetic"}}`

func TestDecodeComposite(t *testing.T) {
	valid := compositeJSON(compositeCreate)
	for _, input := range []string{valid, strings.Replace(valid, `"allOrNone":true`, `"allOrNone":true,"collateSubrequests":false`, 1), strings.Replace(valid, `"Name":"Synthetic"`, `"Unknown":true`, 1)} {
		if _, err := decodeComposite(strings.NewReader(input)); err != nil {
			t.Fatal(err)
		}
	}
	for name, input := range map[string]string{
		"missing": `{}`, "false": strings.Replace(valid, `true`, `false`, 1), "null": strings.Replace(valid, `true`, `null`, 1), "collate": strings.Replace(valid, `true`, `true,"collateSubrequests":true`, 1), "collate-null": strings.Replace(valid, `true`, `true,"collateSubrequests":null`, 1), "unknown": strings.Replace(valid, `true`, `true,"private_marker":1`, 1), "empty": compositeJSON(""), "null-sub": compositeJSON("null"), "null-array": `{"allOrNone":true,"compositeRequest":null}`, "trailing": valid + ` {}`, "bad-json": `{`, "headers": strings.Replace(valid, `"body":`, `"httpHeaders":{},"body":`, 1), "body-null": strings.Replace(valid, `{"Name":"Synthetic"}`, `null`, 1), "body-array": strings.Replace(valid, `{"Name":"Synthetic"}`, `[]`, 1), "body-missing": strings.Replace(valid, `,"body":{"Name":"Synthetic"}`, ``, 1), "duplicate": compositeJSON(compositeCreate + "," + compositeCreate),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := decodeComposite(strings.NewReader(input))
			var api *APIError
			if !errors.As(err, &api) || api.Status != 400 || strings.Contains(api.Message, "private_marker") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
	// DELETE requires no wire body, not even null.
	d := `{"method":"DELETE","url":"/services/data/v65.0/sobjects/Account/001000000000000AAA","referenceId":"d"}`
	if _, err := decodeComposite(strings.NewReader(compositeJSON(d))); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeComposite(strings.NewReader(compositeJSON(strings.TrimSuffix(d, "}") + `,"body":null}`))); err == nil {
		t.Fatal("null DELETE body")
	}
	for _, n := range []int{1, 25, 26} {
		subs := []string{}
		for i := 0; i < n; i++ {
			subs = append(subs, strings.Replace(compositeCreate, `"a"`, fmt.Sprintf(`"a%d"`, i), 1))
		}
		_, err := decodeComposite(strings.NewReader(compositeJSON(strings.Join(subs, ","))))
		if (err == nil) != (n <= 25) {
			t.Fatalf("count %d: %v", n, err)
		}
	}
}
func TestValidateComposite(t *testing.T) {
	var r CompositeRequest
	if err := json.Unmarshal([]byte(compositeJSON(compositeCreate)), &r); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{strings.Repeat("a", 80), "a", "A", "0_"} {
		r.CompositeRequest[0].ReferenceID = id
		if err := validateComposite(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"", strings.Repeat("a", 81), "_a", "é", "a-b"} {
		r.CompositeRequest[0].ReferenceID = id
		if err := validateComposite(r); err == nil {
			t.Fatal("accepted id")
		}
	}
	r.CompositeRequest[0].ReferenceID = "a"
	for _, url := range []string{"https://private_marker.invalid/", "//evil/", "/services/data/v65.0/composite", "/services/data/v64.0/sobjects/Account", "/services/data/v65.0/sobjects/User", "/services/data/v65.0/sobjects/Account/", "/services/data/v65.0/sobjects/Account?q=1", "/services/data/v65.0/sobjects/Account#x", "/services/data/v65.0/sobjects/%41ccount", "/services/data/v65.0/sobjects/../Account", "/services//data/v65.0/sobjects/Account"} {
		r.CompositeRequest[0].URL = url
		if err := validateComposite(r); err == nil {
			t.Fatal("accepted URL")
		}
	}
	r.CompositeRequest[0].URL = "/services/data/v65.0/sobjects/Account"
	for _, method := range []string{"post", "GET", "PUT", ""} {
		r.CompositeRequest[0].Method = method
		if err := validateComposite(r); err == nil {
			t.Fatal("accepted method")
		}
	}
	r.CompositeRequest[0].Method = "POST"
	second := CompositeSubrequest{Method: "POST", URL: "/services/data/v65.0/sobjects/Contact", ReferenceID: "c", Body: map[string]any{"LastName": "Synthetic", "AccountId": "@{a.id}"}}
	r.CompositeRequest = append(r.CompositeRequest, second)
	if err := validateComposite(r); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"@{a.Id}", "@{c.id}", "@{future.id}", "before@{a.id}", "@{a.records[0].Id}", "@{a.id}/more"} {
		r.CompositeRequest[1].Body["AccountId"] = ref
		if err := validateComposite(r); err == nil {
			t.Fatal("accepted reference")
		}
	}
	r.CompositeRequest[1].Body = map[string]any{"LastName": "@{a.id}"}
	if err := validateComposite(r); err == nil {
		t.Fatal("reference in ordinary field")
	}
	r.CompositeRequest[1].Body = map[string]any{"@{a.id}": "x"}
	if err := validateComposite(r); err == nil {
		t.Fatal("reference key")
	}
	r.CompositeRequest[1] = CompositeSubrequest{Method: "PATCH", URL: "/services/data/v65.0/sobjects/Account/@{a.id}", ReferenceID: "p", Body: map[string]any{"Name": "Changed"}}
	if err := validateComposite(r); err != nil {
		t.Fatal(err)
	}
	r.CompositeRequest = append(r.CompositeRequest, CompositeSubrequest{Method: "DELETE", URL: "/services/data/v65.0/sobjects/Account/@{p.id}", ReferenceID: "d"})
	if err := validateComposite(r); err == nil {
		t.Fatal("reference to non-POST")
	}
}

func TestHTTPCompositeAtomic(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	server := httptest.NewServer(NewHandler(s, ""))
	defer server.Close()
	request := func(method, path, token, input string) (*http.Response, []byte) {
		t.Helper()
		r, e := http.NewRequest(method, server.URL+path, strings.NewReader(input))
		if e != nil {
			t.Fatal(e)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := server.Client().Do(r)
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(res.Body)
		res.Body.Close()
		if e != nil {
			t.Fatal(e)
		}
		return res, data
	}
	path := apiBase + "composite"
	token := "token-" + a.Tenant
	for _, tc := range []struct {
		method, path, token, input string
		status                     int
	}{{"POST", path, token, compositeJSON(compositeCreate), 200}, {"POST", path, "", compositeJSON(compositeCreate), 401}, {"POST", path, "bad", compositeJSON(compositeCreate), 401}, {"GET", path, token, "", 405}, {"HEAD", path, token, "", 405}, {"POST", "/services/data/v99.0/composite", token, "{}", 404}, {"POST", path, token, `{"private_marker":true}`, 400}, {"POST", path, token, strings.Repeat(" ", 1<<20) + "{}", 400}} {
		res, data := request(tc.method, tc.path, tc.token, tc.input)
		if res.StatusCode != tc.status {
			t.Fatalf("%s: got %d want %d", tc.method, res.StatusCode, tc.status)
		}
		if res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("Content-Security-Policy") == "" {
			t.Fatal("headers")
		}
		if tc.status == 405 && res.Header.Get("Allow") != "POST" {
			t.Fatal("allow")
		}
		if tc.method == "HEAD" && len(data) != 0 {
			t.Fatal("HEAD body")
		}
		if strings.Contains(string(data), "private_marker") {
			t.Fatal("private diagnostic")
		}
		if tc.status == 200 {
			var out map[string]any
			if json.Unmarshal(data, &out) != nil || len(out) != 1 {
				t.Fatal("envelope")
			}
			entry := out["compositeResponse"].([]any)[0].(map[string]any)
			if len(entry) != 4 || entry["httpStatusCode"] != float64(201) || entry["referenceId"] != "a" || len(entry["httpHeaders"].(map[string]any)) != 0 {
				t.Fatal("entry")
			}
		}
	}
	res, data := request("POST", path, token, compositeJSON(compositeCreate+`,{"method":"PATCH","url":"/services/data/v65.0/sobjects/Account/@{a.id}","referenceId":"bad","body":{"Unknown":true}}`))
	if res.StatusCode != 200 || !strings.Contains(string(data), "PROCESSING_HALTED") || !strings.Contains(string(data), "INVALID_FIELD") {
		t.Fatal("rollback envelope")
	}
	_, data = request("GET", strings.TrimSuffix(apiBase, "/"), token, "")
	var resources map[string]string
	_ = json.Unmarshal(data, &resources)
	if resources["composite"] != path || len(resources) != 4 {
		t.Fatal("discovery")
	}
	var n int
	if e := s.pool.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE tenant_id=$1", a.Tenant).Scan(&n); e != nil || n != 1 {
		t.Fatal("HTTP partial writes", e, n)
	}
}
