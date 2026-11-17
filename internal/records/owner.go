package records

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/policy"
)

// hasOwner reports whether the object's records carry OwnerId (every record
// object; never a setup object).
func hasOwner(o Object) bool {
	for _, f := range o.Fields {
		if f.Name == "OwnerId" {
			return true
		}
	}
	return false
}

// defaultOwner is the running user, or for a provisioned static-token
// principal (no User row) the tenant's default owner: the first active
// System Administrator by ID, else the first active user (D34-31).
func defaultOwner(ctx context.Context, db database, p Principal) (string, error) {
	if p.Role == "" && p.User != "" {
		return p.User, nil
	}
	var id string
	err := db.QueryRow(ctx, `SELECT u.id FROM sc_setup.users u JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.id=u.profile_id
		WHERE u.tenant_id=$1 AND u.is_active ORDER BY (p.name='System Administrator') DESC,u.id LIMIT 1`, p.Tenant).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil // no user to own it: the owner stays null (fail closed)
	}
	return id, dbError(err)
}

// ownerOnCreate fills OwnerId with the default owner when it is absent or
// null (shr-01). A given owner is checked by checkOwner after the IDs are
// normalized.
func ownerOnCreate(ctx context.Context, db database, p Principal, o Object, data map[string]any) error {
	if !hasOwner(o) {
		return nil
	}
	for k, v := range data {
		if equalFold(k, "OwnerId") && v != nil {
			return nil
		}
		if equalFold(k, "OwnerId") {
			delete(data, k)
		}
	}
	id, err := defaultOwner(ctx, db, p)
	if err != nil || id == "" {
		return err
	}
	data["OwnerId"] = id
	return nil
}

// checkOwner requires a named owner to be an active user in the tenant who
// can read the object (D34-09). Anything else is
// INVALID_CROSS_REFERENCE_KEY, as for a lookup naming no record.
func checkOwner(ctx context.Context, db database, p Principal, object, owner string) error {
	if owner == p.User && p.Role == "" {
		return nil
	}
	var active bool
	err := db.QueryRow(ctx, "SELECT is_active FROM sc_setup.users WHERE tenant_id=$1 AND id=$2", p.Tenant, owner).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
		return crossReference("OwnerId")
	}
	if err != nil {
		return dbError(err)
	}
	perms, _, err := policy.LoadPermissions(ctx, db, p.Tenant, owner)
	if err != nil {
		return dbError(err)
	}
	if !perms.Object(object).Read {
		return crossReference("OwnerId")
	}
	return nil
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
