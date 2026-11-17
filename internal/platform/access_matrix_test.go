package platform

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

// matrixSubject is one row family of the access matrix: a user and the
// grants the spec's "Seeded profiles and migration" gives them.
type matrixSubject struct {
	name    string
	user    string
	admin   bool                   // Modify All Data and View All Data
	api     bool                   // API Enabled: User and Organization readable
	objects map[string][4]bool     // read, create, edit, delete
	read    func(o, f string) bool // field read granted on a permissionable field
	edit    func(o, f string) bool // field edit granted on a permissionable field
}

func none(string, string) bool { return false }

func businessObject(o string) bool { return o == "Account" || o == "Contact" }

// Every catalog object × field × subject has exactly the access the spec
// grants: no cell is allowed that the table doesn't name.
func TestAccessMatrixNoUnexpectedGrants(t *testing.T) {
	f := newWriteFixture(t)
	ctx := context.Background()
	o, e := metadata.CreateObject(ctx, f.s.pool, f.tenant, metadata.ObjectInput{Name: "Matrix__c", Label: "Matrix", PluralLabel: "Matrix"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.s.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+o.TableSQL()+" CASCADE") })
	if _, e = metadata.CreateField(ctx, f.s.pool, f.tenant, metadata.FieldInput{Object: "Matrix__c", Name: "Note__c", Label: "Note", Type: "Text", Length: 20}); e != nil {
		t.Fatal(e)
	}
	user := func(name, profile string) string {
		id, e := f.s.CreateUserWithProfile(ctx, f.tenant, name+"."+f.tenant+"@example.com", "long-password", profile)
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	std, ro, min, custom := user("std", "Standard User"), user("ro", "Read Only"), user("min", "Minimum Access"), user("custom", "Standard User")
	set := f.created("PermissionSet", `{"Name":"Matrix_Set","Label":"Matrix set"}`)
	f.created("ObjectPermissions", `{"ParentId":"`+set+`","SobjectType":"Matrix__c","PermissionsRead":true,"PermissionsCreate":true,"PermissionsEdit":true}`)
	f.created("FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","Field":"Account.Compat_Text__c","PermissionsRead":true}`)
	f.created("FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Matrix__c","Field":"Matrix__c.Note__c","PermissionsRead":true,"PermissionsEdit":true}`)
	f.created("PermissionSetAssignment", `{"AssigneeId":"`+custom+`","PermissionSetId":"`+set+`"}`)

	crud := map[string][4]bool{"Account": {true, true, true, true}, "Contact": {true, true, true, true}}
	stdRead := func(o, fl string) bool { return businessObject(o) && o+"."+fl != "Account.Compat_Text__c" }
	subjects := []matrixSubject{
		{name: "System Administrator", user: f.user, admin: true, api: true, read: none, edit: none},
		{name: "Standard User", user: std, api: true, objects: crud, read: stdRead, edit: stdRead},
		{name: "Read Only", user: ro, api: true, objects: map[string][4]bool{"Account": {true}, "Contact": {true}},
			read: func(o, _ string) bool { return businessObject(o) }, edit: none},
		{name: "Minimum Access", user: min, read: none, edit: none},
		{name: "Standard User + custom set", user: custom, api: true,
			objects: map[string][4]bool{"Account": crud["Account"], "Contact": crud["Contact"], "Matrix__c": {true, true, true, false}},
			read: func(o, fl string) bool {
				return stdRead(o, fl) || o+"."+fl == "Account.Compat_Text__c" || o+"."+fl == "Matrix__c.Note__c"
			},
			edit: func(o, fl string) bool { return stdRead(o, fl) || o+"."+fl == "Matrix__c.Note__c" }},
	}
	cells := 0
	var wrong []string
	for _, sub := range subjects {
		e := f.s.within(ctx, Principal{Tenant: f.tenant, User: sub.user}, func(_ *Store, p Principal) error {
			for _, obj := range p.Catalog.Objects {
				want := sub.objects[obj.Name]
				if sub.admin {
					want = [4]bool{true, true, true, true}
				} else if sub.api && (obj.Name == "User" || obj.Name == "Organization" || metadata.IsSharingSetupObject(obj.Name)) {
					want = [4]bool{true}
				} else if sub.api && metadata.IsShareObject(obj.Name) {
					// Plan 34: share rows are filtered by the share rules.
					want = [4]bool{true, true, true, true}
				}
				a := p.Perms.Object(obj.Name)
				if got := [4]bool{a.Read, a.Create, a.Edit, a.Delete}; got != want {
					wrong = append(wrong, fmt.Sprintf("%s %s: {read,create,edit,delete} %v, want %v", sub.name, obj.Name, got, want))
				}
				cells++
				for _, fl := range obj.Fields {
					perm, setup := policy.Permissionable(fl), metadata.IsSetupObject(obj.Name)
					read := want[0] && (!perm || sub.admin || setup || sub.read(obj.Name, fl.Name))
					edit := (want[1] || want[2]) && (fl.Createable || fl.Updateable) && (!perm || sub.admin || sub.edit(obj.Name, fl.Name))
					gr, ge := p.Perms.FieldRead(obj.Name, fl), p.Perms.FieldEdit(obj.Name, fl)
					if gr != read || ge != edit {
						wrong = append(wrong, fmt.Sprintf("%s %s.%s: {fieldRead,fieldEdit} {%v %v}, want {%v %v}", sub.name, obj.Name, fl.Name, gr, ge, read, edit))
					}
					cells++
				}
			}
			return nil
		})
		if e != nil {
			t.Fatalf("%s: %v", sub.name, e)
		}
	}
	sort.Strings(wrong)
	for _, w := range wrong {
		t.Error(w)
	}
	if cells < 200 {
		t.Fatalf("only %d cells checked", cells)
	}
}
