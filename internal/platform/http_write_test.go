package platform

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

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
