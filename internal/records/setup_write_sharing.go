package records

import (
	"salescompel/internal/recordid"
)

// Plan 34 setup writes: UserRole, Group and GroupMember (D34-12, D34-13,
// D34-15). Each change rebuilds the tenant's sharing closure in the same
// transaction (shr-05).

// roleGroupTypes are the system groups every role owns (D34-12).
var roleGroupTypes = []string{"Role", "RoleAndSubordinates", "RoleAndSubordinatesInternal"}

func (w setupWriter) rebuild() error { return dbError(RebuildSharing(w.ctx, w.db, w.p.Tenant)) }

func (w setupWriter) userRole(op, id string, data map[string]any) (string, error) {
	switch op {
	case "create":
		if err := Validate(w.p, "UserRole", data, true); err != nil {
			return "", err
		}
		if _, err := w.reference(data, "ParentRoleId", "UserRole"); err != nil {
			return "", err
		}
		if err := w.uniqueRoleName(data, ""); err != nil {
			return "", err
		}
		id, err := insertRecord(w.ctx, w.db, w.p, "UserRole", data, "", nil)
		if err != nil {
			return "", err
		}
		name, _ := data["Name"].(string)
		if len(name) > 40 {
			name = name[:40]
		}
		for _, kind := range roleGroupTypes {
			gid, err := recordid.New("00G")
			if err != nil {
				return "", dbError(err)
			}
			if _, err = w.db.Exec(w.ctx, "INSERT INTO sc_setup.groups(id,tenant_id,name,type,related_id) VALUES($1,$2,$3,$4,$5)", gid, w.p.Tenant, name, kind, id); err != nil {
				return "", dbError(err)
			}
		}
		return id, w.rebuild()
	case "update":
		if err := Validate(w.p, "UserRole", data, false); err != nil {
			return "", err
		}
		parent, err := w.reference(data, "ParentRoleId", "UserRole")
		if err != nil {
			return "", err
		}
		if parent != "" {
			if err := w.roleCycle(id, parent); err != nil {
				return "", err
			}
		}
		if err := w.uniqueRoleName(data, id); err != nil {
			return "", err
		}
		if err := updateRecord(w.ctx, w.db, w.p, "UserRole", id, data, nil); err != nil {
			return "", err
		}
		return id, w.rebuild()
	}
	var busy bool
	if err := w.db.QueryRow(w.ctx, `SELECT EXISTS(SELECT FROM sc_setup.users WHERE tenant_id=$1 AND user_role_id=$2)
		OR EXISTS(SELECT FROM sc_setup.user_roles WHERE tenant_id=$1 AND parent_role_id=$2)`, w.p.Tenant, id).Scan(&busy); err != nil {
		return "", dbError(err)
	}
	if busy {
		// D34-15 (assumed): a role with users or child roles can't be deleted.
		return "", invalid("DELETE_FAILED", "Cannot delete a role that has users or subordinate roles.")
	}
	if _, err := w.db.Exec(w.ctx, `DELETE FROM sc_sharing.shares WHERE tenant_id=$1 AND user_or_group_id IN
		(SELECT id FROM sc_setup.groups WHERE tenant_id=$1 AND related_id=$2)`, w.p.Tenant, id); err != nil {
		return "", dbError(err)
	}
	if err := removeRecord(w.ctx, w.db, w.p, "UserRole", id); err != nil {
		return "", err
	}
	return id, w.rebuild()
}

// roleCycle refuses a parent that is the role itself or one of its
// subordinates.
func (w setupWriter) roleCycle(id, parent string) error {
	var cycle bool
	err := w.db.QueryRow(w.ctx, `WITH RECURSIVE up(id) AS (SELECT $2::varchar UNION SELECT r.parent_role_id FROM up JOIN sc_setup.user_roles r ON r.tenant_id=$1 AND r.id=up.id WHERE r.parent_role_id IS NOT NULL)
		SELECT EXISTS(SELECT FROM up WHERE id=$3)`, w.p.Tenant, parent, id).Scan(&cycle)
	if err != nil {
		return dbError(err)
	}
	if cycle {
		return fieldProblem(integrityCode, "A role can't be its own parent or report to one of its subordinates.", "ParentRoleId")
	}
	return nil
}

func (w setupWriter) uniqueRoleName(data map[string]any, id string) error {
	name, ok := data["DeveloperName"].(string)
	if !ok {
		return nil
	}
	return w.duplicate("SELECT id FROM sc_setup.user_roles WHERE tenant_id=$1 AND lower(developer_name)=lower($2) AND id<>$3", name, id)
}

// group writes a Regular group. Role and Organization groups are system
// maintained (D34-12).
func (w setupWriter) group(op, id string, data map[string]any) (string, error) {
	if op == "create" {
		if v, ok := data["Type"]; ok && v != nil && v != "Regular" {
			return "", fieldProblem("INVALID_OPERATION", "Only Regular groups can be created.", "Type")
		}
		data["Type"] = "Regular"
		if err := Validate(w.p, "Group", data, true); err != nil {
			return "", err
		}
		return insertRecord(w.ctx, w.db, w.p, "Group", data, "", nil)
	}
	if err := w.regularGroup(id); err != nil {
		return "", err
	}
	if op == "update" {
		if err := Validate(w.p, "Group", data, false); err != nil {
			return "", err
		}
		return id, updateRecord(w.ctx, w.db, w.p, "Group", id, data, nil)
	}
	if _, err := w.db.Exec(w.ctx, "DELETE FROM sc_sharing.shares WHERE tenant_id=$1 AND user_or_group_id=$2", w.p.Tenant, id); err != nil {
		return "", dbError(err)
	}
	if _, err := w.db.Exec(w.ctx, "DELETE FROM sc_setup.group_members WHERE tenant_id=$1 AND user_or_group_id=$2", w.p.Tenant, id); err != nil {
		return "", dbError(err)
	}
	if err := removeRecord(w.ctx, w.db, w.p, "Group", id); err != nil {
		return "", err
	}
	return id, w.rebuild()
}

func (w setupWriter) regularGroup(id string) error {
	var kind string
	if err := w.db.QueryRow(w.ctx, "SELECT type FROM sc_setup.groups WHERE tenant_id=$1 AND id=$2", w.p.Tenant, id).Scan(&kind); err != nil {
		return dbError(err)
	}
	if kind != "Regular" {
		return errInsufficientAccess
	}
	return nil
}

// groupMember adds a user or a group to a Regular group, refusing cycles.
func (w setupWriter) groupMember(op, id string, data map[string]any) (string, error) {
	if op == "delete" {
		var group string
		if err := w.db.QueryRow(w.ctx, "SELECT group_id FROM sc_setup.group_members WHERE tenant_id=$1 AND id=$2", w.p.Tenant, id).Scan(&group); err != nil {
			return "", dbError(err)
		}
		if err := removeRecord(w.ctx, w.db, w.p, "GroupMember", id); err != nil {
			return "", err
		}
		return id, w.rebuild()
	}
	if err := Validate(w.p, "GroupMember", data, true); err != nil {
		return "", err
	}
	group, err := w.reference(data, "GroupId", "Group")
	if err != nil {
		return "", err
	}
	if err = w.regularGroup(group); err != nil {
		return "", err
	}
	member, _ := data["UserOrGroupId"].(string)
	if member, err = w.memberID(member); err != nil {
		return "", err
	}
	data["UserOrGroupId"] = member
	if member[:3] == "00G" {
		var cycle bool
		if err = w.db.QueryRow(w.ctx, `WITH RECURSIVE inner_groups(id) AS (SELECT $2::varchar UNION SELECT m.user_or_group_id FROM inner_groups i JOIN sc_setup.group_members m ON m.tenant_id=$1 AND m.group_id=i.id WHERE m.user_or_group_id LIKE '00G%')
			SELECT EXISTS(SELECT FROM inner_groups WHERE id=$3)`, w.p.Tenant, member, group).Scan(&cycle); err != nil {
			return "", dbError(err)
		}
		if cycle {
			return "", fieldProblem(integrityCode, "A group can't contain itself.", "UserOrGroupId")
		}
	}
	if err = w.duplicate("SELECT id FROM sc_setup.group_members WHERE tenant_id=$1 AND group_id=$2 AND user_or_group_id=$3", group, member); err != nil {
		return "", err
	}
	id, err = insertRecord(w.ctx, w.db, w.p, "GroupMember", data, "", nil)
	if err != nil {
		return "", err
	}
	return id, w.rebuild()
}

// memberID resolves a user or group ID in the tenant; anything else is
// INVALID_CROSS_REFERENCE_KEY.
func (w setupWriter) memberID(s string) (string, error) {
	if len(s) < 3 {
		return "", malformedID("UserOrGroupId", s)
	}
	var table string
	switch s[:3] {
	case "005":
		table = "sc_setup.users"
	case "00G":
		table = "sc_setup.groups"
	default:
		return "", crossReference("UserOrGroupId")
	}
	id, err := recordid.Normalize(s[:3], s)
	if err != nil {
		return "", malformedID("UserOrGroupId", s)
	}
	var exists bool
	if err = w.db.QueryRow(w.ctx, "SELECT EXISTS(SELECT FROM "+table+" WHERE tenant_id=$1 AND id=$2)", w.p.Tenant, id).Scan(&exists); err != nil {
		return "", dbError(err)
	}
	if !exists {
		return "", crossReference("UserOrGroupId")
	}
	return id, nil
}

// afterUser keeps sharing in step with a User write: a new user, a role
// change or an activation change rebuilds the closure, and deactivation
// deletes the manual shares granted to the user (D34-25).
func (w setupWriter) afterUser(op, id string, data map[string]any) error {
	_, role := data["UserRoleId"]
	active, activeSet := data["IsActive"]
	if op != "create" && !role && !activeSet {
		return nil
	}
	if activeSet && active == false {
		if _, err := w.db.Exec(w.ctx, "DELETE FROM sc_sharing.shares WHERE tenant_id=$1 AND user_or_group_id=$2", w.p.Tenant, id); err != nil {
			return dbError(err)
		}
	}
	return w.rebuild()
}
