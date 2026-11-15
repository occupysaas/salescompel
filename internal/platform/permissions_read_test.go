package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// readFixture is an OAuth user (System Administrator) with an access token
// and one Account record, served through the real HTTP handler.
type readFixture struct {
	t       *testing.T
	s       *Store
	h       http.Handler
	tenant  string
	user    string
	token   string
	account string
}

func newReadFixture(t *testing.T) readFixture {
	t.Helper()
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	f := readFixture{t: t, s: s, h: NewHandler(s, ""), tenant: g.User.Tenant, user: g.User.ID, token: v.Access}
	code, body := f.do("POST", "sobjects/Account", `{"Name":"Read fixture","Compat_Text__c":"secret"}`, false)
	if code != 201 {
		t.Fatal(code, body)
	}
	f.account = body.(map[string]any)["id"].(string)
	return f
}

// do sends a request under /services/data/v65.0/; ui marks it as coming from
// the first-party UI session.
func (f readFixture) do(method, path, body string, ui bool) (int, any) {
	f.t.Helper()
	r := httptest.NewRequest(method, apiBase+path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+f.token)
	if ui {
		r = withUISession(r)
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	var out any
	if w.Body.Len() > 0 {
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			f.t.Fatalf("non-json response %s", w.Body.String())
		}
	}
	return w.Code, out
}

func (f readFixture) query(q string) (int, any) {
	f.t.Helper()
	return f.do("GET", "query?q="+url.QueryEscape(q), "", false)
}

func (f readFixture) profile(name string) { moveToProfile(f.t, f.s, f.tenant, f.user, name) }

// revoke removes a profile's object permission on object and bumps the
// permission revision, as a setup write would.
func (f readFixture) revoke(profile, object string) {
	f.t.Helper()
	ctx := context.Background()
	tx, e := f.s.pool.Begin(ctx)
	if e != nil {
		f.t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	for _, sql := range []string{
		"SELECT set_config('salescompel.tenant_id',$1,true)",
		`DELETE FROM sc_setup.object_permissions o USING sc_setup.profiles p
			WHERE p.tenant_id=$1 AND p.name=$2 AND o.tenant_id=$1 AND o.parent_id=p.permission_set_id AND o.sobject_type=$3`,
		"UPDATE sc_setup.revisions SET revision=revision+1 WHERE tenant_id=$1",
	} {
		args := []any{f.tenant, profile, object}
		if !strings.Contains(sql, "$2") {
			args = args[:1]
		}
		if _, e = tx.Exec(ctx, sql, args...); e != nil {
			f.t.Fatalf("%s: %v", sql, e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		f.t.Fatal(e)
	}
}

// failure returns the first errorCode and fields of an error response.
func failure(body any) (string, []string) {
	list, ok := body.([]any)
	if !ok || len(list) == 0 {
		return "", nil
	}
	m, _ := list[0].(map[string]any)
	code, _ := m["errorCode"].(string)
	var fields []string
	if raw, ok := m["fields"].([]any); ok {
		for _, v := range raw {
			s, _ := v.(string)
			fields = append(fields, s)
		}
	}
	return code, fields
}

func wantFailure(t *testing.T, what string, code int, body any, status int, errorCode string) {
	t.Helper()
	if got, _ := failure(body); code != status || got != errorCode {
		t.Fatalf("%s: %d %v, want %d %s", what, code, body, status, errorCode)
	}
}

func describedObjects(body any) map[string]bool {
	out := map[string]bool{}
	m, _ := body.(map[string]any)
	list, _ := m["sobjects"].([]any)
	for _, o := range list {
		d, _ := o.(map[string]any)
		name, _ := d["name"].(string)
		out[name] = true
	}
	return out
}

func describedFields(body any) map[string]bool {
	out := map[string]bool{}
	m, _ := body.(map[string]any)
	list, _ := m["fields"].([]any)
	for _, o := range list {
		d, _ := o.(map[string]any)
		name, _ := d["name"].(string)
		out[name] = true
	}
	return out
}

func TestHiddenObjectTraversal(t *testing.T) {
	f := newReadFixture(t)
	f.profile("Standard User")
	if code, body := f.query("SELECT Account.Name FROM Contact"); code != 200 {
		t.Fatal("visible traversal", code, body)
	}
	f.revoke("Standard User", "Account")
	// D33-01: a hidden object in a traversal or subquery is INVALID_TYPE, as
	// for an object that doesn't exist.
	for _, q := range []string{"SELECT Account.Name FROM Contact", "SELECT (SELECT Id FROM Contacts) FROM Account", "SELECT Id FROM Account"} {
		code, body := f.query(q)
		wantFailure(t, q, code, body, 400, "INVALID_TYPE")
	}
	_, unknown := f.query("SELECT Id FROM Nonexistent__c")
	if _, hidden := f.query("SELECT Id FROM Account"); stringJSON(hidden) != strings.Replace(stringJSON(unknown), "Nonexistent__c", "Account", -1) {
		t.Fatalf("hidden %v differs from unknown %v", hidden, unknown)
	}
	// The child side of a subquery is checked too.
	f2 := newReadFixture(t)
	f2.profile("Standard User")
	f2.revoke("Standard User", "Contact")
	code, body := f2.query("SELECT (SELECT Id FROM Contacts) FROM Account")
	wantFailure(t, "hidden child", code, body, 400, "INVALID_TYPE")
}

func TestHiddenObjectDescribe(t *testing.T) {
	f := newReadFixture(t)
	f.profile("Standard User")
	f.revoke("Standard User", "Account")
	code, body := f.do("GET", "sobjects", "", false)
	if objects := describedObjects(body); code != 200 || objects["Account"] || !objects["Contact"] || !objects["User"] || objects["PermissionSet"] {
		t.Fatal(code, objects)
	}
	// D33-02: describing, reading or writing a hidden object is NOT_FOUND.
	for _, r := range []struct{ method, path, body string }{
		{"GET", "sobjects/Account/describe", ""},
		{"GET", "sobjects/Account/" + f.account, ""},
		{"PATCH", "sobjects/Account/" + f.account, `{"Name":"x"}`},
		{"DELETE", "sobjects/Account/" + f.account, ""},
		{"POST", "sobjects/Account", `{"Name":"x"}`},
		{"GET", "sobjects/PermissionSet/describe", ""},
	} {
		code, body := f.do(r.method, r.path, r.body, false)
		wantFailure(t, r.method+" "+r.path, code, body, 404, "NOT_FOUND")
	}
}

func TestHiddenFieldEverywhere(t *testing.T) {
	f := newReadFixture(t)
	f.profile("Standard User")
	code, body := f.do("GET", "sobjects/Account/describe", "", false)
	if fields := describedFields(body); code != 200 || fields["Compat_Text__c"] || !fields["Name"] {
		t.Fatal(code, fields)
	}
	// D33-03: a hidden field is INVALID_FIELD in every clause.
	for _, q := range []string{
		"SELECT Compat_Text__c FROM Account",
		"SELECT Id FROM Account WHERE Compat_Text__c = 'secret'",
		"SELECT Id FROM Account ORDER BY Compat_Text__c",
		"SELECT COUNT(Id) FROM Account GROUP BY Compat_Text__c",
	} {
		code, body := f.query(q)
		wantFailure(t, q, code, body, 400, "INVALID_FIELD")
	}
	code, body = f.do("GET", "sobjects/Account/"+f.account+"?fields=Name,Compat_Text__c", "", false)
	wantFailure(t, "?fields=", code, body, 400, "INVALID_FIELD")
	if _, fields := failure(body); len(fields) != 1 || fields[0] != "Compat_Text__c" {
		t.Fatal(body)
	}
	code, body = f.do("GET", "sobjects/Account/"+f.account+"?fields=Name", "", false)
	row, _ := body.(map[string]any)
	if _, leaked := row["Compat_Text__c"]; code != 200 || row["Name"] != "Read fixture" || leaked || row["Phone"] != nil {
		t.Fatal(code, body)
	}
	code, body = f.do("GET", "sobjects/Account/"+f.account, "", false)
	if row, _ := body.(map[string]any); code != 200 || row["Compat_Text__c"] != nil {
		t.Fatal(code, body)
	}
}

func TestAPIDisabledRefused(t *testing.T) {
	f := newReadFixture(t)
	f.profile("Minimum Access")
	// D33-07: API Enabled gates every REST and Composite call.
	for _, r := range []struct{ method, path, body string }{
		{"GET", "sobjects", ""},
		{"GET", "sobjects/Account/describe", ""},
		{"GET", "query?q=SELECT+Id+FROM+Account", ""},
		{"POST", "composite", compositeJSON(compositeCreate)},
	} {
		code, body := f.do(r.method, r.path, r.body, false)
		wantFailure(t, r.method+" "+r.path, code, body, 403, "API_CURRENTLY_DISABLED")
	}
}

func TestUISessionExempt(t *testing.T) {
	f := newReadFixture(t)
	f.profile("Minimum Access")
	code, body := f.do("GET", "sobjects", "", true)
	if code != 200 || len(describedObjects(body)) != 0 {
		t.Fatal(code, body)
	}
	code, body = f.do("POST", "composite", compositeJSON(compositeCreate), true)
	if got, _ := failure(body); code != 200 || got == "API_CURRENTLY_DISABLED" {
		t.Fatal(code, body)
	}
}

func TestSetupObjectsNeedManageUsers(t *testing.T) {
	f := newReadFixture(t)
	f.profile("Standard User")
	// D33-09: a setup object without Manage Users or View All Data is hidden.
	code, body := f.query("SELECT Id FROM PermissionSet")
	wantFailure(t, "PermissionSet", code, body, 400, "INVALID_TYPE")
	if code, body = f.query("SELECT Id FROM User"); code != 200 {
		t.Fatal(code, body)
	}
}

func TestSecurityEnforcedUsesPermissions(t *testing.T) {
	f := newReadFixture(t)
	q := "SELECT Id, Compat_Text__c FROM Account WITH SECURITY_ENFORCED"
	if code, body := f.query(q); code != 200 {
		t.Fatal("admin", code, body)
	}
	f.profile("Standard User")
	// D33-03 (G33-05): the same INVALID_FIELD as without the clause.
	code, body := f.query(q)
	wantFailure(t, q, code, body, 400, "INVALID_FIELD")
}
