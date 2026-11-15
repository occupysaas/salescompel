package platform

import (
	"strings"
	"testing"
)

// Plan 33 visibility on Plan 31 routes: a hidden object answers as an
// unknown one inside Composite and Collections too (D33-02), and a hidden
// field is INVALID_FIELD (D33-03, D33-04).

func compositeEntries(t *testing.T, body any) []map[string]any {
	t.Helper()
	m, _ := body.(map[string]any)
	list, _ := m["compositeResponse"].([]any)
	out := []map[string]any{}
	for _, e := range list {
		entry, _ := e.(map[string]any)
		out = append(out, entry)
	}
	return out
}

func itemErrorCode(t *testing.T, body any) string {
	t.Helper()
	list, _ := body.([]any)
	if len(list) != 1 {
		t.Fatalf("want one item: %v", body)
	}
	item, _ := list[0].(map[string]any)
	errs, _ := item["errors"].([]any)
	if item["success"] != false || len(errs) != 1 {
		t.Fatalf("want one failed item: %v", body)
	}
	e, _ := errs[0].(map[string]any)
	code, _ := e["statusCode"].(string)
	return code
}

func TestPlan31HiddenObject(t *testing.T) {
	f := newReadFixture(t)
	f.profile("Standard User")
	f.revoke("Standard User", "Account")
	id := f.account

	// Standalone Collections.
	code, body := f.do("GET", "composite/sobjects/Account?ids="+id+"&fields=Name", "", false)
	wantFailure(t, "retrieve GET", code, body, 404, "NOT_FOUND")
	code, body = f.do("POST", "composite/sobjects/Account", `{"ids":["`+id+`"],"fields":["Name"]}`, false)
	wantFailure(t, "retrieve POST", code, body, 404, "NOT_FOUND")
	code, body = f.do("DELETE", "composite/sobjects?ids="+id, "", false)
	if code != 200 || itemErrorCode(t, body) != "MALFORMED_ID" {
		t.Fatalf("delete hidden: %d %v", code, body)
	}
	_, unknown := f.do("DELETE", "composite/sobjects?ids=zzz"+id[3:], "", false)
	if strings.Replace(stringJSON(body), id, "", -1) != strings.Replace(stringJSON(unknown), "zzz"+id[3:], "", -1) {
		t.Fatalf("hidden %v differs from unknown prefix %v", body, unknown)
	}

	// Inside Composite: every entry is the item 404 the standalone route gives.
	req := `{"allOrNone":false,"compositeRequest":[
		{"method":"GET","url":"/services/data/v65.0/sobjects/Account/` + id + `","referenceId":"r"},
		{"method":"GET","url":"/services/data/v65.0/sobjects/Account/` + id + `?fields=Name","referenceId":"rf"},
		{"method":"POST","url":"/services/data/v65.0/composite/sobjects/Account","referenceId":"rp","body":{"ids":["` + id + `"],"fields":["Name"]}},
		{"method":"PATCH","url":"/services/data/v65.0/sobjects/Account/` + id + `","referenceId":"u","body":{"Name":"x"}}]}`
	code, body = f.do("POST", "composite", req, false)
	entries := compositeEntries(t, body)
	if code != 200 || len(entries) != 4 {
		t.Fatalf("composite: %d %v", code, body)
	}
	for _, e := range entries {
		if status, _ := e["httpStatusCode"].(float64); status != 404 {
			t.Fatalf("%v: want 404, got %v", e["referenceId"], e)
		}
		if got, _ := failure(e["body"]); got != "NOT_FOUND" {
			t.Fatalf("%v: want NOT_FOUND, got %v", e["referenceId"], e)
		}
	}
}

func TestPlan31HiddenField(t *testing.T) {
	f := newReadFixture(t)
	f.profile("Standard User")
	id := f.account

	code, body := f.do("GET", "composite/sobjects/Account?ids="+id+"&fields=Name,Compat_Text__c", "", false)
	wantFailure(t, "retrieve hidden field", code, body, 400, "INVALID_FIELD")
	code, body = f.do("GET", "composite/sobjects/Account?ids="+id+"&fields=Name", "", false)
	list, _ := body.([]any)
	if code != 200 || len(list) != 1 {
		t.Fatalf("retrieve visible: %d %v", code, body)
	}
	if row, _ := list[0].(map[string]any); row["Name"] != "Read fixture" || row["Compat_Text__c"] != nil {
		t.Fatalf("retrieve leaked: %v", row)
	}
	code, body = f.do("POST", "composite/sobjects", `{"allOrNone":false,"records":[{"attributes":{"type":"Account"},"Name":"x","Compat_Text__c":"y"}]}`, false)
	if code != 200 || itemErrorCode(t, body) != "INVALID_FIELD" {
		t.Fatalf("create hidden field: %d %v", code, body)
	}
}
