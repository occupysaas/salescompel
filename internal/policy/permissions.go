package policy

import (
	"strings"

	"salescompel/internal/metadata"
)

// ObjectAccess is the CRUD and record-visibility grant on one object.
type ObjectAccess struct{ Read, Create, Edit, Delete, ViewAll, ModifyAll bool }

// FieldAccess is the grant on one permissionable field.
type FieldAccess struct{ Read, Edit bool }

// SystemPerms are the org-wide permissions a permission set can carry.
type SystemPerms struct{ ModifyAllData, ViewAllData, APIEnabled, ManageUsers bool }

// PermissionSet is one profile or permission set. Map keys are canonical API
// names: "Account" and "Account.Phone".
type PermissionSet struct {
	ID      string
	System  SystemPerms
	Objects map[string]ObjectAccess
	Fields  map[string]FieldAccess
}

// Permissions is the additive union of a user's permission sets. The zero
// value denies everything.
type Permissions struct {
	system  SystemPerms
	objects map[string]ObjectAccess
	fields  map[string]FieldAccess // keys lower-cased
}

// Union merges sets additively: any set granting a permission grants it.
func Union(sets ...PermissionSet) Permissions {
	p := Permissions{objects: map[string]ObjectAccess{}, fields: map[string]FieldAccess{}}
	for _, s := range sets {
		p.system.ModifyAllData = p.system.ModifyAllData || s.System.ModifyAllData
		p.system.ViewAllData = p.system.ViewAllData || s.System.ViewAllData
		p.system.APIEnabled = p.system.APIEnabled || s.System.APIEnabled
		p.system.ManageUsers = p.system.ManageUsers || s.System.ManageUsers
		for k, a := range s.Objects {
			o := p.objects[k]
			o.Read, o.Create, o.Edit = o.Read || a.Read, o.Create || a.Create, o.Edit || a.Edit
			o.Delete, o.ViewAll, o.ModifyAll = o.Delete || a.Delete, o.ViewAll || a.ViewAll, o.ModifyAll || a.ModifyAll
			p.objects[k] = o
		}
		for k, a := range s.Fields {
			k = strings.ToLower(k)
			f := p.fields[k]
			f.Read, f.Edit = f.Read || a.Read, f.Edit || a.Edit
			p.fields[k] = f
		}
	}
	return p
}

func (p Permissions) System() SystemPerms { return p.system }

// Object returns the effective access, applying Modify All Data and View All Data.
func (p Permissions) Object(object string) ObjectAccess {
	if p.system.ModifyAllData {
		return ObjectAccess{true, true, true, true, true, true}
	}
	if metadata.IsSetupObject(object) {
		return setupObjectAccess(p.system, object)
	}
	a := p.objects[object]
	if p.system.ViewAllData {
		a.Read, a.ViewAll = true, true
	}
	return a
}

// setupObjectAccess is the read rule for setup objects, which never take
// ObjectPermissions grants: every API-enabled user reads User and
// Organization; the others need Manage Users or View All Data. Empty
// permissions (an inactive or unknown user) read nothing. Writes follow the
// setup write rules in internal/records.
func setupObjectAccess(s SystemPerms, object string) ObjectAccess {
	everyone := strings.EqualFold(object, "User") || strings.EqualFold(object, "Organization")
	read := (everyone && s.APIEnabled) || s.ManageUsers || s.ViewAllData
	return ObjectAccess{Read: read, ViewAll: s.ViewAllData}
}

// FieldRead reports whether the field can be read on the object.
func (p Permissions) FieldRead(object string, f metadata.Field) bool {
	o := p.Object(object)
	if !o.Read {
		return false
	}
	// Setup-object fields have no field-level security.
	if !Permissionable(f) || p.system.ModifyAllData || p.system.ViewAllData || metadata.IsSetupObject(object) {
		return true
	}
	return p.fields[fieldKey(object, f)].Read
}

// FieldEdit reports whether the field can be written on the object.
func (p Permissions) FieldEdit(object string, f metadata.Field) bool {
	o := p.Object(object)
	if !(o.Create || o.Edit) || !(f.Createable || f.Updateable) {
		return false
	}
	if !Permissionable(f) || p.system.ModifyAllData {
		return true
	}
	return p.fields[fieldKey(object, f)].Edit
}

func fieldKey(object string, f metadata.Field) string {
	return strings.ToLower(object + "." + f.Name)
}

// Permissionable reports whether field-level security can be set on f. It is
// false for Id, required (non-nillable), name, generated and system fields.
func Permissionable(f metadata.Field) bool { return metadata.Permissionable(f) }
