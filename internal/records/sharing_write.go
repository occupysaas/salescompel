package records

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/metadata"
	"salescompel/internal/soql"
)

// Write-path sharing (Plan 34, shr-11). A hidden record answers exactly as
// an unknown ID; a readable record without the needed level is
// INSUFFICIENT_ACCESS_OR_READONLY (D34-01..03).

// recordAccess reports whether the caller can read the record and whether
// they have level on it. A missing record reads as not readable.
func recordAccess(ctx context.Context, db database, p Principal, o Object, id string, level soql.Level) (read, ok bool, err error) {
	args := []any{p.Tenant, id}
	add := func(v any) int { args = append(args, v); return len(args) }
	clause := func(l soql.Level) string {
		if c := soql.SharingClause(&p, "r", o, l, add); c != "" {
			return c
		}
		return "TRUE"
	}
	readSQL, levelSQL := clause(soql.LevelRead), clause(level)
	err = db.QueryRow(ctx, "SELECT "+readSQL+","+levelSQL+" FROM "+o.TableSQL()+" r WHERE r.tenant_id=$1 AND r.id=$2", args...).Scan(&read, &ok)
	if err == pgx.ErrNoRows {
		return false, false, nil
	}
	return read, ok, dbError(err)
}

// requireAccess returns the error for a write the caller can't make.
func requireAccess(ctx context.Context, db database, p Principal, o Object, id string, level soql.Level) error {
	read, ok, err := recordAccess(ctx, db, p, o, id, level)
	switch {
	case err != nil:
		return err
	case !read:
		return dbError(pgx.ErrNoRows)
	case !ok:
		return errInsufficientAccess
	}
	return nil
}

// ownerChange returns the new owner when data changes the record's owner.
func ownerChange(ctx context.Context, db database, p Principal, o Object, id string, data map[string]any) (newOwner, oldOwner string, changed bool, err error) {
	for k, v := range data {
		if !strings.EqualFold(k, "OwnerId") {
			continue
		}
		s, _ := v.(string)
		var current *string
		if err = db.QueryRow(ctx, "SELECT owner_id FROM "+o.TableSQL()+" WHERE tenant_id=$1 AND id=$2", p.Tenant, id).Scan(&current); err != nil {
			return "", "", false, dbError(err)
		}
		if current != nil {
			oldOwner = *current
		}
		normalized, nerr := normalizeID(p, "User", s)
		if nerr != nil {
			return "", "", false, nil // Validate reports the bad value
		}
		return normalized, oldOwner, normalized != oldOwner, nil
	}
	return "", "", false, nil
}

// checkUpdate guards an update: Edit access, and owner-level access for a
// transfer (owner, above the owner, Modify All; D34-09).
func checkUpdate(ctx context.Context, db database, p Principal, o Object, id string, data map[string]any) (transfer func() error, err error) {
	if !hasOwner(o) {
		return nil, nil
	}
	nid, err := normalizeID(p, o.Name, id)
	if err != nil {
		return nil, dbError(pgx.ErrNoRows)
	}
	if err = requireAccess(ctx, db, p, o, nid, soql.LevelEdit); err != nil {
		return nil, err
	}
	newOwner, oldOwner, changed, err := ownerChange(ctx, db, p, o, nid, data)
	if err != nil || !changed {
		return nil, err
	}
	if err = requireAccess(ctx, db, p, o, nid, soql.LevelDelete); err != nil {
		return nil, err
	}
	return func() error { return afterTransfer(ctx, db, p, o, nid, oldOwner, newOwner) }, nil
}

// afterTransfer deletes the record's manual shares and, for an Account,
// moves the old owner's Contacts under it to the new owner and deletes
// their manual shares (D34-10, D34-11).
func afterTransfer(ctx context.Context, db database, p Principal, o Object, id, oldOwner, newOwner string) error {
	if _, err := db.Exec(ctx, "DELETE FROM sc_sharing.shares WHERE tenant_id=$1 AND record_id=$2", p.Tenant, id); err != nil {
		return dbError(err)
	}
	if !strings.EqualFold(o.Name, "Account") || oldOwner == "" {
		return nil
	}
	contact, err := p.Catalog.Object("Contact")
	if err != nil {
		return nil
	}
	_, err = db.Exec(ctx, `WITH moved AS (UPDATE `+contact.TableSQL()+` SET owner_id=$3 WHERE tenant_id=$1 AND account_id=$2 AND owner_id=$4 RETURNING id)
		DELETE FROM sc_sharing.shares s USING moved WHERE s.tenant_id=$1 AND s.record_id=moved.id`, p.Tenant, id, newOwner, oldOwner)
	return dbError(err)
}

// checkDelete requires owner-level access (shr-11).
func checkDelete(ctx context.Context, db database, p Principal, o Object, id string) (string, error) {
	nid, err := normalizeID(p, o.Name, id)
	if err != nil || !hasOwner(o) {
		return nid, nil // removeRecord reports a malformed ID
	}
	return nid, requireAccess(ctx, db, p, o, nid, soql.LevelDelete)
}

// checkLookups refuses a lookup to a record the caller can't read, as an
// unknown ID (shr-11).
func checkLookups(ctx context.Context, db database, p Principal, object string, data map[string]any) error {
	for k, v := range data {
		s, ok := v.(string)
		if !ok || s == "" || strings.EqualFold(k, "OwnerId") {
			continue
		}
		f, err := field(p, object, k)
		if err != nil || f.Type != "reference" || len(f.ReferenceTo) != 1 || metadata.IsSetupObject(f.ReferenceTo[0]) {
			continue
		}
		target, err := p.Catalog.Object(f.ReferenceTo[0])
		if err != nil || !hasOwner(target) {
			continue
		}
		read, _, err := recordAccess(ctx, db, p, target, s, soql.LevelRead)
		if err != nil {
			return err
		}
		if !read {
			// Missing parents keep their existing error; hidden ones match it.
			var exists bool
			if err = db.QueryRow(ctx, "SELECT EXISTS(SELECT FROM "+target.TableSQL()+" WHERE tenant_id=$1 AND id=$2)", p.Tenant, s).Scan(&exists); err != nil {
				return dbError(err)
			}
			if exists {
				return invalid("INVALID_CROSS_REFERENCE_KEY", "Relationship is unavailable or still referenced")
			}
		}
	}
	return nil
}

// dropShares removes a deleted record's manual shares.
func dropShares(ctx context.Context, db database, p Principal, id string) error {
	_, err := db.Exec(ctx, "DELETE FROM sc_sharing.shares WHERE tenant_id=$1 AND record_id=$2", p.Tenant, id)
	return dbError(err)
}
