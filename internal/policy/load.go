package policy

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/metadata"
)

// Querier is a pgx transaction (or anything reading like one). Callers scope
// it to the tenant: every sc_setup table has forced row-level security.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Revision is the tenant's permission revision, bumped by every setup write.
func Revision(ctx context.Context, q Querier, tenant string) (int64, error) {
	var rev int64
	e := q.QueryRow(ctx, "SELECT revision FROM sc_setup.revisions WHERE tenant_id=$1", tenant).Scan(&rev)
	return rev, e
}

const setColumns = "s.id,s.permissions_modify_all_data,s.permissions_view_all_data,s.permissions_api_enabled,s.permissions_manage_users"

// LoadPermissions returns the union of every permission set assigned to the
// user (the profile-owned set arrives through its assignment) and the
// revision it reflects. An unknown or inactive user, or one whose profile is
// missing, gets empty permissions and no error (fail closed).
//
// The revision is read first, so the sets read after it are never older than
// the revision they are cached under.
func LoadPermissions(ctx context.Context, q Querier, tenant, user string) (Permissions, int64, error) {
	rev, e := Revision(ctx, q, tenant)
	if e != nil {
		return Permissions{}, 0, e
	}
	sets, e := loadSets(ctx, q, tenant, `SELECT `+setColumns+` FROM sc_setup.users u
		JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.id=u.profile_id
		JOIN sc_setup.permission_set_assignments a ON a.tenant_id=u.tenant_id AND a.assignee_id=u.id
		JOIN sc_setup.permission_sets s ON s.tenant_id=a.tenant_id AND s.id=a.permission_set_id
		WHERE u.tenant_id=$1 AND u.id=$2 AND u.is_active`, user)
	if e != nil {
		return Permissions{}, 0, e
	}
	return Union(sets...), rev, nil
}

// ErrUnknownRole is returned for a provisioned role with no seeded profile.
var ErrUnknownRole = errors.New("unknown provisioned role")

// roleProfiles maps a provisioned token's stored role to its seeded profile.
var roleProfiles = map[string]string{"admin": "System Administrator", "restricted": "Standard User"}

// RoleProfilePermissions gives a provisioned (static-token) principal the
// permissions of the seeded profile its role maps to, and the revision.
func RoleProfilePermissions(ctx context.Context, q Querier, tenant, role string) (Permissions, int64, error) {
	profile, ok := roleProfiles[role]
	if !ok {
		return Permissions{}, 0, ErrUnknownRole
	}
	rev, e := Revision(ctx, q, tenant)
	if e != nil {
		return Permissions{}, 0, e
	}
	sets, e := loadSets(ctx, q, tenant, `SELECT `+setColumns+` FROM sc_setup.profiles p
		JOIN sc_setup.permission_sets s ON s.tenant_id=p.tenant_id AND s.id=p.permission_set_id
		WHERE p.tenant_id=$1 AND p.name=$2`, profile)
	if e != nil {
		return Permissions{}, 0, e
	}
	return Union(sets...), rev, nil
}

// loadSets reads the sets selected by query ($1 tenant, $2 arg) with their
// object and field grants. Grants naming a setup object are ignored: those
// objects are reached only through system permissions.
func loadSets(ctx context.Context, q Querier, tenant, query, arg string) ([]PermissionSet, error) {
	rows, e := q.Query(ctx, query, tenant, arg)
	if e != nil {
		return nil, e
	}
	sets, e := pgx.CollectRows(rows, func(r pgx.CollectableRow) (PermissionSet, error) {
		s := PermissionSet{Objects: map[string]ObjectAccess{}, Fields: map[string]FieldAccess{}}
		return s, r.Scan(&s.ID, &s.System.ModifyAllData, &s.System.ViewAllData, &s.System.APIEnabled, &s.System.ManageUsers)
	})
	if e != nil || len(sets) == 0 {
		return sets, e
	}
	index := map[string]int{}
	ids := make([]string, len(sets))
	for i, s := range sets {
		index[s.ID], ids[i] = i, s.ID
	}
	rows, e = q.Query(ctx, `SELECT parent_id,sobject_type,permissions_read,permissions_create,permissions_edit,permissions_delete,permissions_view_all_records,permissions_modify_all_records
		FROM sc_setup.object_permissions WHERE tenant_id=$1 AND parent_id=ANY($2)`, tenant, ids)
	if e != nil {
		return nil, e
	}
	var parent, name string
	var o ObjectAccess
	if _, e = pgx.ForEachRow(rows, []any{&parent, &name, &o.Read, &o.Create, &o.Edit, &o.Delete, &o.ViewAll, &o.ModifyAll}, func() error {
		if !metadata.IsSetupObject(name) {
			sets[index[parent]].Objects[name] = o
		}
		return nil
	}); e != nil {
		return nil, e
	}
	rows, e = q.Query(ctx, `SELECT parent_id,sobject_type,field,permissions_read,permissions_edit
		FROM sc_setup.field_permissions WHERE tenant_id=$1 AND parent_id=ANY($2)`, tenant, ids)
	if e != nil {
		return nil, e
	}
	var field string
	var f FieldAccess
	if _, e = pgx.ForEachRow(rows, []any{&parent, &name, &field, &f.Read, &f.Edit}, func() error {
		if !metadata.IsSetupObject(name) {
			sets[index[parent]].Fields[field] = f
		}
		return nil
	}); e != nil {
		return nil, e
	}
	return sets, nil
}

// Cache holds effective permissions per (tenant, user) at one revision. A
// lookup at any other revision misses, so a stale set is never served. The
// zero value is ready to use.
type Cache struct {
	mu      sync.Mutex
	entries map[[2]string]cached
}

type cached struct {
	revision int64
	perms    Permissions
}

// Get returns the cached permissions only if they were stored at revision.
func (c *Cache) Get(tenant, user string, revision int64) (Permissions, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[[2]string{tenant, user}]
	if !ok || v.revision != revision {
		return Permissions{}, false
	}
	return v.perms, true
}

// Put stores permissions computed at revision, unless a newer revision is
// already cached.
func (c *Cache) Put(tenant, user string, revision int64, p Permissions) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[[2]string]cached{}
	}
	key := [2]string{tenant, user}
	if v, ok := c.entries[key]; ok && v.revision > revision {
		return
	}
	c.entries[key] = cached{revision, p}
}
