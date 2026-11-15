package policy

import (
	"testing"

	"salescompel/internal/metadata"
)

var phone = metadata.Field{Name: "Phone", Nillable: true, Createable: true, Updateable: true}

func TestUnionGrantsAreAdditive(t *testing.T) {
	p := Union(
		PermissionSet{Objects: map[string]ObjectAccess{"Account": {Read: true}}},
		PermissionSet{Objects: map[string]ObjectAccess{"Account": {Read: true, Edit: true}}},
	)
	if got := p.Object("Account"); got != (ObjectAccess{Read: true, Edit: true}) {
		t.Fatalf("got %+v", got)
	}
}

func TestModifyAllDataGrantsEverything(t *testing.T) {
	p := Union(PermissionSet{System: SystemPerms{ModifyAllData: true}})
	if got := p.Object("Anything__c"); got != (ObjectAccess{true, true, true, true, true, true}) {
		t.Fatalf("got %+v", got)
	}
	if !p.FieldEdit("Anything__c", phone) || !p.FieldRead("Anything__c", phone) {
		t.Fatal("field access denied")
	}
}

func TestViewAllDataGrantsReadOnly(t *testing.T) {
	p := Union(PermissionSet{System: SystemPerms{ViewAllData: true}})
	if got := p.Object("Account"); got != (ObjectAccess{Read: true, ViewAll: true}) {
		t.Fatalf("got %+v", got)
	}
	if !p.FieldRead("Account", phone) || p.FieldEdit("Account", phone) {
		t.Fatal("expected read-only field access")
	}
}

func TestEmptyIsDenied(t *testing.T) {
	var p Permissions
	if got := p.Object("Account"); got != (ObjectAccess{}) {
		t.Fatalf("got %+v", got)
	}
	if p.FieldRead("Account", phone) || p.FieldEdit("Account", phone) {
		t.Fatal("zero permissions allowed a field")
	}
}

func TestFieldGrantsAreCaseInsensitiveAndObjectGated(t *testing.T) {
	set := PermissionSet{
		Objects: map[string]ObjectAccess{"Account": {Read: true, Edit: true}},
		Fields:  map[string]FieldAccess{"account.PHONE": {Read: true, Edit: true}},
	}
	p := Union(set)
	if !p.FieldRead("Account", phone) || !p.FieldEdit("Account", phone) {
		t.Fatal("field grant not honored")
	}
	other := metadata.Field{Name: "Fax", Nillable: true, Createable: true, Updateable: true}
	if p.FieldRead("Account", other) {
		t.Fatal("ungranted field readable")
	}
	noObj := Union(PermissionSet{Fields: set.Fields})
	if noObj.FieldRead("Account", phone) {
		t.Fatal("field readable without object read")
	}
}

func TestNonPermissionableFollowsObject(t *testing.T) {
	id := metadata.Field{Name: "Id", Createable: false}
	name := metadata.Field{Name: "Name", NameField: true, Createable: true, Updateable: true}
	readable := Union(PermissionSet{Objects: map[string]ObjectAccess{"Account": {Read: true}}})
	for _, f := range []metadata.Field{id, name} {
		if !readable.FieldRead("Account", f) {
			t.Fatalf("%s not readable", f.Name)
		}
		if Union().FieldRead("Account", f) {
			t.Fatalf("%s readable without object", f.Name)
		}
	}
	if readable.FieldEdit("Account", name) {
		t.Fatal("name editable without Edit/Create")
	}
	editable := Union(PermissionSet{Objects: map[string]ObjectAccess{"Account": {Read: true, Edit: true}}})
	if !editable.FieldEdit("Account", name) || editable.FieldEdit("Account", id) {
		t.Fatal("wrong non-permissionable edit result")
	}
}

func TestPermissionable(t *testing.T) {
	cases := map[string]metadata.Field{
		"id":        {Name: "Id", Nillable: true},
		"required":  {Name: "Phone"},
		"name":      {Name: "Name", Nillable: true, NameField: true},
		"generated": {Name: "X", Nillable: true, Options: metadata.Options{Generated: true}},
		"system":    {Name: "createdbyid", Nillable: true},
	}
	for n, f := range cases {
		if Permissionable(f) {
			t.Errorf("%s should not be permissionable", n)
		}
	}
	if !Permissionable(phone) {
		t.Error("phone should be permissionable")
	}
}

func TestObjectGrantDependencies(t *testing.T) {
	bad := map[string]ObjectAccess{
		"create without read":  {Create: true},
		"edit without read":    {Edit: true},
		"delete without edit":  {Read: true, Delete: true},
		"viewall without read": {ViewAll: true},
		"modifyall alone":      {ModifyAll: true},
	}
	for n, a := range bad {
		if err := ValidateObjectGrant(a); err == nil {
			t.Errorf("%s: want error", n)
		}
	}
	if err := ValidateObjectGrant(ObjectAccess{true, true, true, true, true, true}); err != nil {
		t.Fatal(err)
	}
	err := ValidateObjectGrant(ObjectAccess{Create: true})
	if err.Error() != "FIELD_INTEGRITY_EXCEPTION: Permission Create depends on permission(s): Read" {
		t.Fatal(err)
	}
}

func TestFieldGrantDependencies(t *testing.T) {
	if err := ValidateFieldGrant(FieldAccess{Edit: true}); err == nil {
		t.Fatal("want error")
	}
	if err := ValidateFieldGrant(FieldAccess{Read: true, Edit: true}); err != nil {
		t.Fatal(err)
	}
}
