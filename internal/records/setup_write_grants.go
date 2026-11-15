package records

import (
	"strings"

	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

const integrityCode = "FIELD_INTEGRITY_EXCEPTION"

// permissionSet writes a PermissionSet. Profile-owned sets are updatable
// (the only way to change a profile's system permissions) but never created
// or deleted (D33-21).
func (w setupWriter) permissionSet(op, id string, data map[string]any) (string, error) {
	switch op {
	case "create":
		if flag(data, "IsOwnedByProfile", false) || data["ProfileId"] != nil {
			return "", errInsufficientAccess // D33-21
		}
		if err := Validate(w.p, "PermissionSet", data, true); err != nil {
			return "", err
		}
		if err := w.uniqueSetName(data, ""); err != nil {
			return "", err
		}
		return insertRecord(w.ctx, w.db, w.p, "PermissionSet", data, "", nil)
	case "update":
		if err := Validate(w.p, "PermissionSet", data, false); err != nil {
			return "", err
		}
		if err := w.uniqueSetName(data, id); err != nil {
			return "", err
		}
		return id, updateRecord(w.ctx, w.db, w.p, "PermissionSet", id, data, nil)
	}
	var owned bool
	if err := w.db.QueryRow(w.ctx, "SELECT is_owned_by_profile FROM sc_setup.permission_sets WHERE tenant_id=$1 AND id=$2", w.p.Tenant, id).Scan(&owned); err != nil {
		return "", dbError(err)
	}
	if owned {
		return "", errInsufficientAccess // D33-21, D33-31
	}
	return id, removeRecord(w.ctx, w.db, w.p, "PermissionSet", id)
}

func (w setupWriter) uniqueSetName(data map[string]any, id string) error {
	name, ok := data["Name"].(string)
	if !ok {
		return nil
	}
	return w.duplicate("SELECT id FROM sc_setup.permission_sets WHERE tenant_id=$1 AND lower(name)=lower($2) AND id<>$3", name, id)
}

// grantObject resolves a grant's SobjectType. Unknown names and the setup
// objects, which take no grants, are a bad restricted-picklist value
// (D33-12, D33-32).
func (w setupWriter) grantObject(v any, field string) (metadata.Object, error) {
	s, _ := v.(string)
	o, err := w.p.Catalog.Object(s)
	if err != nil || metadata.IsSetupObject(o.Name) {
		return metadata.Object{}, badPicklist(s, field)
	}
	return o, nil
}

func badPicklist(value, field string) error {
	return fieldProblem("INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST", "bad value for restricted picklist field: "+value, field)
}

// objectPermissions writes an ObjectPermissions row, enforcing the
// dependencies on the row as it will be stored (D33-10).
func (w setupWriter) objectPermissions(op, id string, data map[string]any) (string, error) {
	var current policy.ObjectAccess
	switch op {
	case "create":
		if err := Validate(w.p, "ObjectPermissions", data, true); err != nil {
			return "", err
		}
		parent, err := w.reference(data, "ParentId", "PermissionSet")
		if err != nil {
			return "", err
		}
		o, err := w.grantObject(data["SobjectType"], "SobjectType")
		if err != nil {
			return "", err
		}
		data["SobjectType"] = o.Name
		if err = policy.ValidateObjectGrant(objectGrant(data, current)); err != nil {
			return "", err
		}
		if err = w.duplicate("SELECT id FROM sc_setup.object_permissions WHERE tenant_id=$1 AND parent_id=$2 AND sobject_type=$3", parent, o.Name); err != nil {
			return "", err
		}
		return insertRecord(w.ctx, w.db, w.p, "ObjectPermissions", data, "", nil)
	case "update":
		if err := Validate(w.p, "ObjectPermissions", data, false); err != nil {
			return "", err
		}
		err := w.db.QueryRow(w.ctx, `SELECT permissions_read,permissions_create,permissions_edit,permissions_delete,permissions_view_all_records,permissions_modify_all_records
			FROM sc_setup.object_permissions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, w.p.Tenant, id).
			Scan(&current.Read, &current.Create, &current.Edit, &current.Delete, &current.ViewAll, &current.ModifyAll)
		if err != nil {
			return "", dbError(err)
		}
		if err = policy.ValidateObjectGrant(objectGrant(data, current)); err != nil {
			return "", err
		}
		return id, updateRecord(w.ctx, w.db, w.p, "ObjectPermissions", id, data, nil)
	}
	return id, removeRecord(w.ctx, w.db, w.p, "ObjectPermissions", id)
}

func objectGrant(data map[string]any, c policy.ObjectAccess) policy.ObjectAccess {
	return policy.ObjectAccess{
		Read: flag(data, "PermissionsRead", c.Read), Create: flag(data, "PermissionsCreate", c.Create),
		Edit: flag(data, "PermissionsEdit", c.Edit), Delete: flag(data, "PermissionsDelete", c.Delete),
		ViewAll: flag(data, "PermissionsViewAllRecords", c.ViewAll), ModifyAll: flag(data, "PermissionsModifyAllRecords", c.ModifyAll),
	}
}

// fieldPermissions writes a FieldPermissions row. Edit needs Read; a row
// granting nothing is rejected on create and deleted on update (D33-11).
func (w setupWriter) fieldPermissions(op, id string, data map[string]any) (string, error) {
	switch op {
	case "create":
		if err := Validate(w.p, "FieldPermissions", data, true); err != nil {
			return "", err
		}
		parent, err := w.reference(data, "ParentId", "PermissionSet")
		if err != nil {
			return "", err
		}
		f, name, err := w.grantField(data)
		if err != nil {
			return "", err
		}
		a := policy.FieldAccess{Read: flag(data, "PermissionsRead", false), Edit: flag(data, "PermissionsEdit", false)}
		if err = checkFieldGrant(a, f, true); err != nil {
			return "", err
		}
		if err = w.duplicate("SELECT id FROM sc_setup.field_permissions WHERE tenant_id=$1 AND parent_id=$2 AND field=$3", parent, name); err != nil {
			return "", err
		}
		return insertRecord(w.ctx, w.db, w.p, "FieldPermissions", data, "", nil)
	case "update":
		if err := Validate(w.p, "FieldPermissions", data, false); err != nil {
			return "", err
		}
		var object, name string
		var a policy.FieldAccess
		err := w.db.QueryRow(w.ctx, "SELECT sobject_type,field,permissions_read,permissions_edit FROM sc_setup.field_permissions WHERE tenant_id=$1 AND id=$2 FOR UPDATE", w.p.Tenant, id).
			Scan(&object, &name, &a.Read, &a.Edit)
		if err != nil {
			return "", dbError(err)
		}
		a = policy.FieldAccess{Read: flag(data, "PermissionsRead", a.Read), Edit: flag(data, "PermissionsEdit", a.Edit)}
		if err = policy.ValidateFieldGrant(a); err != nil {
			return "", err
		}
		if !a.Read {
			return id, removeRecord(w.ctx, w.db, w.p, "FieldPermissions", id) // D33-11: granting nothing deletes the row
		}
		f, _ := w.p.Catalog.Field(object, strings.TrimPrefix(name, object+"."))
		if err = checkFieldGrant(a, f, false); err != nil {
			return "", err
		}
		return id, updateRecord(w.ctx, w.db, w.p, "FieldPermissions", id, data, nil)
	}
	return id, removeRecord(w.ctx, w.db, w.p, "FieldPermissions", id)
}

// grantField resolves SobjectType and Field ("Object.Field") to a catalog
// field that takes field permissions, and canonicalizes both in data.
func (w setupWriter) grantField(data map[string]any) (metadata.Field, string, error) {
	o, err := w.grantObject(data["SobjectType"], "SobjectType")
	if err != nil {
		return metadata.Field{}, "", err
	}
	raw, _ := data["Field"].(string)
	object, name, _ := strings.Cut(raw, ".")
	if !strings.EqualFold(object, o.Name) {
		return metadata.Field{}, "", badPicklist(raw, "Field") // D33-12, D33-32
	}
	f, err := w.p.Catalog.Field(o.Name, name)
	if err != nil {
		return metadata.Field{}, "", badPicklist(raw, "Field") // D33-12
	}
	qualified := o.Name + "." + f.Name
	if !policy.Permissionable(f) {
		return metadata.Field{}, "", fieldProblem(integrityCode, "Field "+qualified+" does not support field permissions", "Field") // D33-13
	}
	data["SobjectType"], data["Field"] = o.Name, qualified
	return f, qualified, nil
}

// checkFieldGrant applies D33-11 and D33-13 to the access a row will store.
func checkFieldGrant(a policy.FieldAccess, f metadata.Field, create bool) error {
	if err := policy.ValidateFieldGrant(a); err != nil {
		return err
	}
	if create && !a.Read {
		return fieldProblem(integrityCode, "A field permission must grant at least Read", "PermissionsRead") // D33-11
	}
	if a.Edit && f.Name != "" && !f.Createable && !f.Updateable {
		return fieldProblem(integrityCode, "Field is read-only", "PermissionsEdit") // D33-13
	}
	return nil
}

// assignment writes a PermissionSetAssignment. Assignments to a
// profile-owned set are system-maintained: never created or deleted
// directly (D33-21). They can't be updated (D33-31, from the describe flag).
func (w setupWriter) assignment(op, id string, data map[string]any) (string, error) {
	var owned bool
	if op == "create" {
		if err := Validate(w.p, "PermissionSetAssignment", data, true); err != nil {
			return "", err
		}
		assignee, err := w.reference(data, "AssigneeId", "User")
		if err != nil {
			return "", err
		}
		set, err := w.reference(data, "PermissionSetId", "PermissionSet")
		if err != nil {
			return "", err
		}
		if err = w.db.QueryRow(w.ctx, "SELECT is_owned_by_profile FROM sc_setup.permission_sets WHERE tenant_id=$1 AND id=$2", w.p.Tenant, set).Scan(&owned); err != nil {
			return "", dbError(err)
		}
		if owned {
			return "", errInsufficientAccess // D33-21
		}
		if err = w.duplicate("SELECT id FROM sc_setup.permission_set_assignments WHERE tenant_id=$1 AND assignee_id=$2 AND permission_set_id=$3", assignee, set); err != nil {
			return "", err
		}
		return insertRecord(w.ctx, w.db, w.p, "PermissionSetAssignment", data, "", nil)
	}
	err := w.db.QueryRow(w.ctx, `SELECT s.is_owned_by_profile FROM sc_setup.permission_set_assignments a
		JOIN sc_setup.permission_sets s ON s.tenant_id=a.tenant_id AND s.id=a.permission_set_id
		WHERE a.tenant_id=$1 AND a.id=$2`, w.p.Tenant, id).Scan(&owned)
	if err != nil {
		return "", dbError(err)
	}
	if owned || op != "delete" {
		return "", errInsufficientAccess // D33-21
	}
	return id, removeRecord(w.ctx, w.db, w.p, "PermissionSetAssignment", id)
}
