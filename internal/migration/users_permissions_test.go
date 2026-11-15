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
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/recordid"
)

var seededProfileNames = []string{"Minimum Access", "Read Only", "Standard User", "System Administrator"}

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

func TestMigration7SeedsProfiles(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := up(ctx, p, manifest()[:1]); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Exec(ctx, "INSERT INTO tenants(id) VALUES('old')"); e != nil {
		t.Fatal(e)
	}
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	seedTenant(t, p, "new")
	for _, tenant := range []string{"old", "new"} {
		rows, _ := p.Query(ctx, "SELECT name FROM sc_setup.profiles WHERE tenant_id=$1 ORDER BY name", tenant)
		names, _ := pgx.CollectRows(rows, pgx.RowTo[string])
		if strings.Join(names, ",") != strings.Join(seededProfileNames, ",") {
			t.Fatalf("%s profiles = %v", tenant, names)
		}
		var sets, ownedSets, revision int
		if e := p.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE is_owned_by_profile),
			(SELECT revision FROM sc_setup.revisions WHERE tenant_id=$1) FROM sc_setup.permission_sets WHERE tenant_id=$1`, tenant).Scan(&sets, &ownedSets, &revision); e != nil {
			t.Fatal(e)
		}
		if sets != 4 || ownedSets != 4 || revision != 1 {
			t.Fatalf("%s sets=%d owned=%d revision=%d", tenant, sets, ownedSets, revision)
		}
		for _, profile := range seededProfileNames {
			got, want := profileGrants(t, p, tenant, profile), expectedGrants(profile)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("%s %s grants:\n%s\nwant:\n%s", tenant, profile, strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		}
		var tz, locale, language string
		if e := p.QueryRow(ctx, "SELECT time_zone_sid_key,default_locale_sid_key,language_locale_key FROM sc_catalog.settings WHERE tenant_id=$1", tenant).Scan(&tz, &locale, &language); e != nil {
			t.Fatal(e)
		}
		if tz != "America/Los_Angeles" || locale != "en_US" || language != "en_US" {
			t.Fatalf("%s org defaults %s %s %s", tenant, tz, locale, language)
		}
	}
}

// legacyWithUsers builds a database at migration 6 with one tenant, an admin,
// a restricted user and one Plan 27 custom object, then migrates to head.
func legacyWithUsers(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p := testPool(t)
	ctx := context.Background()
	if e := up(ctx, p, manifest()[:1]); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Exec(ctx, `INSERT INTO tenants VALUES('t1');
		INSERT INTO users(id,tenant_id,username,username_canonical,password_hash,role) VALUES
		('005000000000001AAA','t1','Admin@Example.com','admin@example.com','admin-hash','admin'),
		('005000000000002AAA','t1','restricted.user@example.com','restricted.user@example.com','restricted-hash','restricted')`); e != nil {
		t.Fatal(e)
	}
	if e := up(ctx, p, manifest()[:6]); e != nil {
		t.Fatal(e)
	}
	id, name := metadata.Field{Name: "Id", Column: "id", Options: metadata.Options{Kind: fieldtype.Id}}, metadata.Field{Name: "Name", Column: "name", Length: 80, Createable: true, Updateable: true, NameField: true, Options: metadata.Options{Kind: fieldtype.Text}}
	notes := metadata.Field{Name: "Notes__c", Column: "f_notes", Nillable: true, Length: 80, Createable: true, Updateable: true, Options: metadata.Options{Kind: fieldtype.Text}}
	stmts := []string{
		"SELECT set_config('salescompel.tenant_id','t1',false)",
		"CREATE TABLE sc_records.o_0123456789abcdef0123456789abcdef(tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,id varchar(18) NOT NULL,name varchar(80) NOT NULL,f_notes varchar(80),PRIMARY KEY(tenant_id,id))",
		`INSERT INTO sc_catalog.objects VALUES('0123456789abcdef0123456789abcdef','t1','Project__c','Project','Projects',false,'sc_records','o_0123456789abcdef0123456789abcdef','a00')`,
	}
	for _, s := range stmts {
		if _, e := p.Exec(ctx, s); e != nil {
			t.Fatalf("%s: %v", s, e)
		}
	}
	for i, f := range []metadata.Field{id, name, notes} {
		raw, e := metadata.EncodeDefinition(metadata.Complete(f))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = p.Exec(ctx, "INSERT INTO sc_catalog.fields VALUES($1,'t1','0123456789abcdef0123456789abcdef',$2,$3,$4,$5)", fmt.Sprintf("p-%d", i), f.Name, f.Column, string(raw), i); e != nil {
			t.Fatal(e)
		}
	}
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	return p
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

func TestMigration7MapsRoles(t *testing.T) {
	p := legacyWithUsers(t)
	ctx := context.Background()
	type user struct{ id, profile, last, alias, email, tz, locale, language, encoding string }
	rows, e := p.Query(ctx, `SELECT u.id,p.name,u.last_name,u.alias,u.email,u.time_zone_sid_key,u.locale_sid_key,u.language_locale_key,u.email_encoding_key
		FROM sc_setup.users u JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.id=u.profile_id WHERE u.tenant_id='t1' AND u.is_active ORDER BY u.id`)
	if e != nil {
		t.Fatal(e)
	}
	got, e := pgx.CollectRows(rows, func(r pgx.CollectableRow) (user, error) {
		var u user
		return u, r.Scan(&u.id, &u.profile, &u.last, &u.alias, &u.email, &u.tz, &u.locale, &u.language, &u.encoding)
	})
	if e != nil {
		t.Fatal(e)
	}
	want := []user{
		{"005000000000001AAA", "System Administrator", "Admin", "Admin", "Admin@Example.com", "America/Los_Angeles", "en_US", "en_US", "UTF-8"},
		{"005000000000002AAA", "Standard User", "restricted.user", "restrict", "restricted.user@example.com", "America/Los_Angeles", "en_US", "en_US", "UTF-8"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("migrated users:\n%v\nwant:\n%v", got, want)
	}
	var assignments, mismatched int
	if e = p.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE a.permission_set_id<>p.permission_set_id OR a.id NOT LIKE '0Pa%')
		FROM sc_setup.permission_set_assignments a JOIN sc_setup.users u ON u.tenant_id=a.tenant_id AND u.id=a.assignee_id
		JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.id=u.profile_id`).Scan(&assignments, &mismatched); e != nil || assignments != 2 || mismatched != 0 {
		t.Fatalf("assignments=%d mismatched=%d %v", assignments, mismatched, e)
	}
	var hash string
	if e = p.QueryRow(ctx, "SELECT password_hash FROM public.users WHERE id='005000000000002AAA'").Scan(&hash); e != nil || hash != "restricted-hash" {
		t.Fatalf("credentials changed: %q %v", hash, e)
	}

	admin, restricted := userPermissions(t, p, "t1", want[0].id), userPermissions(t, p, "t1", want[1].id)
	for _, o := range metadata.StandardObjects() {
		for _, f := range o.Fields {
			hidden := o.Name == "Account" && f.Name == "Compat_Text__c"
			if restricted.FieldRead(o.Name, f) == hidden || !admin.FieldRead(o.Name, f) {
				t.Fatalf("%s.%s: restricted read=%v admin read=%v", o.Name, f.Name, restricted.FieldRead(o.Name, f), admin.FieldRead(o.Name, f))
			}
		}
	}
	account := metadata.StandardObjects()[0]
	for _, f := range account.Fields {
		if f.Name == "Phone" && !(restricted.FieldRead("Account", f) && restricted.FieldEdit("Account", f)) {
			t.Fatal("restricted user lost Read and Edit on Account.Phone")
		}
		if f.Name == "Compat_Text__c" && (restricted.FieldRead("Account", f) || restricted.FieldEdit("Account", f)) {
			t.Fatal("restricted user gained Account.Compat_Text__c")
		}
	}
	for _, object := range []string{"Account", "Contact", "Project__c"} {
		if a := restricted.Object(object); !(a.Read && a.Create && a.Edit && a.Delete) || a.ViewAll || a.ModifyAll {
			t.Fatalf("restricted %s access = %+v", object, a)
		}
	}
	notes := metadata.Complete(metadata.Field{Name: "Notes__c", Nillable: true, Createable: true, Updateable: true, Options: metadata.Options{Kind: fieldtype.Text}})
	if !restricted.FieldRead("Project__c", notes) || !restricted.FieldEdit("Project__c", notes) {
		t.Fatal("restricted user lost Project__c.Notes__c")
	}
	if s := admin.System(); !(s.ModifyAllData && s.ViewAllData && s.APIEnabled && s.ManageUsers) {
		t.Fatalf("admin system permissions %+v", s)
	}
	if s := restricted.System(); !s.APIEnabled || s.ModifyAllData || s.ViewAllData || s.ManageUsers {
		t.Fatalf("restricted system permissions %+v", s)
	}
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

func TestRecordsLoginCannotReadCredentials(t *testing.T) {
	admin := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, admin); e != nil {
		t.Fatal(e)
	}
	seedTenant(t, admin, "t1")
	seedTenant(t, admin, "t2")
	records := recordsPool(t, admin)
	if _, e := records.Exec(ctx, "SELECT 1 FROM public.users"); sqlState(e) != "42501" {
		t.Fatalf("records read public.users: %v", e)
	}
	if _, e := records.Exec(ctx, "SELECT sc_setup.id_checksum('005000000000001')"); sqlState(e) != "42501" {
		t.Fatalf("records executed a private helper: %v", e)
	}
	id := newUserID(t)
	call := func(tenant, sql string, args ...any) error {
		return inTenant(records, tenant, func(tx pgx.Tx) error { _, e := tx.Exec(ctx, sql, args...); return e })
	}
	if e := call("t1", "SELECT sc_setup.create_identity($1,$2,$3)", id, "t1", "New.User@example.com"); e != nil {
		t.Fatalf("create_identity: %v", e)
	}
	for name, c := range map[string]struct {
		state string
		args  []any
	}{
		"other tenant":     {"42501", []any{newUserID(t), "t2", "x@example.com"}},
		"wrong prefix":     {"22023", []any{"001" + id[3:], "t1", "y@example.com"}},
		"bad checksum":     {"22023", []any{id[:15] + "ZZZ", "t1", "z@example.com"}},
		"no at sign":       {"22023", []any{newUserID(t), "t1", "not-an-email"}},
		"non-ascii":        {"22023", []any{newUserID(t), "t1", "josé@example.com"}},
		"too long":         {"22023", []any{newUserID(t), "t1", strings.Repeat("a", 70) + "@example.com"}},
		"duplicate userid": {"23505", []any{id, "t1", "other@example.com"}},
	} {
		if e := call("t1", "SELECT sc_setup.create_identity($1,$2,$3)", c.args...); sqlState(e) != c.state {
			t.Errorf("%s: got %v, want SQLSTATE %s", name, e, c.state)
		}
	}
	// The records login writes the setup row in the same transaction.
	if e := inTenant(records, "t1", func(tx pgx.Tx) error {
		second := newUserID(t)
		if _, e := tx.Exec(ctx, "SELECT sc_setup.create_identity($1,'t1','second@example.com')", second); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, `INSERT INTO sc_setup.users(id,tenant_id,username,username_canonical,last_name,alias,email,profile_id,time_zone_sid_key,locale_sid_key,language_locale_key,email_encoding_key)
			SELECT $1,'t1','second@example.com','second@example.com','Second','second','second@example.com',id,'America/Los_Angeles','en_US','en_US','UTF-8' FROM sc_setup.profiles WHERE name='Standard User'`, second)
		return e
	}); e != nil {
		t.Fatalf("records setup row: %v", e)
	}
	if e := call("t1", "SELECT sc_setup.rename_identity($1,$2,$3)", id, "t1", "Renamed@example.com"); e != nil {
		t.Fatalf("rename_identity: %v", e)
	}
	if e := call("t2", "SELECT sc_setup.rename_identity($1,$2,$3)", id, "t2", "stolen@example.com"); sqlState(e) != "P0002" {
		t.Fatalf("rename across tenants: %v", e)
	}
	var username, canonical, hash, role, active string
	if e := admin.QueryRow(ctx, "SELECT username,username_canonical,password_hash,role,active::text FROM public.users WHERE id=$1", id).Scan(&username, &canonical, &hash, &role, &active); e != nil {
		t.Fatal(e)
	}
	if username != "Renamed@example.com" || canonical != "renamed@example.com" || hash != "" || role != "restricted" || active != "true" {
		t.Fatalf("identity row %s %s %q %s %v", username, canonical, hash, role, active)
	}
}

func TestCreateIdentityRejectsDuplicateAcrossTenants(t *testing.T) {
	admin := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, admin); e != nil {
		t.Fatal(e)
	}
	seedTenant(t, admin, "t1")
	seedTenant(t, admin, "t2")
	records := recordsPool(t, admin)
	create := func(tenant, id, username string) error {
		return inTenant(records, tenant, func(tx pgx.Tx) error {
			_, e := tx.Exec(ctx, "SELECT sc_setup.create_identity($1,$2,$3)", id, tenant, username)
			return e
		})
	}
	if e := create("t1", newUserID(t), "same@example.com"); e != nil {
		t.Fatal(e)
	}
	other := newUserID(t)
	e := create("t2", other, "Same@Example.com")
	var pgErr *pgconn.PgError
	if !errors.As(e, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "users_username_canonical_key" {
		t.Fatalf("duplicate across tenants: %v", e)
	}
	if e = create("t2", other, "different@example.com"); e != nil {
		t.Fatal(e)
	}
	e = inTenant(records, "t2", func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, "SELECT sc_setup.rename_identity($1,'t2','SAME@example.com')", other)
		return e
	})
	if !errors.As(e, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "users_username_canonical_key" {
		t.Fatalf("rename onto another tenant's username: %v", e)
	}
}

func TestSetupTablesForceRLS(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	rows, e := p.Query(ctx, `SELECT c.relname,c.relrowsecurity,c.relforcerowsecurity,
		(SELECT count(*) FROM pg_policy WHERE polrelid=c.oid AND polname='tenant_isolation'),pg_get_userbyid(c.relowner)
		FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='sc_setup' AND c.relkind IN ('r','p') ORDER BY c.relname`)
	if e != nil {
		t.Fatal(e)
	}
	var tables []string
	for rows.Next() {
		var name, owner string
		var rls, force bool
		var policies int
		if e = rows.Scan(&name, &rls, &force, &policies, &owner); e != nil {
			t.Fatal(e)
		}
		if !rls || !force || policies != 1 || owner != "salescompel_owner" {
			t.Errorf("sc_setup.%s rls=%v force=%v policies=%d owner=%s", name, rls, force, policies, owner)
		}
		tables = append(tables, name)
	}
	rows.Close()
	want := "field_permissions,object_permissions,permission_set_assignments,permission_sets,profiles,revisions,users"
	if strings.Join(tables, ",") != want {
		t.Fatalf("sc_setup tables = %v", tables)
	}
}
