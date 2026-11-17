package records

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/recordid"
)

// setupWriter carries one setup write's transaction and caller.
type setupWriter struct {
	ctx context.Context
	db  database
	p   Principal
}

// setupWrite applies the setup-object rules (spec "Consistency rules") to a
// create, update or delete of User, PermissionSet, ObjectPermissions,
// FieldPermissions or PermissionSetAssignment, and bumps the tenant's
// permission revision in the same transaction. op is create, update or
// delete; id is empty on create. writeAccess has already refused hidden
// objects and unsupported operations. It returns the record ID.
func setupWrite(ctx context.Context, db database, p Principal, op, object, id string, data map[string]any) (string, error) {
	if s := p.Perms.System(); !s.ManageUsers && !s.ModifyAllData {
		return "", errInsufficientAccess // D33-08
	}
	var err error
	if op != "create" {
		if id, err = normalizeID(p, object, id); err != nil {
			return "", dbError(pgx.ErrNoRows)
		}
	}
	if data, err = canonicalFields(p, object, data); err != nil {
		return "", err
	}
	w := setupWriter{ctx: ctx, db: db, p: p}
	// The revision row lock also serializes the tenant's setup writes, so
	// two concurrent demotions can't both pass the last-admin check.
	if err = w.bumpRevision(); err != nil {
		return "", err
	}
	var before adminHolders
	if op != "create" {
		if before, err = w.adminHolders(); err != nil {
			return "", err
		}
	}
	switch object {
	case "User":
		if _, err = w.reference(data, "UserRoleId", "UserRole"); err != nil {
			return "", err
		}
		if id, err = w.user(op, id, data); err == nil {
			err = w.afterUser(op, id, data)
		}
	case "UserRole":
		id, err = w.userRole(op, id, data)
	case "Group":
		id, err = w.group(op, id, data)
	case "GroupMember":
		id, err = w.groupMember(op, id, data)
	case "PermissionSet":
		id, err = w.permissionSet(op, id, data)
	case "ObjectPermissions":
		id, err = w.objectPermissions(op, id, data)
	case "FieldPermissions":
		id, err = w.fieldPermissions(op, id, data)
	case "PermissionSetAssignment":
		id, err = w.assignment(op, id, data)
	default:
		return "", errInsufficientAccess // D33-31: Profile and Organization are read-only
	}
	if err != nil || op == "create" {
		return id, err
	}
	after, err := w.adminHolders()
	if err != nil {
		return "", err
	}
	if before.manageUsers > 0 && after.manageUsers == 0 || before.modifyAllData > 0 && after.modifyAllData == 0 {
		// D33-28
		return "", invalid("INVALID_OPERATION", "Can't remove Manage Users or Modify All Data from the last active user who has it.")
	}
	return id, nil
}

func (w setupWriter) bumpRevision() error {
	tag, err := w.db.Exec(w.ctx, "UPDATE sc_setup.revisions SET revision=revision+1 WHERE tenant_id=$1", w.p.Tenant)
	if err == nil && tag.RowsAffected() != 1 {
		err = errors.New("tenant has no permission revision")
	}
	return dbError(err)
}

// adminHolders counts the tenant's active users holding each permission
// through their effective union (profile-owned set plus assigned sets).
// Provisioned static-token principals have no User row and never count.
type adminHolders struct{ manageUsers, modifyAllData int }

func (w setupWriter) adminHolders() (adminHolders, error) {
	var h adminHolders
	err := w.db.QueryRow(w.ctx, `SELECT count(DISTINCT u.id) FILTER (WHERE s.permissions_manage_users),
			count(DISTINCT u.id) FILTER (WHERE s.permissions_modify_all_data)
		FROM sc_setup.users u
		JOIN sc_setup.permission_set_assignments a ON a.tenant_id=u.tenant_id AND a.assignee_id=u.id
		JOIN sc_setup.permission_sets s ON s.tenant_id=a.tenant_id AND s.id=a.permission_set_id
		WHERE u.tenant_id=$1 AND u.is_active`, w.p.Tenant).Scan(&h.manageUsers, &h.modifyAllData)
	return h, dbError(err)
}

// reference resolves a lookup value in data to the target's 18-character ID
// and stores it back. An absent, null or empty value returns "" and is left
// to Validate. A malformed ID is MALFORMED_ID; a well-formed ID of another
// object, or one naming no record, is INVALID_CROSS_REFERENCE_KEY (D33-29,
// D33-30).
func (w setupWriter) reference(data map[string]any, field, target string) (string, error) {
	v, ok := data[field]
	if !ok || v == nil || v == "" {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fieldProblem("JSON_PARSER_ERROR", "Expected a string or null", field)
	}
	if len(s) < 3 {
		return "", malformedID(field, s)
	}
	if _, err := recordid.Normalize(s[:3], s); err != nil {
		return "", malformedID(field, s)
	}
	o, err := w.p.Catalog.Object(target)
	if err != nil {
		return "", err
	}
	id, err := recordid.Normalize(o.Prefix, s)
	if err != nil {
		return "", crossReference(field)
	}
	var exists bool
	if err = w.db.QueryRow(w.ctx, "SELECT EXISTS(SELECT FROM "+o.TableSQL()+" WHERE tenant_id=$1 AND id=$2)", w.p.Tenant, id).Scan(&exists); err != nil {
		return "", dbError(err)
	}
	if !exists {
		return "", crossReference(field)
	}
	data[field] = id
	return id, nil
}

func malformedID(field, value string) error {
	return fieldProblem("MALFORMED_ID", field+": id value of incorrect type: "+value, field)
}

func crossReference(field string) error {
	return fieldProblem("INVALID_CROSS_REFERENCE_KEY", "invalid cross reference id", field)
}

// duplicate returns D33-14's error when query ($1 tenant, then args) finds
// an existing row's ID.
func (w setupWriter) duplicate(query string, args ...any) error {
	var existing string
	err := w.db.QueryRow(w.ctx, query, append([]any{w.p.Tenant}, args...)...).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return dbError(err)
	}
	return invalid("DUPLICATE_VALUE", "duplicate value found: <unknown> duplicates value on record with id: "+existing) // D33-14
}

// flag reads a checkbox value from data, falling back to current when the
// field is absent or not a boolean (Validate reports the latter).
func flag(data map[string]any, field string, current bool) bool {
	if b, ok := data[field].(bool); ok {
		return b
	}
	if v, ok := data[field]; ok && v == nil {
		return false
	}
	return current
}
