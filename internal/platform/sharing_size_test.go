package platform

import (
	"context"
	"os"
	"testing"
	"time"

	"salescompel/internal/records"
	soqlpkg "salescompel/internal/soql"
)

// TestSharingSize measures a Private-OWD tenant with 1,000 users in a
// six-level role tree and 100,000 Accounts (Plan 34 Task 13). It runs only
// with SALESCOMPEL_SIZE_TEST=1; the measured budget is recorded in
// docs/compat/plan-34-sharing.md.
func TestSharingSize(t *testing.T) {
	if os.Getenv("SALESCOMPEL_SIZE_TEST") != "1" {
		t.Skip("set SALESCOMPEL_SIZE_TEST=1 to run the sharing size test")
	}
	f := newWriteFixture(t)
	ctx := context.Background()
	// Remove the bulk rows before the fixture deletes the tenant: a cascade
	// over them is slow.
	t.Cleanup(func() {
		tx, e := f.s.pool.Begin(ctx)
		if e != nil {
			return
		}
		defer tx.Rollback(ctx)
		for _, sql := range []string{
			"SELECT set_config('salescompel.tenant_id',$1,true)",
			"DELETE FROM public.accounts WHERE tenant_id=$1",
			"DELETE FROM sc_sharing.user_principals WHERE tenant_id=$1",
			"DELETE FROM sc_sharing.user_groups WHERE tenant_id=$1",
			"DELETE FROM public.users WHERE tenant_id=$1 AND username LIKE 'size%'",
			"DELETE FROM sc_setup.groups WHERE tenant_id=$1 AND related_id IS NOT NULL",
			"UPDATE sc_setup.users SET user_role_id=NULL WHERE tenant_id=$1",
			"UPDATE sc_setup.user_roles SET parent_role_id=NULL WHERE tenant_id=$1",
			"DELETE FROM sc_setup.user_roles WHERE tenant_id=$1",
		} {
			if _, e = tx.Exec(ctx, sql, f.tenant); e != nil {
				t.Logf("cleanup %s: %v", sql, e)
				return
			}
		}
		tx.Commit(ctx)
	})
	start := time.Now()
	tx, e := f.s.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	steps := []string{
		"SELECT set_config('salescompel.tenant_id',$1,true)",
		// 6 levels, fan-out 3 below the root: 1+3+9+27+81+243 = 364 roles.
		`WITH RECURSIVE tree(n,parent,depth) AS (SELECT 1,NULL::int,1 UNION ALL SELECT t.n*3+k-1,t.n,t.depth+1 FROM tree t, generate_series(0,2) k WHERE t.depth<6)
		 INSERT INTO sc_setup.user_roles(id,tenant_id,name,developer_name,parent_role_id)
		 SELECT '00E'||lpad(to_hex(n),12,'0')||'AAA',$1,'R'||n,'R'||n,CASE WHEN parent IS NULL THEN NULL ELSE '00E'||lpad(to_hex(parent),12,'0')||'AAA' END FROM tree ORDER BY depth`,
		`INSERT INTO sc_setup.groups(id,tenant_id,name,type,related_id)
		 SELECT '00G'||lpad(to_hex(row_number() OVER ()),12,'0')||'AAA',$1,r.name,k.type,r.id FROM sc_setup.user_roles r, (VALUES('Role'),('RoleAndSubordinates'),('RoleAndSubordinatesInternal')) k(type) WHERE r.tenant_id=$1`,
		`INSERT INTO public.users(id,tenant_id,username,username_canonical,password_hash,role)
		 SELECT '005'||lpad(to_hex(g),12,'0')||'AAA',$1,'size'||g||'@'||$1||'.example.com','size'||g||'@'||$1||'.example.com','x','restricted' FROM generate_series(1,1000) g`,
		`INSERT INTO sc_setup.users(id,tenant_id,username,username_canonical,last_name,alias,email,profile_id,time_zone_sid_key,locale_sid_key,language_locale_key,email_encoding_key,user_role_id)
		 SELECT u.id,u.tenant_id,u.username,u.username_canonical,'Size','size',u.username,p.id,'America/Los_Angeles','en_US','en_US','UTF-8',
		  '00E'||lpad(to_hex(1+(('x'||substr(u.id,4,12))::bit(48)::bigint % 364)),12,'0')||'AAA'
		 FROM public.users u JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.name='Standard User' WHERE u.tenant_id=$1 AND u.username LIKE 'size%'`,
		`INSERT INTO sc_setup.permission_set_assignments(id,tenant_id,assignee_id,permission_set_id)
		 SELECT '0Pa'||substr(u.id,4,12)||'AAA',u.tenant_id,u.id,p.permission_set_id FROM sc_setup.users u JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.id=u.profile_id WHERE u.tenant_id=$1 AND u.username LIKE 'size%'`,
		`INSERT INTO public.accounts(tenant_id,id,name,owner_id)
		 SELECT $1,'001'||lpad(to_hex(g),12,'0')||'AAA','Size '||g,'005'||lpad(to_hex(1+g%1000),12,'0')||'AAA' FROM generate_series(1,100000) g`,
	}
	for _, s := range steps {
		args := []any{f.tenant}
		if _, e = tx.Exec(ctx, s, args...); e != nil {
			t.Fatalf("%s: %v", s[:40], e)
		}
	}
	if e = records.RebuildSharing(ctx, tx, f.tenant); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	t.Logf("seeded in %s", time.Since(start))
	setOWD(t, f, "Account", "None")
	rebuild := time.Now()
	if e := f.s.within(ctx, Principal{Tenant: f.tenant, User: f.user}, func(_ *Store, _ Principal) error { return nil }); e != nil {
		t.Fatal(e)
	}
	tx2, _ := f.s.pool.Begin(ctx)
	tx2.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", f.tenant)
	if e := records.RebuildSharing(ctx, tx2, f.tenant); e != nil {
		t.Fatal(e)
	}
	tx2.Commit(ctx)
	t.Logf("closure rebuild: %s", time.Since(rebuild))
	// The root's manager (top role) sees everything in their tree; a leaf
	// user sees their own 100 records.
	for _, who := range []struct{ name, role string }{{"top", "00E000000000001AAA"}, {"leaf", "00E00000000016cAAA"}} {
		var user string
		f.s.pool.QueryRow(ctx, "SELECT id FROM sc_setup.users WHERE tenant_id=$1 AND user_role_id=$2 LIMIT 1", f.tenant, who.role).Scan(&user)
		if user == "" {
			continue
		}
		for _, q := range []string{"SELECT COUNT() FROM Account", "SELECT Id, Name FROM Account ORDER BY Name LIMIT 200"} {
			began := time.Now()
			if e := as(t, f, user, func(s *Store, p Principal) error {
				_, e := s.recordTx(p).Query(ctx, q, soqlpkg.Options{})
				return e
			}); e != nil {
				t.Fatal(e)
			}
			took := time.Since(began)
			t.Logf("%s user, %s: %s", who.name, q, took)
			if took > 5*time.Second {
				t.Errorf("%s user, %s took %s (budget 5s)", who.name, q, took)
			}
		}
	}
}
