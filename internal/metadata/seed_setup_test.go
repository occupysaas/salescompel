package metadata

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func TestSetupObjectsMatchSpec(t *testing.T) {
	want := map[string]struct {
		prefix                    string
		create, update, deletable bool
	}{
		"Organization": {"00D", false, false, false}, "User": {"005", true, true, false},
		"Profile": {"00e", false, false, false}, "PermissionSet": {"0PS", true, true, true},
		"ObjectPermissions": {"110", true, true, true}, "FieldPermissions": {"01k", true, true, true},
		"PermissionSetAssignment": {"0Pa", true, false, true},
		// Plan 34 sharing objects.
		"UserRole": {"00E", true, true, true}, "Group": {"00G", true, true, true},
		"GroupMember": {"011", true, false, true}, "AccountShare": {"00r", true, true, true},
		"ContactShare": {"03s", true, true, true},
	}
	objects := SetupObjects()
	if len(objects) != len(want) {
		t.Fatalf("%d setup objects", len(objects))
	}
	snap := Snapshot{Objects: StandardObjects()}
	for _, o := range objects {
		w, ok := want[o.Name]
		if !ok || o.Prefix != w.prefix || o.Createable != w.create || o.Updateable != w.update || o.Deletable != w.deletable || o.Schema != SetupSchema || !o.Standard || !IsSetupObject(o.Name) {
			t.Fatalf("%s: %+v", o.Name, o)
		}
		for _, f := range o.Fields {
			if _, e := EncodeDefinition(f); e != nil {
				t.Fatalf("%s.%s: %v", o.Name, f.Name, e)
			}
		}
	}
	children := map[string][]string{"User": {"PermissionSetAssignments"}, "PermissionSet": {"Assignments", "ObjectPerms", "FieldPerms"}}
	for parent, names := range children {
		for _, n := range names {
			if _, _, e := snap.ChildRelationship(parent, n); e != nil {
				t.Fatalf("%s.%s: %v", parent, n, e)
			}
		}
	}
	if IsSetupObject("Account") {
		t.Fatal("Account is not a setup object")
	}
}

func TestTimeZoneSidKeysLoad(t *testing.T) {
	if len(TimeZoneSidKeys) != 94 {
		t.Fatalf("%d time zones", len(TimeZoneSidKeys))
	}
	for _, z := range TimeZoneSidKeys {
		if _, e := time.LoadLocation(z); e != nil {
			t.Fatalf("%s: %v", z, e)
		}
	}
}
