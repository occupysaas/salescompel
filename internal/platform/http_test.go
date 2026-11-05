package platform

import (
	"context"
	"encoding/json"
	"fmt"
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
		{"POST", base + "composite/batch", "{}", token, 404},
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
	for _, tc := range []struct{ encoded, key string }{{"key%2F", "key/"}, {"a%2Fb", "a/b"}, {"key%252F", "key%2F"}} {
		encoded := tc.encoded
		r := httptest.NewRequest("PATCH", apiBase+"sobjects/Account/Compat_Key__c/"+encoded, strings.NewReader(`{"Name":"Distinct"}`))
		r.Header.Set("Authorization", "Bearer token-"+a.Tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 201 {
			t.Fatalf("%s: status %d, %s", encoded, w.Code, w.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		id, _ := result["id"].(string)
		row, err := s.Read(ctx, a, "Account", id)
		if err != nil || row["Compat_Key__c"] != tc.key {
			t.Fatalf("%s must store exact key %q: %v, %v", encoded, tc.key, row, err)
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

func TestHTTPCustomFieldLengths(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	h := NewHandler(s, "")
	base := "/services/data/v65.0/sobjects/Account"
	request := func(method, path string, data any) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(data)
		r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
		r.Header.Set("Authorization", "Bearer token-"+a.Tenant)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	d := request("GET", base+"/describe", nil)
	if d.Code != 200 {
		t.Fatal(d.Code, d.Body.String())
	}
	var obj Object
	if e := json.Unmarshal(d.Body.Bytes(), &obj); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"Compat_Text__c", "Compat_Key__c"} {
		found := false
		for _, f := range obj.Fields {
			if f.Name == name {
				found = true
				if f.Length != 80 || f.Type != "string" || !f.Nillable || f.ExternalID != (name == "Compat_Key__c") || f.ReferenceTo == nil || len(f.ReferenceTo) != 0 {
					t.Fatal(f)
				}
			}
		}
		if !found {
			t.Fatal("missing", name)
		}
		bad := request("POST", base, map[string]any{"Name": "oversized", name: strings.Repeat("x", 81)})
		if bad.Code != 400 || !strings.Contains(bad.Body.String(), "STRING_TOO_LONG") {
			t.Fatal(bad.Code, bad.Body.String())
		}
		good := request("POST", base, map[string]any{"Name": "boundary", name: strings.Repeat("x", 80)})
		if good.Code != 201 {
			t.Fatal(good.Code, good.Body.String())
		}
		var created struct {
			ID string `json:"id"`
		}
		json.Unmarshal(good.Body.Bytes(), &created)
		path := base + "/" + created.ID
		before := request("GET", path, nil).Body.String()
		bad = request("PATCH", path, map[string]any{"Name": "must not change", name: strings.Repeat("x", 81)})
		if bad.Code != 400 || !strings.Contains(bad.Body.String(), "STRING_TOO_LONG") {
			t.Fatal(bad.Code, bad.Body.String())
		}
		if after := request("GET", path, nil).Body.String(); before != after {
			t.Fatal("rejected write changed record")
		}
		if w := request("PATCH", path, map[string]any{name: strings.Repeat("é", 80)}); w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
		if w := request("DELETE", path, nil); w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func bodyValidationRequest(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func assertBodyValidationEnvelope(t *testing.T, w *httptest.ResponseRecorder, code string) {
	t.Helper()
	var items []map[string]any
	if w.Code != 400 || json.Unmarshal(w.Body.Bytes(), &items) != nil || len(items) != 1 {
		t.Fatalf("expected one 400 error item, got %d %s", w.Code, w.Body.String())
	}
	item := items[0]
	if _, present := item["fields"]; present {
		t.Errorf("fields must be absent: %s", w.Body.String())
	}
	wantMessage := "External ID value must be supplied in the upsert URL"
	if code == "JSON_PARSER_ERROR" {
		wantMessage = "External ID requires a scalar value or null"
	}
	if len(item) != 2 || item["errorCode"] != code || item["message"] != wantMessage {
		t.Errorf("error item = %v, want message and %s only", item, code)
	}
}

func TestHTTPUpsertBodyValidation(t *testing.T) {
	s := testStore(t)
	h := NewHandler(s, "")
	path := apiBase + "sobjects/Account/Compat_Key__c/body-key"
	for _, secondTenant := range []bool{false, true} {
		for _, existing := range []bool{false, true} {
			for _, tc := range upsertBodyCases() {
				t.Run(fmt.Sprintf("tenantB=%v/existing=%v/%s", secondTenant, existing, tc.name), func(t *testing.T) {
					p, peer, oldID := upsertBodyFixture(t, s, existing, secondTenant)
					before, peerBefore := accountSnapshot(t, s, p), accountSnapshot(t, s, peer)
					data := map[string]any{"Name": "Requested"}
					if tc.present {
						data["Compat_Key__c"] = tc.value
					}
					inputBefore := stringJSON(data)
					w := bodyValidationRequest(t, h, "PATCH", path, inputBefore, "token-"+p.Tenant)
					if stringJSON(data) != inputBefore {
						t.Error("caller input changed")
					}
					if tc.code != "" {
						if after := accountSnapshot(t, s, p); after != before {
							t.Errorf("rejected write changed rows: %s -> %s", before, after)
						}
						assertBodyValidationEnvelope(t, w, tc.code)
					} else {
						wantStatus := 201
						if existing {
							wantStatus = 200
						}
						var result map[string]any
						if w.Code != wantStatus || json.Unmarshal(w.Body.Bytes(), &result) != nil {
							t.Fatalf("status = %d, want %d: %s", w.Code, wantStatus, w.Body.String())
						}
						id, _ := result["id"].(string)
						errors, errorsOK := result["errors"].([]any)
						created, createdOK := result["created"].(bool)
						if len(result) != 4 || result["success"] != true || !errorsOK || len(errors) != 0 || !createdOK {
							t.Fatalf("invalid success response: %v", result)
						}
						assertUpsertBodySuccess(t, s, p, id, oldID, created, existing)
						// A successful create must become an update of the same ID.
						if !existing {
							w = bodyValidationRequest(t, h, "PATCH", path, inputBefore, "token-"+p.Tenant)
							var updated map[string]any
							if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &updated) != nil || updated["id"] != id || updated["created"] != false || updated["success"] != true {
								t.Fatalf("second upsert must update same ID: %d %s", w.Code, w.Body.String())
							}
							assertUpsertBodySuccess(t, s, p, id, id, false, false)
						}
					}
					if after := accountSnapshot(t, s, peer); after != peerBefore {
						t.Errorf("peer tenant changed: %s -> %s", peerBefore, after)
					}
				})
			}
		}
	}
}

func TestHTTPUpsertBodyBoundaryControls(t *testing.T) {
	s := testStore(t)
	p, peer, _ := upsertBodyFixture(t, s, true, false)
	h := NewHandler(s, "")
	path := apiBase + "sobjects/Account/Compat_Key__c/body-key"
	before, peerBefore := accountSnapshot(t, s, p), accountSnapshot(t, s, peer)
	for _, tc := range []struct {
		name, body, token, code string
		status                  int
	}{
		{"malformed", `{"Name":`, "token-" + p.Tenant, "JSON_PARSER_ERROR", 400},
		{"trailing-json", `{"Name":"Requested"} {}`, "token-" + p.Tenant, "JSON_PARSER_ERROR", 400},
		{"null-document", `null`, "token-" + p.Tenant, "JSON_PARSER_ERROR", 400},
		{"unauthorized", `{"Compat_Key__c":"body-key"}`, "", "INVALID_SESSION_ID", 401},
		{"invalid-token", `{"Compat_Key__c":"body-key"}`, "bad", "INVALID_SESSION_ID", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := bodyValidationRequest(t, h, "PATCH", path, tc.body, tc.token)
			var items []map[string]any
			if w.Code != tc.status || json.Unmarshal(w.Body.Bytes(), &items) != nil || len(items) != 1 || items[0]["errorCode"] != tc.code {
				t.Errorf("got %d %s, want %d %s", w.Code, w.Body.String(), tc.status, tc.code)
			}
			if accountSnapshot(t, s, p) != before || accountSnapshot(t, s, peer) != peerBefore {
				t.Error("boundary rejection changed rows")
			}
		})
	}
}

func TestHTTPOrdinaryExternalIDWrites(t *testing.T) {
	s := testStore(t)
	p, peer := principals(t, s)
	h := NewHandler(s, "")
	path := apiBase + "sobjects/Account"
	w := bodyValidationRequest(t, h, "POST", path, `{"Name":"Ordinary","Compat_Key__c":"create-key"}`, "token-"+p.Tenant)
	var result map[string]any
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	id, _ := result["id"].(string)
	row, err := s.Read(context.Background(), p, "Account", id)
	if err != nil || row["Compat_Key__c"] != "create-key" {
		t.Fatal(row, err)
	}
	w = bodyValidationRequest(t, h, "PATCH", path+"/"+id, `{"Name":"Updated","Compat_Key__c":"update-key"}`, "token-"+p.Tenant)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	row, err = s.Read(context.Background(), p, "Account", id)
	if err != nil || row["Name"] != "Updated" || row["Compat_Key__c"] != "update-key" {
		t.Fatal(row, err)
	}
	if rows := accountSnapshot(t, s, peer); rows != "[]" {
		t.Fatalf("ordinary write affected peer: %s", rows)
	}
}

func TestHTTPRecordIDForms(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	h := NewHandler(s, "")
	token := "token-" + p.Tenant
	req := func(method, path, data string) *httptest.ResponseRecorder {
		return bodyValidationRequest(t, h, method, apiBase+"sobjects/"+path, data, token)
	}
	id, e := s.Create(context.Background(), p, "Account", map[string]any{"Name": "Before"})
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{"bad", "003" + id[3:], strings.ToLower(id[:15])} {
		w := req("GET", "Account/"+bad, "")
		if w.Code != 404 || !strings.Contains(w.Body.String(), "NOT_FOUND") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := req("GET", "Account/"+id[:15], "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), id) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req("PATCH", "Account/"+id[:15], `{"Name":"After"}`)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req("POST", "Contact", `{"LastName":"Child","AccountId":"bad"}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"fields":["AccountId"]`) || !strings.Contains(w.Body.String(), "MALFORMED_ID") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req("POST", "Contact", `{"LastName":"Child","AccountId":"`+id[:15]+`"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	w = req("GET", "Contact/"+result["id"].(string), "")
	if !strings.Contains(w.Body.String(), id) {
		t.Fatal(w.Body.String())
	}
	req("DELETE", "Contact/"+result["id"].(string), "")
	w = req("DELETE", "Account/"+id[:15], "")
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestHTTPCaseInsensitiveRoutes(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	h := NewHandler(s, "")
	token := "token-" + p.Tenant
	req := func(m, path, b string) *httptest.ResponseRecorder {
		return bodyValidationRequest(t, h, m, apiBase+"sobjects/"+path, b, token)
	}
	w := req("GET", "account/describe", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"Account"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req("POST", "ACCOUNT", `{"name":"Mixed"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var r map[string]any
	json.Unmarshal(w.Body.Bytes(), &r)
	w = req("GET", "account/"+r["id"].(string), "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"Name":"Mixed"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req("PATCH", "account/compat_key__c/mixed", `{"name":"Mixed"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestHTTPDuplicateBodyFields(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	h := NewHandler(s, "")
	for _, data := range []string{`{"Name":"first","Name":"second"}`, `{"name":"first","Name":"second"}`, `{"Name":"first","name":"second"}`} {
		w := bodyValidationRequest(t, h, "POST", apiBase+"sobjects/Account", data, "token-"+p.Tenant)
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var r map[string]any
		json.Unmarshal(w.Body.Bytes(), &r)
		row, e := s.Read(context.Background(), p, "Account", r["id"].(string))
		if e != nil || row["Name"] != "second" {
			t.Fatal(row, e)
		}
	}
	restricted := p
	restricted.Role = "restricted"
	token := "restricted-" + p.Tenant
	if e := s.Provision(context.Background(), token, restricted); e != nil {
		t.Fatal(e)
	}
	id, e := s.Create(context.Background(), p, "Account", map[string]any{"Name": "Original"})
	if e != nil {
		t.Fatal(e)
	}
	for _, data := range []string{`{"Name":"x","compat_text__c":"y"}`, `{"Name":"x","Compat_Text__c":"y","compat_text__c":null}`} {
		w := bodyValidationRequest(t, h, "PATCH", apiBase+"sobjects/Account/"+id, data, token)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_FIELD") {
			t.Fatal(w.Code, w.Body.String())
		}
		row, e := s.Read(context.Background(), p, "Account", id)
		if e != nil || row["Name"] != "Original" {
			t.Fatal(row, e)
		}
	}
}
func TestHTTPUnknownObjects(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	h := NewHandler(s, "")
	for _, path := range []string{"sobjects/Nope/describe", "sobjects/Nope"} {
		m := "GET"
		if path == "sobjects/Nope" {
			m = "POST"
		}
		w := bodyValidationRequest(t, h, m, apiBase+path, `{}`, "token-"+p.Tenant)
		if w.Code != 404 || !strings.Contains(w.Body.String(), "NOT_FOUND") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := bodyValidationRequest(t, h, "GET", apiBase+"query?q=SELECT+Id+FROM+Nope", "", "token-"+p.Tenant)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_TYPE") {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestHTTPFieldErrorEnvelope(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	h := NewHandler(s, "")
	if _, e := s.Create(context.Background(), p, "Account", map[string]any{"Name": "Existing", "Compat_Key__c": "key"}); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ body, code, field string }{{`{}`, "REQUIRED_FIELD_MISSING", "Name"}, {`{"Name":"` + strings.Repeat("x", 256) + `"}`, "STRING_TOO_LONG", "Name"}, {`{"Name":"x","Id":"x"}`, "INVALID_FIELD_FOR_INSERT_UPDATE", "Id"}, {`{"Name":"x","Compat_Key__c":"key"}`, "DUPLICATE_VALUE", "Compat_Key__c"}} {
		w := bodyValidationRequest(t, h, "POST", apiBase+"sobjects/Account", tc.body, "token-"+p.Tenant)
		var errs []APIError
		json.Unmarshal(w.Body.Bytes(), &errs)
		if w.Code != 400 || len(errs) != 1 || errs[0].Code != tc.code || len(errs[0].Fields) != 1 || errs[0].Fields[0] != tc.field {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestHTTPOrdinaryExternalIDUniqueness(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	h := NewHandler(s, "")
	id, e := s.Create(ctx, p, "Account", map[string]any{"Name": "Original", "Compat_Key__c": "AbC"})
	if e != nil {
		t.Fatal(e)
	}
	other, e := s.Create(ctx, p, "Account", map[string]any{"Name": "Other"})
	if e != nil {
		t.Fatal(e)
	}
	before := accountSnapshot(t, s, p)
	for _, path := range []string{"", "/" + other} {
		m := "POST"
		if path != "" {
			m = "PATCH"
		}
		w := bodyValidationRequest(t, h, m, apiBase+"sobjects/Account"+path, `{"Name":"Fail","Compat_Key__c":"abc"}`, "token-"+p.Tenant)
		if w.Code != 400 || !strings.Contains(w.Body.String(), `"fields":["Compat_Key__c"]`) || !strings.Contains(w.Body.String(), "DUPLICATE_VALUE") {
			t.Fatal(w.Code, w.Body.String())
		}
		if accountSnapshot(t, s, p) != before {
			t.Fatal("rejected write changed rows")
		}
	}
	if e = s.Update(ctx, p, "Account", id, map[string]any{"Compat_Key__c": "abc"}); e != nil {
		t.Fatal(e)
	}
	row, e := s.Read(ctx, p, "Account", id)
	if e != nil || row["Compat_Key__c"] != "abc" {
		t.Fatal(row, e)
	}
	for _, key := range []string{"   ", strings.Repeat("ß", 80)} {
		id, e := s.Create(ctx, p, "Account", map[string]any{"Name": "Key", "Compat_Key__c": key})
		if e != nil {
			t.Fatal(e)
		}
		row, e := s.Read(ctx, p, "Account", id)
		var want any = key
		if key == "   " {
			want = nil
		}
		if e != nil || row["Compat_Key__c"] != want {
			t.Fatal(row, e)
		}
	}
	w := bodyValidationRequest(t, h, "POST", apiBase+"sobjects/Account", `{"Name":"Long","Compat_Key__c":"`+strings.Repeat("s", 160)+`"}`, "token-"+p.Tenant)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "STRING_TOO_LONG") {
		t.Fatal(w.Code, w.Body.String())
	}
}
