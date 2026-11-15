package records

import (
	"errors"
	"strings"
	"testing"

	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/policy/policytest"
)

func apiError(t *testing.T, err error) *APIError {
	t.Helper()
	var api *APIError
	if !errors.As(err, &api) {
		t.Fatalf("got %v, want an API error", err)
	}
	return api
}

func setupSnapshot() *metadata.Snapshot {
	objects := append(metadata.StandardObjects(), metadata.SetupObjects()...)
	for i := range objects {
		if metadata.IsSetupObject(objects[i].Name) {
			continue
		}
		objects[i].Createable, objects[i].Updateable, objects[i].Deletable = true, true, true
	}
	return &metadata.Snapshot{Objects: objects}
}

// Setup writes without Manage Users or Modify All Data are refused before
// any storage is touched (D33-08).
func TestSetupWriteNeedsManageUsers(t *testing.T) {
	c := setupSnapshot()
	p := Principal{Tenant: "t", Catalog: c, Perms: policy.Union(policy.PermissionSet{System: policy.SystemPerms{APIEnabled: true, ViewAllData: true}})}
	api := apiError(t, func() error {
		_, e := setupWrite(t.Context(), nil, p, "create", "PermissionSet", "", map[string]any{"Name": "X"})
		return e
	}())
	if api.Status != 403 || api.Code != "INSUFFICIENT_ACCESS_OR_READONLY" {
		t.Fatalf("got %+v", api)
	}
}

// writeAccess answers hidden objects as unknown ones, refuses operations the
// describe flags exclude, and applies object and field permissions.
func TestWriteAccess(t *testing.T) {
	c := setupSnapshot()
	std := Principal{Catalog: c, Perms: policytest.StandardUser(c.Objects)}
	admin := Principal{Catalog: c, Perms: policytest.Admin()}
	for _, tc := range []struct {
		name      string
		p         Principal
		object    string
		op        string
		data      map[string]any
		status    int
		code      string
		fieldName string
	}{
		{"unknown object", admin, "Nope__c", "create", nil, 404, "NOT_FOUND", ""},
		{"hidden setup object", std, "PermissionSet", "create", nil, 404, "NOT_FOUND", ""},
		{"user delete", admin, "User", "delete", nil, 403, "INSUFFICIENT_ACCESS_OR_READONLY", ""},
		{"profile update", admin, "Profile", "update", nil, 403, "INSUFFICIENT_ACCESS_OR_READONLY", ""},
		{"assignment update", admin, "PermissionSetAssignment", "update", nil, 403, "INSUFFICIENT_ACCESS_OR_READONLY", ""},
		{"hidden field", std, "Account", "update", map[string]any{"compat_text__c": "x"}, 400, "INVALID_FIELD", "compat_text__c"},
		{"hidden field upsert", std, "Account", "upsert", map[string]any{"Compat_Text__c": "x"}, 400, "INVALID_FIELD", "Compat_Text__c"},
	} {
		_, err := writeAccess(tc.p, tc.object, tc.op, tc.data)
		api := apiError(t, err)
		if api.Status != tc.status || api.Code != tc.code || tc.fieldName != "" && (len(api.Fields) != 1 || api.Fields[0] != tc.fieldName) {
			t.Fatalf("%s: got %+v", tc.name, api)
		}
	}
	readOnly := policy.Union(policy.PermissionSet{
		Objects: map[string]policy.ObjectAccess{"Account": {Read: true, Edit: true}},
		Fields:  map[string]policy.FieldAccess{"Account.Phone": {Read: true}},
	})
	_, err := writeAccess(Principal{Catalog: c, Perms: readOnly}, "Account", "update", map[string]any{"Phone": "1"})
	if api := apiError(t, err); api.Code != "INVALID_FIELD_FOR_INSERT_UPDATE" || !strings.HasPrefix(api.Message, "Unable to create/update fields: Phone.") {
		t.Fatalf("readable, not editable: %+v", api)
	}
	_, err = writeAccess(Principal{Catalog: c, Perms: readOnly}, "Account", "create", map[string]any{"Name": "x"})
	if api := apiError(t, err); api.Status != 403 {
		t.Fatalf("create without Create: %+v", api)
	}
	if _, err = writeAccess(std, "Account", "update", map[string]any{"Phone": "1", "Name": "x"}); err != nil {
		t.Fatalf("standard user update: %v", err)
	}
	if _, err = writeAccess(admin, "User", "update", map[string]any{"LastName": "x"}); err != nil {
		t.Fatalf("admin user update: %v", err)
	}
}

func TestCheckUsername(t *testing.T) {
	if _, err := checkUsername("Mixed.Case@Example.com"); err != nil {
		t.Fatal(err)
	}
	for _, v := range []any{"no-at-sign", "a@@b.com", "é@example.com", strings.Repeat("a", 75) + "@b.com", 7} {
		if _, err := checkUsername(v); err == nil {
			t.Fatalf("accepted %v", v)
		}
	}
	api := apiError(t, func() error { _, e := checkUsername("bad"); return e }())
	if api.Code != "INVALID_EMAIL_ADDRESS" || api.Message != "Username: invalid email address: bad" {
		t.Fatalf("got %+v", api)
	}
}

func TestCheckFieldGrant(t *testing.T) {
	text := metadata.Field{Name: "Phone", Nillable: true, Createable: true, Updateable: true}
	formula := metadata.Field{Name: "Total__c", Nillable: true}
	for _, tc := range []struct {
		a       policy.FieldAccess
		f       metadata.Field
		create  bool
		message string
	}{
		{policy.FieldAccess{Edit: true}, text, true, "Permission Edit depends on permission(s): Read"},
		{policy.FieldAccess{}, text, true, "A field permission must grant at least Read"},
		{policy.FieldAccess{Read: true, Edit: true}, formula, true, "Field is read-only"},
		{policy.FieldAccess{Read: true}, text, true, ""},
		{policy.FieldAccess{Read: true, Edit: true}, text, false, ""},
	} {
		err := checkFieldGrant(tc.a, tc.f, tc.create)
		if tc.message == "" {
			if err != nil {
				t.Fatalf("%+v: %v", tc.a, err)
			}
			continue
		}
		if api := apiError(t, err); api.Code != "FIELD_INTEGRITY_EXCEPTION" || api.Message != tc.message {
			t.Fatalf("%+v: got %+v", tc.a, api)
		}
	}
}

func TestObjectGrantMerge(t *testing.T) {
	current := policy.ObjectAccess{Read: true, Edit: true}
	got := objectGrant(map[string]any{"PermissionsDelete": true, "PermissionsEdit": nil}, current)
	if want := (policy.ObjectAccess{Read: true, Delete: true}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if err := policy.ValidateObjectGrant(got); err == nil {
		t.Fatal("Delete without Edit accepted")
	}
}

// A default on a field the caller can't set is left to the column default,
// so Validate doesn't reject a value the caller never sent.
func TestApplyDefaultsSkipsUncreateable(t *testing.T) {
	o, err := setupSnapshot().Object("PermissionSet")
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"Name": "X", "Label": "X"}
	applyDefaults(o, data)
	if _, ok := data["IsOwnedByProfile"]; ok {
		t.Fatalf("IsOwnedByProfile defaulted: %v", data)
	}
	if v, ok := data["PermissionsManageUsers"]; !ok || v != false {
		t.Fatalf("PermissionsManageUsers = %v, %v; want false", v, ok)
	}
}
