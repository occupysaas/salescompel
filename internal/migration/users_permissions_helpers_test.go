package migration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/recordid"
)

// seedTenant inserts a tenant after migration and seeds it as runtime code does.
func seedTenant(t *testing.T, p *pgxpool.Pool, tenant string) {
	t.Helper()
	if e := inTenant(p, tenant, func(tx pgx.Tx) error {
		if _, e := tx.Exec(context.Background(), "INSERT INTO public.tenants(id) VALUES($1)", tenant); e != nil {
			return e
		}
		return metadata.SeedTenant(context.Background(), tx, tenant)
	}); e != nil {
		t.Fatal(e)
	}
}

// profileGrants renders a profile's owned set as sorted lines: system
// permissions, then one line per object and field grant.
func profileGrants(t *testing.T, p *pgxpool.Pool, tenant, profile string) []string {
	t.Helper()
	ctx := context.Background()
	var setID, profileID, setProfile, extra string
	var owned, mad, vad, api, mu bool
	if e := p.QueryRow(ctx, `SELECT p.id,s.id,s.profile_id,s.is_owned_by_profile,s.permissions_modify_all_data,s.permissions_view_all_data,s.permissions_api_enabled,s.permissions_manage_users,s.extra_permissions::text
		FROM sc_setup.profiles p JOIN sc_setup.permission_sets s ON s.tenant_id=p.tenant_id AND s.id=p.permission_set_id
		WHERE p.tenant_id=$1 AND p.name=$2`, tenant, profile).Scan(&profileID, &setID, &setProfile, &owned, &mad, &vad, &api, &mu, &extra); e != nil {
		t.Fatalf("%s %s: %v", tenant, profile, e)
	}
	if !owned || setProfile != profileID || extra != "{}" {
		t.Fatalf("%s %s: owned=%v set profile=%s extra=%s", tenant, profile, owned, setProfile, extra)
	}
	for prefix, id := range map[string]string{"00e": profileID, "0PS": setID} {
		if _, e := recordid.Normalize(prefix, id); e != nil || len(id) != 18 {
			t.Fatalf("%s id %q is not a valid %s id", profile, id, prefix)
		}
	}
	out := []string{fmt.Sprintf("system mad=%v vad=%v api=%v mu=%v", mad, vad, api, mu)}
	rows, e := p.Query(ctx, `SELECT id,sobject_type,permissions_read,permissions_create,permissions_edit,permissions_delete,permissions_view_all_records,permissions_modify_all_records
		FROM sc_setup.object_permissions WHERE tenant_id=$1 AND parent_id=$2`, tenant, setID)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var id, object string
		var r, c, ed, d, va, ma bool
		if e = rows.Scan(&id, &object, &r, &c, &ed, &d, &va, &ma); e != nil {
			t.Fatal(e)
		}
		if _, e = recordid.Normalize("110", id); e != nil {
			t.Fatalf("object permission id %q", id)
		}
		out = append(out, fmt.Sprintf("object %s r=%v c=%v e=%v d=%v va=%v ma=%v", object, r, c, ed, d, va, ma))
	}
	rows.Close()
	rows, e = p.Query(ctx, `SELECT id,sobject_type,field,permissions_read,permissions_edit FROM sc_setup.field_permissions WHERE tenant_id=$1 AND parent_id=$2`, tenant, setID)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var id, object, field string
		var r, ed bool
		if e = rows.Scan(&id, &object, &field, &r, &ed); e != nil {
			t.Fatal(e)
		}
		if _, e = recordid.Normalize("01k", id); e != nil || !strings.HasPrefix(field, object+".") {
			t.Fatalf("field permission %q %s %s", id, object, field)
		}
		out = append(out, fmt.Sprintf("field %s r=%v e=%v", field, r, ed))
	}
	rows.Close()
	sort.Strings(out)
	return out
}

// expectedGrants derives the spec's seeded grants for Account and Contact.
func expectedGrants(profile string) []string {
	system := map[string]string{
		"System Administrator": "system mad=true vad=true api=true mu=true",
		"Standard User":        "system mad=false vad=false api=true mu=false",
		"Read Only":            "system mad=false vad=false api=true mu=false",
		"Minimum Access":       "system mad=false vad=false api=false mu=false",
	}
	out := []string{system[profile]}
	edit := profile == "Standard User"
	if edit || profile == "Read Only" {
		for _, o := range metadata.StandardObjects() {
			if o.Name != "Account" && o.Name != "Contact" {
				continue
			}
			out = append(out, fmt.Sprintf("object %s r=true c=%v e=%v d=%v va=false ma=false", o.Name, edit, edit, edit))
			for _, f := range o.Fields {
				if !policy.Permissionable(f) || (edit && o.Name == "Account" && f.Name == "Compat_Text__c") {
					continue
				}
				out = append(out, fmt.Sprintf("field %s.%s r=true e=%v", o.Name, f.Name, edit && (f.Createable || f.Updateable)))
			}
		}
	}
	sort.Strings(out)
	return out
}

// userPermissions loads every set assigned to a user and unions them.
func userPermissions(t *testing.T, p *pgxpool.Pool, tenant, user string) policy.Permissions {
	t.Helper()
	ctx := context.Background()
	rows, e := p.Query(ctx, `SELECT s.id,s.permissions_modify_all_data,s.permissions_view_all_data,s.permissions_api_enabled,s.permissions_manage_users
		FROM sc_setup.permission_set_assignments a JOIN sc_setup.permission_sets s ON s.tenant_id=a.tenant_id AND s.id=a.permission_set_id
		WHERE a.tenant_id=$1 AND a.assignee_id=$2`, tenant, user)
	if e != nil {
		t.Fatal(e)
	}
	var sets []policy.PermissionSet
	for rows.Next() {
		s := policy.PermissionSet{Objects: map[string]policy.ObjectAccess{}, Fields: map[string]policy.FieldAccess{}}
		if e = rows.Scan(&s.ID, &s.System.ModifyAllData, &s.System.ViewAllData, &s.System.APIEnabled, &s.System.ManageUsers); e != nil {
			t.Fatal(e)
		}
		sets = append(sets, s)
	}
	rows.Close()
	for _, s := range sets {
		rows, _ = p.Query(ctx, `SELECT sobject_type,permissions_read,permissions_create,permissions_edit,permissions_delete,permissions_view_all_records,permissions_modify_all_records FROM sc_setup.object_permissions WHERE tenant_id=$1 AND parent_id=$2`, tenant, s.ID)
		for rows.Next() {
			var name string
			var a policy.ObjectAccess
			rows.Scan(&name, &a.Read, &a.Create, &a.Edit, &a.Delete, &a.ViewAll, &a.ModifyAll)
			s.Objects[name] = a
		}
		rows.Close()
		rows, _ = p.Query(ctx, `SELECT field,permissions_read,permissions_edit FROM sc_setup.field_permissions WHERE tenant_id=$1 AND parent_id=$2`, tenant, s.ID)
		for rows.Next() {
			var name string
			var a policy.FieldAccess
			rows.Scan(&name, &a.Read, &a.Edit)
			s.Fields[name] = a
		}
		rows.Close()
	}
	return policy.Union(sets...)
}

// recordsPool connects as the records login to the test's own database.
func recordsPool(t *testing.T, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_RECORDS_URL")
	if url == "" {
		t.Fatal("TEST_RECORDS_URL required")
	}
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.Database = admin.Config().ConnConfig.Database
	p, e := pgxpool.NewWithConfig(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	return p
}

// inTenant runs fn in a transaction scoped to tenant and commits it.
func inTenant(p *pgxpool.Pool, tenant string, fn func(pgx.Tx) error) error {
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenant); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func sqlState(e error) (code string) {
	if pgErr := (*pgconn.PgError)(nil); errors.As(e, &pgErr) {
		code = pgErr.Code
	}
	return code
}

func newUserID(t *testing.T) string {
	t.Helper()
	id, e := recordid.New("005")
	if e != nil {
		t.Fatal(e)
	}
	return id
}
