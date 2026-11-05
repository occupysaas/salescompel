package migration

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"salescompel/internal/migration/legacy"
	"testing"
	"time"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL required")
	}
	root, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	name := fmt.Sprintf("migration_%d", time.Now().UnixNano())
	if _, e = root.Exec(ctx, "CREATE DATABASE "+name); e != nil {
		t.Fatal(e)
	}
	cfg, _ := pgxpool.ParseConfig(url)
	cfg.ConnConfig.Database = name
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		p.Close()
		r, _ := pgxpool.New(ctx, url)
		defer r.Close()
		r.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
	})
	return p
}
func TestFreshMigration(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e := CheckHead(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	entries, e := Status(ctx, p)
	if e != nil || len(entries) < 1 || !entries[0].Applied || len(entries[0].Checksum) != 64 {
		t.Fatal(entries, e)
	}
}
func TestMigrationHistory(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Exec(ctx, "UPDATE sc_control.schema_migrations SET checksum='tampered' WHERE version=1"); e != nil {
		t.Fatal(e)
	}
	if e := Up(ctx, p); e == nil {
		t.Fatal("accepted changed history")
	}
}
func TestUnmigratedHead(t *testing.T) {
	p := testPool(t)
	if e := CheckHead(context.Background(), p); e == nil {
		t.Fatal("accepted unmigrated database")
	}
}
func TestMigrationRejectsUnexplained(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	p.Exec(ctx, "CREATE TABLE public.unknown(id int)")
	if e := Up(ctx, p); e == nil {
		t.Fatal("overwrote unexplained database")
	}
}
func TestMigrationRejectsIdentityDrift(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = legacy.Install(ctx, tx); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "ALTER TABLE users ALTER COLUMN password_hash TYPE varchar(255)"); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	if e = Up(ctx, p); e == nil {
		t.Fatal("accepted a changed identity layout")
	}
}
func TestLegacyAdoption(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = legacy.Install(ctx, tx); e != nil {
		t.Fatal(e)
	}
	for _, sql := range []string{
		"INSERT INTO tenants VALUES('adopt')",
		"INSERT INTO accounts(tenant_id,id,name,compat_key,compat_key_canonical)VALUES('adopt','001000000000001AAA','Retain','KEY','key')",
		"INSERT INTO users(id,tenant_id,username,username_canonical,password_hash,role,auth_version,failed_attempts,locked_until)VALUES('005000000000001AAA','adopt','user@example.invalid','user@example.invalid','hashed-password','admin',7,2,'2030-01-01')",
		"INSERT INTO oauth_apps(client_id,tenant_id,name,redirect_uris,grants,scopes)VALUES('app','adopt','App','{}','{refresh_token}','{api}')",
		"INSERT INTO oauth_codes(code_hash,client_id,user_id,redirect_uri,scope,expires_at,used)VALUES('code','app','005000000000001AAA','https://example.invalid/cb','api','2030-01-01',true)",
		"INSERT INTO oauth_refresh_tokens(token_hash,client_id,user_id,scope,issued_at,code_hash)VALUES('refresh','app','005000000000001AAA','api','2026-01-01','code')",
		"INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,issued_at,expires_at,refresh_token_hash,code_hash)VALUES('access','app','005000000000001AAA','api','2026-01-01','2030-01-01','refresh','code')",
		"INSERT INTO oauth_requests(request_hash,client_id,redirect_uri,scope,cookie_hash,expires_at)VALUES('pending','app','https://example.invalid/cb','api','binding','2030-01-01')",
		"INSERT INTO oauth_jwt_ids VALUES('app','jti','2030-01-01')",
	} {
		if _, e = tx.Exec(ctx, sql); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	snapshot := func() string {
		out := ""
		for _, table := range []string{"accounts", "users", "oauth_apps", "oauth_codes", "oauth_refresh_tokens", "oauth_access_tokens", "oauth_requests", "oauth_jwt_ids"} {
			var raw string
			if e := p.QueryRow(ctx, "SELECT COALESCE(jsonb_agg(to_jsonb(t)),'[]'::jsonb)::text FROM "+table+" t").Scan(&raw); e != nil {
				t.Fatal(e)
			}
			out += raw
		}
		return out
	}
	before := snapshot()
	if e = Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if after := snapshot(); after != before {
		t.Fatal("adoption changed business/identity data")
	}
}
func TestConcurrentMigrate(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	errs := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func() { errs <- Up(ctx, p) }()
	}
	for n := 0; n < 2; n++ {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
	}
	entries, e := Status(ctx, p)
	if e != nil || len(entries) != 3 {
		t.Fatal(entries, e)
	}
}
func TestMigrationResume(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	changes := manifest()
	changes[1].run = func(ctx context.Context, tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, "CREATE TABLE public.should_rollback(id int)"); e != nil {
			return e
		}
		return errors.New("injected migration failure")
	}
	if e := up(ctx, p, changes); e == nil {
		t.Fatal("ignored failure")
	}
	var versions int
	var absent bool
	if e := p.QueryRow(ctx, "SELECT count(*) FROM sc_control.schema_migrations").Scan(&versions); e != nil || versions != 1 {
		t.Fatal(versions, e)
	}
	p.QueryRow(ctx, "SELECT to_regclass('public.should_rollback') IS NULL").Scan(&absent)
	if !absent {
		t.Fatal("failed migration DDL committed")
	}
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e := CheckHead(ctx, p); e != nil {
		t.Fatal(e)
	}
}
func TestMigrationRejectsLegacyPolicies(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	tx, _ := p.Begin(ctx)
	if e := legacy.Install(ctx, tx); e != nil {
		t.Fatal(e)
	}
	if _, e := tx.Exec(ctx, "CREATE POLICY tenant_isolation ON accounts USING(true) WITH CHECK(true)"); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	if e := Up(ctx, p); e == nil {
		t.Fatal("adopted an open legacy tenant policy")
	}
}
func TestMigrationRejectsPolicyAtRLSInstall(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := up(ctx, p, manifest()[:2]); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Exec(ctx, "CREATE POLICY allow_all ON accounts USING(true) WITH CHECK(true)"); e != nil {
		t.Fatal(e)
	}
	if e := Up(ctx, p); e == nil {
		t.Fatal("retained an unexpected permissive policy")
	}
	var head int
	if e := p.QueryRow(ctx, "SELECT max(version) FROM sc_control.schema_migrations").Scan(&head); e != nil || head != 2 {
		t.Fatal(head, e)
	}
}
func TestMigrationRejectsPartialIdentity(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	tx, _ := p.Begin(ctx)
	if e := legacy.Install(ctx, tx); e != nil {
		t.Fatal(e)
	}
	if _, e := tx.Exec(ctx, "DROP TABLE oauth_jwt_ids"); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	if e := Up(ctx, p); e == nil {
		t.Fatal("accepted incomplete identity tables")
	}
	var absent bool
	if e := p.QueryRow(ctx, "SELECT to_regclass('sc_control.schema_migrations') IS NULL AND to_regclass('oauth_jwt_ids') IS NULL").Scan(&absent); e != nil || !absent {
		t.Fatal("partial adoption changed schema", e)
	}
}
func TestLegacyOptionalAuthVersion(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	tx, _ := p.Begin(ctx)
	if e := legacy.Install(ctx, tx); e != nil {
		t.Fatal(e)
	}
	if _, e := tx.Exec(ctx, "INSERT INTO tenants VALUES('legacy');INSERT INTO users(id,tenant_id,username,username_canonical,password_hash,role)VALUES('005000000000001AAA','legacy','legacy@example.invalid','legacy@example.invalid','retain-hash','admin');ALTER TABLE users DROP COLUMN auth_version"); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	var version int64
	var hash string
	if e := p.QueryRow(ctx, "SELECT auth_version,password_hash FROM users WHERE username_canonical='legacy@example.invalid'").Scan(&version, &hash); e != nil || version != 0 || hash != "retain-hash" {
		t.Fatal("optional auth_version upgrade rewrote credentials", version, e)
	}
}
