package platform

import (
	"context"
	"testing"
)

func TestHTTPReadFieldsParam(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	h := NewHandler(s, "")
	token := "token-" + a.Tenant
	id, err := s.Create(context.Background(), a, "Account", map[string]any{"Name": "Fields", "Compat_Text__c": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	base := apiBase + "sobjects/Account/" + id

	// Without fields the body is unchanged from the plain read.
	plain, err := s.Read(context.Background(), a, "Account", id)
	if err != nil {
		t.Fatal(err)
	}
	code, _, raw := apiCall(t, h, "GET", base, "", token)
	if code != 200 || raw != stringJSON(plain)+"\n" {
		t.Fatalf("plain read changed: %d %s", code, raw)
	}

	code, row, _ := apiCall(t, h, "GET", base+"?fields=name,%20Compat_Text__c", "", token)
	m, _ := row.(map[string]any)
	if code != 200 || m["Name"] != "Fields" || m["Compat_Text__c"] != "secret" {
		t.Fatalf("subset: %d %v", code, row)
	}
	if _, extra := m["CreatedDate"]; extra {
		t.Fatalf("unrequested field returned: %v", row)
	}
	if len(m) != 3 { // attributes, Name, Compat_Text__c (D33-03 adds no Id)
		t.Fatalf("unexpected keys: %v", row)
	}

	code, row, _ = apiCall(t, h, "GET", base+"?fields=Nope__c", "", token)
	if code != 400 || errorCode(row) != "INVALID_FIELD" {
		t.Fatalf("unknown field: %d %v", code, row)
	}
	// An empty value reads every field, as the plain route does (D33-03).
	if code, _, raw = apiCall(t, h, "GET", base+"?fields=", "", token); code != 200 || raw != stringJSON(plain)+"\n" {
		t.Fatalf("empty fields: %d %s", code, raw)
	}

	// A restricted caller cannot name a field it cannot read.
	p := a
	p.Role, p.User = "restricted", "reader"
	restricted := "restricted-fields-" + a.Tenant
	if err := s.Provision(context.Background(), restricted, p); err != nil {
		t.Fatal(err)
	}
	code, row, _ = apiCall(t, h, "GET", base+"?fields=Compat_Text__c", "", restricted)
	if code != 400 || errorCode(row) != "INVALID_FIELD" {
		t.Fatalf("restricted: %d %v", code, row)
	}
	code, row, _ = apiCall(t, h, "GET", base+"?fields=Name", "", restricted)
	if code != 200 || row.(map[string]any)["Name"] != "Fields" {
		t.Fatalf("restricted readable: %d %v", code, row)
	}
}
