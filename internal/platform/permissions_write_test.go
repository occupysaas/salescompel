package platform

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"salescompel/internal/metadata"
)

// writeFixture is readFixture's System Administrator plus the OAuth app, so
// further users in the tenant can get tokens.
type writeFixture struct {
	readFixture
	app string
	now time.Time
}

func newWriteFixture(t *testing.T) writeFixture {
	t.Helper()
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	f := readFixture{t: t, s: s, h: NewHandler(s, ""), tenant: g.User.Tenant, user: g.User.ID, token: v.Access}
	code, body := f.do("POST", "sobjects/Account", `{"Name":"Write fixture","Compat_Text__c":"secret"}`, false)
	if code != 201 {
		t.Fatal(code, body)
	}
	f.account = body.(map[string]any)["id"].(string)
	return writeFixture{readFixture: f, app: g.ClientID, now: n}
}

// newUser creates a CLI user on profile and returns a fixture acting as them.
func (w writeFixture) newUser(name, profile string) readFixture {
	w.t.Helper()
	ctx := context.Background()
	username := name + "." + w.tenant + "@example.com"
	id, e := w.s.CreateUser(ctx, w.tenant, username, "long-password", "restricted")
	if e != nil {
		w.t.Fatal(e)
	}
	if profile != "Standard User" {
		moveToProfile(w.t, w.s, w.tenant, id, profile)
	}
	u, e := w.s.VerifyLogin(ctx, username, "long-password", w.now)
	if e != nil {
		w.t.Fatal(e)
	}
	v := issueTest(w.t, w.s, TokenGrant{ClientID: w.app, User: u, Scope: "api"}, w.now)
	f := w.readFixture
	f.user, f.token = id, v.Access
	return f
}

// created posts a record and returns its ID.
func (f readFixture) created(object, body string) string {
	f.t.Helper()
	code, out := f.do("POST", "sobjects/"+object, body, false)
	if code != 201 {
		f.t.Fatalf("create %s %s: %d %v", object, body, code, out)
	}
	return out.(map[string]any)["id"].(string)
}

// one returns the first record's field from a query that must succeed.
func (f readFixture) one(q, field string) string {
	f.t.Helper()
	code, body := f.query(q)
	m, _ := body.(map[string]any)
	records, _ := m["records"].([]any)
	if code != 200 || len(records) == 0 {
		f.t.Fatalf("%s: %d %v", q, code, body)
	}
	v, _ := records[0].(map[string]any)[field].(string)
	return v
}

func (f readFixture) profileID(name string) string {
	return f.one("SELECT Id FROM Profile WHERE Name = '"+name+"'", "Id")
}

func (f readFixture) setupCount(sql string, args ...any) int {
	f.t.Helper()
	ctx := context.Background()
	tx, e := f.s.pool.Begin(ctx)
	if e != nil {
		f.t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", f.tenant)
	var n int
	if e = tx.QueryRow(ctx, sql, append([]any{f.tenant}, args...)...).Scan(&n); e != nil {
		f.t.Fatal(e)
	}
	return n
}

func (f readFixture) revision() int {
	return f.setupCount("SELECT revision::int FROM sc_setup.revisions WHERE tenant_id=$1")
}

func wantFields(t *testing.T, what string, body any, want ...string) {
	t.Helper()
	if _, got := failure(body); !slices.Equal(got, want) {
		t.Fatalf("%s: fields %v, want %v (%v)", what, got, want, body)
	}
}

func TestStandardUserCannotEditHiddenField(t *testing.T) {
	f := newWriteFixture(t)
	std := f.newUser("std", "Standard User")
	code, body := std.do("PATCH", "sobjects/Account/"+f.account, `{"Compat_Text__c":"changed"}`, false)
	wantFailure(t, "hidden field update", code, body, 400, "INVALID_FIELD")
	wantFields(t, "hidden field update", body, "Compat_Text__c")
	if !strings.Contains(fmt.Sprint(body), "No such column 'Compat_Text__c' on sobject of type Account") {
		t.Fatalf("message %v", body)
	}
	code, body = std.do("POST", "sobjects/Account", `{"Name":"x","compat_text__c":"y"}`, false)
	wantFailure(t, "hidden field create", code, body, 400, "INVALID_FIELD")
	if code, body = std.do("PATCH", "sobjects/Account/"+f.account, `{"Phone":"555"}`, false); code != 204 {
		t.Fatalf("editable field: %d %v", code, body)
	}
}

func TestReadOnlyCannotCreate(t *testing.T) {
	f := newWriteFixture(t)
	ro := f.newUser("ro", "Read Only")
	code, body := ro.do("POST", "sobjects/Contact", `{"LastName":"Refused"}`, false)
	wantFailure(t, "read only create", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = ro.do("PATCH", "sobjects/Account/"+f.account, `{}`, false)
	wantFailure(t, "read only update", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = ro.do("DELETE", "sobjects/Account/"+f.account, "", false)
	wantFailure(t, "read only delete", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	if code, _ = ro.do("GET", "sobjects/Account/"+f.account, "", false); code != 200 {
		t.Fatalf("read only read: %d", code)
	}
}

func TestObjectGrantRejectsCreateWithoutRead(t *testing.T) {
	f := newWriteFixture(t)
	set := f.created("PermissionSet", `{"Name":"Create_Only","Label":"Create only"}`)
	before := f.revision()
	code, body := f.do("POST", "sobjects/ObjectPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","PermissionsCreate":true,"PermissionsRead":false}`, false)
	wantFailure(t, "create without read", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	wantFields(t, "create without read", body, "PermissionsCreate")
	if !strings.Contains(fmt.Sprint(body), "Permission Create depends on permission(s): Read") {
		t.Fatalf("message %v", body)
	}
	if f.revision() != before {
		t.Fatal("refused write bumped the revision")
	}
	grant := f.created("ObjectPermissions", `{"ParentId":"`+set+`","SobjectType":"account","PermissionsRead":true,"PermissionsCreate":true}`)
	if f.revision() != before+1 {
		t.Fatal("setup write did not bump the revision")
	}
	if got := f.one("SELECT SobjectType FROM ObjectPermissions WHERE Id = '"+grant+"'", "SobjectType"); got != "Account" {
		t.Fatalf("SobjectType stored as %q", got)
	}
	code, body = f.do("PATCH", "sobjects/ObjectPermissions/"+grant, `{"PermissionsRead":false}`, false)
	wantFailure(t, "update drops read", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	code, body = f.do("POST", "sobjects/ObjectPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","PermissionsRead":true}`, false)
	wantFailure(t, "duplicate grant", code, body, 400, "DUPLICATE_VALUE")
	if !strings.Contains(fmt.Sprint(body), grant) {
		t.Fatalf("duplicate message lacks existing id: %v", body)
	}
	for _, c := range []struct{ parent, code string }{{"bad", "MALFORMED_ID"}, {f.user, "INVALID_CROSS_REFERENCE_KEY"}, {"0PS000000000000GAA", "INVALID_CROSS_REFERENCE_KEY"}, {"0PS000000000000AAA", "MALFORMED_ID"}} {
		code, body = f.do("POST", "sobjects/ObjectPermissions", `{"ParentId":"`+c.parent+`","SobjectType":"Contact","PermissionsRead":true}`, false)
		wantFailure(t, "parent "+c.parent, code, body, 400, c.code)
		wantFields(t, "parent "+c.parent, body, "ParentId")
	}
}

func TestGrantOnSetupObjectRefused(t *testing.T) {
	f := newWriteFixture(t)
	set := f.created("PermissionSet", `{"Name":"Setup_Grants","Label":"Setup grants"}`)
	for _, c := range []struct{ object, body, field string }{
		{"ObjectPermissions", `"SobjectType":"User","PermissionsRead":true`, "SobjectType"},
		{"ObjectPermissions", `"SobjectType":"PermissionSet","PermissionsRead":true`, "SobjectType"},
		{"ObjectPermissions", `"SobjectType":"Nope__c","PermissionsRead":true`, "SobjectType"},
		{"FieldPermissions", `"SobjectType":"User","Field":"User.Email","PermissionsRead":true`, "SobjectType"},
		{"FieldPermissions", `"SobjectType":"Account","Field":"User.Email","PermissionsRead":true`, "Field"},
		{"FieldPermissions", `"SobjectType":"Account","Field":"Account.Nope__c","PermissionsRead":true`, "Field"},
	} {
		code, body := f.do("POST", "sobjects/"+c.object, `{"ParentId":"`+set+`",`+c.body+`}`, false)
		wantFailure(t, c.body, code, body, 400, "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST")
		wantFields(t, c.body, body, c.field)
	}
	code, body := f.do("POST", "sobjects/FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","Field":"Account.Name","PermissionsRead":true}`, false)
	wantFailure(t, "name field", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	code, body = f.do("POST", "sobjects/FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","Field":"Account.Phone","PermissionsRead":false}`, false)
	wantFailure(t, "grants nothing", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	code, body = f.do("POST", "sobjects/FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","Field":"Account.Phone","PermissionsEdit":true}`, false)
	wantFailure(t, "edit without read", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	wantFields(t, "edit without read", body, "PermissionsEdit")
	if n := f.setupCount("SELECT count(*)::int FROM sc_setup.field_permissions WHERE tenant_id=$1 AND parent_id=$2", set); n != 0 {
		t.Fatalf("%d refused grants stored", n)
	}
	grant := f.created("FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","Field":"account.phone","PermissionsRead":true}`)
	if code, body = f.do("PATCH", "sobjects/FieldPermissions/"+grant, `{"PermissionsRead":false}`, false); code != 204 {
		t.Fatalf("revoking read: %d %v", code, body)
	}
	if n := f.setupCount("SELECT count(*)::int FROM sc_setup.field_permissions WHERE tenant_id=$1 AND id=$2", grant); n != 0 {
		t.Fatal("a row granting nothing was kept")
	}
}

func TestAssignPermsetGrantsField(t *testing.T) {
	f := newWriteFixture(t)
	std := f.newUser("std", "Standard User")
	const q = "SELECT Compat_Text__c FROM Account"
	code, body := std.query(q)
	wantFailure(t, "before assignment", code, body, 400, "INVALID_FIELD")
	set := f.created("PermissionSet", `{"Name":"Compat_Text_Read","Label":"Compat text read"}`)
	f.created("FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","Field":"Account.Compat_Text__c","PermissionsRead":true}`)
	f.created("PermissionSetAssignment", `{"AssigneeId":"`+std.user+`","PermissionSetId":"`+set[:15]+`"}`)
	if got := std.one(q, "Compat_Text__c"); got != "secret" {
		t.Fatalf("after assignment got %q", got)
	}
	code, body = std.do("PATCH", "sobjects/Account/"+f.account, `{"Compat_Text__c":"changed"}`, false)
	wantFailure(t, "readable not editable", code, body, 400, "INVALID_FIELD_FOR_INSERT_UPDATE")
	wantFields(t, "readable not editable", body, "Compat_Text__c")
	code, body = f.do("POST", "sobjects/PermissionSetAssignment", `{"AssigneeId":"`+std.user+`","PermissionSetId":"`+set+`"}`, false)
	wantFailure(t, "duplicate assignment", code, body, 400, "DUPLICATE_VALUE")
}

func TestProfileAssignmentRowsSystemMaintained(t *testing.T) {
	f := newWriteFixture(t)
	std := f.newUser("std", "Standard User")
	psa := f.one("SELECT Id FROM PermissionSetAssignment WHERE AssigneeId = '"+std.user+"'", "Id")
	readOnly := f.profileID("Read Only")
	readOnlySet := f.one("SELECT Id FROM PermissionSet WHERE ProfileId = '"+readOnly+"'", "Id")
	if code, body := f.do("PATCH", "sobjects/User/"+std.user, `{"ProfileId":"`+readOnly+`"}`, false); code != 204 {
		t.Fatalf("profile change: %d %v", code, body)
	}
	if got := f.one("SELECT PermissionSetId FROM PermissionSetAssignment WHERE Id = '"+psa+"'", "PermissionSetId"); got != readOnlySet {
		t.Fatalf("assignment points at %s, want %s", got, readOnlySet)
	}
	if n := f.setupCount("SELECT count(*)::int FROM sc_setup.permission_set_assignments WHERE tenant_id=$1 AND assignee_id=$2", std.user); n != 1 {
		t.Fatalf("%d assignments after the move", n)
	}
	code, body := std.do("POST", "sobjects/Contact", `{"LastName":"Now read only"}`, false)
	wantFailure(t, "moved user create", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = f.do("DELETE", "sobjects/PermissionSetAssignment/"+psa, "", false)
	wantFailure(t, "delete profile assignment", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = f.do("POST", "sobjects/PermissionSetAssignment", `{"AssigneeId":"`+std.user+`","PermissionSetId":"`+readOnlySet+`"}`, false)
	wantFailure(t, "create profile assignment", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = f.do("DELETE", "sobjects/PermissionSet/"+readOnlySet, "", false)
	wantFailure(t, "delete profile-owned set", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = f.do("POST", "sobjects/PermissionSet", `{"Name":"Owned","Label":"Owned","IsOwnedByProfile":true}`, false)
	wantFailure(t, "create profile-owned set", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	if code, body = f.do("PATCH", "sobjects/PermissionSet/"+readOnlySet, `{"Description":"Edited"}`, false); code != 204 {
		t.Fatalf("update profile-owned set: %d %v", code, body)
	}
	code, body = f.do("PATCH", "sobjects/Profile/"+readOnly, `{"Description":"x"}`, false)
	wantFailure(t, "profile update", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = f.do("DELETE", "sobjects/User/"+std.user, "", false)
	wantFailure(t, "user delete", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
}

func TestLastAdminCannotBeDemoted(t *testing.T) {
	f := newWriteFixture(t)
	standard := f.profileID("Standard User")
	admin := f.profileID("System Administrator")
	adminSet := f.one("SELECT Id FROM PermissionSet WHERE ProfileId = '"+admin+"'", "Id")
	for _, c := range []struct{ path, body string }{
		{"sobjects/User/" + f.user, `{"ProfileId":"` + standard + `"}`},
		{"sobjects/User/" + f.user, `{"IsActive":false}`},
		{"sobjects/PermissionSet/" + adminSet, `{"PermissionsManageUsers":false}`},
		{"sobjects/PermissionSet/" + adminSet, `{"PermissionsModifyAllData":false}`},
	} {
		code, body := f.do("PATCH", c.path, c.body, false)
		wantFailure(t, c.body, code, body, 400, "INVALID_OPERATION")
	}
	if got := f.one("SELECT ProfileId FROM User WHERE Id = '"+f.user+"'", "ProfileId"); got != admin {
		t.Fatalf("refused demotion changed the profile to %s", got)
	}
	// Modify All Data alone doesn't count as Manage Users (D33-28).
	other := f.newUser("other", "Standard User")
	set := f.created("PermissionSet", `{"Name":"MAD_Only","Label":"MAD only","PermissionsModifyAllData":true}`)
	assignment := f.created("PermissionSetAssignment", `{"AssigneeId":"`+other.user+`","PermissionSetId":"`+set+`"}`)
	code, body := f.do("PATCH", "sobjects/User/"+f.user, `{"ProfileId":"`+standard+`"}`, false)
	wantFailure(t, "demotion with MAD-only holder", code, body, 400, "INVALID_OPERATION")
	if code, body = f.do("PATCH", "sobjects/PermissionSet/"+set, `{"PermissionsManageUsers":true}`, false); code != 204 {
		t.Fatalf("grant manage users: %d %v", code, body)
	}
	if code, body = f.do("PATCH", "sobjects/User/"+f.user, `{"ProfileId":"`+standard+`"}`, false); code != 204 {
		t.Fatalf("demotion with another admin: %d %v", code, body)
	}
	code, body = other.do("DELETE", "sobjects/PermissionSetAssignment/"+assignment, "", false)
	wantFailure(t, "unassign last admin", code, body, 400, "INVALID_OPERATION")
	code, body = other.do("DELETE", "sobjects/PermissionSet/"+set, "", false)
	wantFailure(t, "delete last admin set", code, body, 400, "INVALID_OPERATION")
}

func TestRESTUserCreateHasNoPassword(t *testing.T) {
	f := newWriteFixture(t)
	standard := f.profileID("Standard User")
	username := "Rest.User." + f.tenant + "@Example.com"
	fields := `"LastName":"User","Alias":"ruser","Email":"rest@example.com","ProfileId":"` + standard + `","TimeZoneSidKey":"Europe/London","LocaleSidKey":"en_GB","LanguageLocaleKey":"en_US","EmailEncodingKey":"UTF-8"`
	code, body := f.do("POST", "sobjects/User", `{"Username":"x@example.com","LastName":"User"}`, false)
	wantFailure(t, "missing fields", code, body, 400, "REQUIRED_FIELD_MISSING")
	wantFields(t, "missing fields", body, "Alias", "Email", "ProfileId", "TimeZoneSidKey", "LocaleSidKey", "LanguageLocaleKey", "EmailEncodingKey")
	code, body = f.do("POST", "sobjects/User", `{"Username":"not-an-email",`+fields+`}`, false)
	wantFailure(t, "bad username", code, body, 400, "INVALID_EMAIL_ADDRESS")
	code, body = f.do("POST", "sobjects/User", `{"Username":"`+username+`",`+strings.Replace(fields, standard, "00e000000000000AAA", 1)+`}`, false)
	wantFailure(t, "unknown profile", code, body, 400, "INVALID_CROSS_REFERENCE_KEY")
	id := f.created("User", `{"Username":"`+username+`",`+fields+`}`)
	for _, password := range []string{"", "long-password", "anything-at-all"} {
		if _, e := f.s.VerifyLogin(context.Background(), username, password, time.Now()); e == nil {
			t.Fatalf("login with %q succeeded", password)
		}
	}
	if got := f.one("SELECT Username FROM User WHERE Id = '"+id+"'", "Username"); got != username {
		t.Fatalf("username stored as %q", got)
	}
	if n := f.setupCount("SELECT count(*)::int FROM sc_setup.permission_set_assignments WHERE tenant_id=$1 AND assignee_id=$2", id); n != 1 {
		t.Fatalf("%d profile assignments for the new user", n)
	}
	renamed := "renamed." + f.tenant + "@example.com"
	if code, body = f.do("PATCH", "sobjects/User/"+id, `{"Username":"`+renamed+`"}`, false); code != 204 {
		t.Fatalf("rename: %d %v", code, body)
	}
	other := newWriteFixture(t)
	code, body = other.do("POST", "sobjects/User", `{"Username":"`+strings.ToUpper(renamed)+`",`+strings.Replace(fields, standard, other.profileID("Standard User"), 1)+`}`, false)
	wantFailure(t, "duplicate across tenants", code, body, 400, "DUPLICATE_USERNAME")
	code, body = f.do("PATCH", "sobjects/User/"+id, `{"Username":"Alice`+other.tenant+`@example.com"}`, false)
	wantFailure(t, "rename to a taken username", code, body, 400, "DUPLICATE_USERNAME")
}

func TestDeactivatedUserTokenRefused(t *testing.T) {
	f := newWriteFixture(t)
	std := f.newUser("std", "Standard User")
	if code, body := std.do("GET", "sobjects/Account/"+f.account, "", false); code != 200 {
		t.Fatalf("before deactivation: %d %v", code, body)
	}
	if code, body := f.do("PATCH", "sobjects/User/"+std.user, `{"IsActive":false}`, false); code != 204 {
		t.Fatalf("deactivate: %d %v", code, body)
	}
	code, body := std.do("GET", "sobjects/Account/"+f.account, "", false)
	wantFailure(t, "deactivated token", code, body, 401, "INVALID_SESSION_ID")
}

func TestDeletedFieldGrantRemoved(t *testing.T) {
	f := newWriteFixture(t)
	ctx := context.Background()
	o, e := metadata.CreateObject(ctx, f.s.pool, f.tenant, metadata.ObjectInput{Name: "Granted__c", Label: "Granted", PluralLabel: "Granted"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.s.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+o.TableSQL()+" CASCADE") })
	if _, e = metadata.CreateField(ctx, f.s.pool, f.tenant, metadata.FieldInput{Object: "Granted__c", Name: "Note__c", Label: "Note", Type: "Text", Length: 20}); e != nil {
		t.Fatal(e)
	}
	set := f.created("PermissionSet", `{"Name":"Granted","Label":"Granted"}`)
	f.created("ObjectPermissions", `{"ParentId":"`+set+`","SobjectType":"Granted__c","PermissionsRead":true}`)
	f.created("FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Granted__c","Field":"Granted__c.Note__c","PermissionsRead":true}`)
	const fields = "SELECT count(*)::int FROM sc_setup.field_permissions WHERE tenant_id=$1 AND sobject_type='Granted__c'"
	const objects = "SELECT count(*)::int FROM sc_setup.object_permissions WHERE tenant_id=$1 AND sobject_type='Granted__c'"
	before := f.revision()
	if e = metadata.DeleteField(ctx, f.s.pool, f.tenant, "Granted__c", "Note__c"); e != nil {
		t.Fatal(e)
	}
	if n := f.setupCount(fields); n != 0 || f.revision() != before+1 {
		t.Fatalf("after field delete: %d grants, revision %d (was %d)", n, f.revision(), before)
	}
	if e = metadata.DeleteObject(ctx, f.s.pool, f.tenant, "Granted__c"); e != nil {
		t.Fatal(e)
	}
	if n := f.setupCount(objects); n != 0 || f.revision() != before+2 {
		t.Fatalf("after object delete: %d grants, revision %d", n, f.revision())
	}
	for _, c := range []error{
		metadata.DeleteField(ctx, f.s.pool, f.tenant, "Account", "Phone"),
		metadata.DeleteObject(ctx, f.s.pool, f.tenant, "Account"),
		metadata.DeleteObject(ctx, f.s.pool, f.tenant, "User"),
	} {
		if c == nil {
			t.Fatal("deleted a standard field or object")
		}
	}
	if _, e = metadata.CreateField(ctx, f.s.pool, f.tenant, metadata.FieldInput{Object: "User", Name: "Extra__c", Label: "Extra", Type: "Text", Length: 20}); e == nil {
		t.Fatal("custom field created on a setup object")
	}
}

func TestCompositeDeniedSubrequest(t *testing.T) {
	f := newWriteFixture(t)
	ctx := context.Background()
	o, e := metadata.CreateObject(ctx, f.s.pool, f.tenant, metadata.ObjectInput{Name: "Hidden__c", Label: "Hidden", PluralLabel: "Hidden"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.s.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+o.TableSQL()+" CASCADE") })
	hidden := f.created("Hidden__c", `{"Name":"Unseen"}`)
	std := f.newUser("std", "Standard User")
	composite := func(subs string) (int, []any) {
		code, body := std.do("POST", "composite", compositeJSON(subs), false)
		m, _ := body.(map[string]any)
		out, _ := m["compositeResponse"].([]any)
		if code != 200 || len(out) == 0 {
			t.Fatalf("composite: %d %v", code, body)
		}
		return code, out
	}
	sub := func(v any) (int, string) {
		m := v.(map[string]any)
		code, _ := failure(m["body"])
		return int(m["httpStatusCode"].(float64)), code
	}
	allowed := `{"method":"POST","url":"/services/data/v65.0/sobjects/Account","referenceId":"ok","body":{"Name":"Composite rolled back"}}`
	for _, c := range []struct {
		denied string
		status int
		code   string
	}{
		{`{"method":"PATCH","url":"/services/data/v65.0/sobjects/Account/` + f.account + `","referenceId":"no","body":{"Compat_Text__c":"x"}}`, 400, "INVALID_FIELD"},
		{`{"method":"PATCH","url":"/services/data/v65.0/sobjects/Hidden__c/` + hidden + `","referenceId":"no","body":{}}`, 404, "NOT_FOUND"},
		{`{"method":"POST","url":"/services/data/v65.0/sobjects/Hidden__c","referenceId":"no","body":{"Name":"x"}}`, 404, "NOT_FOUND"},
		{`{"method":"DELETE","url":"/services/data/v65.0/sobjects/Hidden__c/` + hidden + `","referenceId":"no"}`, 404, "NOT_FOUND"},
	} {
		_, out := composite(allowed + "," + c.denied)
		if status, code := sub(out[0]); status != 400 || code != "PROCESSING_HALTED" {
			t.Fatalf("%s: allowed subrequest %d %s", c.denied, status, code)
		}
		if status, code := sub(out[1]); status != c.status || code != c.code {
			t.Fatalf("%s: denied subrequest %d %s", c.denied, status, code)
		}
	}
	if code, body := f.query("SELECT Id FROM Account WHERE Name = 'Composite rolled back'"); code != 200 || !strings.Contains(fmt.Sprint(body), "totalSize:0") {
		t.Fatalf("rolled-back create persisted: %d %v", code, body)
	}
	// The REST answers match for the same operations.
	code, body := std.do("PATCH", "sobjects/Hidden__c/"+hidden, `{}`, false)
	wantFailure(t, "REST hidden patch", code, body, 404, "NOT_FOUND")
	ro := f.newUser("ro", "Read Only")
	code, body = ro.do("POST", "composite", compositeJSON(`{"method":"POST","url":"/services/data/v65.0/sobjects/Contact","referenceId":"c","body":{"LastName":"x"}}`), false)
	out := body.(map[string]any)["compositeResponse"].([]any)
	if status, errorCode := sub(out[0]); code != 200 || status != 403 || errorCode != "INSUFFICIENT_ACCESS_OR_READONLY" {
		t.Fatalf("read only composite create: %d %v", code, body)
	}
}

func TestStandardUserCannotWriteSetupObjects(t *testing.T) {
	f := newWriteFixture(t)
	std := f.newUser("std", "Standard User")
	const sets = "SELECT count(*)::int FROM sc_setup.permission_sets WHERE tenant_id=$1"
	before, revision := f.setupCount(sets), f.revision()
	code, body := std.do("POST", "sobjects/PermissionSet", `{"Name":"Sneaky","Label":"Sneaky"}`, false)
	wantFailure(t, "standard user permission set insert", code, body, 404, "NOT_FOUND") // D33-09: hidden
	code, body = std.do("PATCH", "sobjects/User/"+std.user, `{"LastName":"Changed"}`, false)
	wantFailure(t, "standard user self update", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY") // D33-08
	code, body = std.do("PATCH", "sobjects/User/"+f.user, `{"IsActive":false}`, false)
	wantFailure(t, "standard user deactivates admin", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	if f.setupCount(sets) != before || f.revision() != revision {
		t.Fatal("refused setup writes changed setup data")
	}
	restricted := Principal{Tenant: f.tenant, User: "reader", Role: "restricted"}
	if _, e := f.s.Create(context.Background(), restricted, "PermissionSet", map[string]any{"Name": "X", "Label": "X"}); e == nil {
		t.Fatal("restricted principal created a permission set")
	}
}
