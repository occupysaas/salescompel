package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func migrationDB(t *testing.T) (string, *pgx.Conn) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	c, e := pgx.Connect(ctx, databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("migration_%d", time.Now().UnixNano())
	ident := pgx.Identifier{name}.Sanitize()
	if _, e = c.Exec(ctx, "CREATE DATABASE "+ident); e != nil {
		t.Fatal(e)
	}
	parsed, e := url.Parse(databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	parsed.Path = "/" + name
	db, e := pgx.Connect(ctx, parsed.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(ctx); c.Exec(ctx, "DROP DATABASE "+ident+" WITH(FORCE)"); c.Close(ctx) })
	return parsed.String(), db
}
func sqlDo(t *testing.T, c *pgx.Conn, q string, args ...any) {
	t.Helper()
	if _, e := c.Exec(context.Background(), q, args...); e != nil {
		t.Fatal(e)
	}
}
func seedLegacy(t *testing.T, c *pgx.Conn) {
	t.Helper()
	sqlDo(t, c, schema)
	b, e := os.ReadFile("catalog_v1.json")
	if e != nil {
		t.Fatal(e)
	}
	sqlDo(t, c, "INSERT INTO platform_catalog VALUES(true,1,$1)", string(b))
	sqlDo(t, c, "INSERT INTO tenants VALUES('first'),('other')")
	sqlDo(t, c, "INSERT INTO accounts VALUES('first','001000000000001AAA','A',$1,NULL),('other','001000000000002AAA','B',NULL,'key')", strings.Repeat("😀", 80))
	sqlDo(t, c, "INSERT INTO contacts VALUES('first','003000000000001AAA',NULL,'Contact','001000000000001AAA')")
}
func snapshot(t *testing.T, c *pgx.Conn) string {
	t.Helper()
	var s string
	e := c.QueryRow(context.Background(), `SELECT jsonb_build_object(
 'accounts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY tenant_id,id) FROM accounts a),
 'contacts',(SELECT jsonb_agg(to_jsonb(c) ORDER BY tenant_id,id) FROM contacts c),
 'tenants',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tenants t),
 'catalog',(SELECT jsonb_agg(to_jsonb(p)) FROM platform_catalog p),
 'columns',(SELECT jsonb_agg(jsonb_build_array(column_name,data_type,character_maximum_length) ORDER BY column_name) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='accounts'))::text`).Scan(&s)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func assertV3(t *testing.T, c *pgx.Conn) {
	t.Helper()
	var version int
	var definition string
	if e := c.QueryRow(context.Background(), "SELECT version,definition::text FROM platform_catalog").Scan(&version, &definition); e != nil {
		t.Fatal(e)
	}
	if version != 3 {
		t.Fatalf("version %d", version)
	}
	var canonicalType string
	if e := c.QueryRow(context.Background(), "SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='accounts' AND column_name='compat_key_canonical'").Scan(&canonicalType); e != nil || canonicalType != "text" {
		t.Fatal(canonicalType, e)
	}
	var unique, pair, old bool
	if e := c.QueryRow(context.Background(), `SELECT
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='accounts'::regclass AND contype='u' AND pg_get_constraintdef(oid)='UNIQUE (tenant_id, compat_key_canonical)'),
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='accounts'::regclass AND contype='c' AND pg_get_constraintdef(oid)='CHECK (((compat_key IS NULL) = (compat_key_canonical IS NULL)))'),
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='accounts'::regclass AND conname='accounts_tenant_id_compat_key_key')`).Scan(&unique, &pair, &old); e != nil || !unique || !pair || old {
		t.Fatal(unique, pair, old, e)
	}
	want, _ := json.Marshal(catalog)
	var eq bool
	if e := c.QueryRow(context.Background(), "SELECT $1::jsonb=$2::jsonb", definition, string(want)).Scan(&eq); e != nil || !eq {
		t.Fatalf("definition mismatch %v", e)
	}
	var n int
	if e := c.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='accounts' AND column_name IN ('compat_text','compat_key') AND data_type='character varying' AND character_maximum_length=80`).Scan(&n); e != nil || n != 2 {
		t.Fatalf("physical bounds %d %v", n, e)
	}
}
func TestCatalogFreshV3(t *testing.T) {
	url, c := migrationDB(t)
	s, e := Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	assertV3(t, c)
	s, e = Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
}
func TestCatalogUpgradeV1(t *testing.T) {
	url, c := migrationDB(t)
	seedLegacy(t, c)
	before := snapshot(t, c)
	s, e := Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	assertV3(t, c)
	var a, b map[string]any
	json.Unmarshal([]byte(before), &a)
	json.Unmarshal([]byte(snapshot(t, c)), &b)
	// Columns added by later migrations are expected; compare the original data.
	for _, table := range []string{"accounts", "contacts"} {
		original := a[table].([]any)[0].(map[string]any)
		for _, row := range b[table].([]any) {
			for k := range row.(map[string]any) {
				if _, ok := original[k]; !ok {
					delete(row.(map[string]any), k)
				}
			}
		}
	}
	for _, key := range []string{"accounts", "contacts", "tenants"} {
		x, _ := json.Marshal(a[key])
		y, _ := json.Marshal(b[key])
		if string(x) != string(y) {
			t.Fatal("data changed", key)
		}
	}
	if _, e = c.Exec(context.Background(), "UPDATE accounts SET compat_text=$1 WHERE tenant_id='first'", strings.Repeat("x", 81)); e == nil {
		t.Fatal("SQL accepted oversized value")
	}
	s, e = Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
}
func TestCatalogUpgradeOversizedRollback(t *testing.T) {
	for _, column := range []string{"compat_text", "compat_key"} {
		t.Run(column, func(t *testing.T) {
			url, c := migrationDB(t)
			seedLegacy(t, c)
			sqlDo(t, c, "UPDATE accounts SET "+column+"=$1 WHERE tenant_id='other'", strings.Repeat("x", 81))
			before := snapshot(t, c)
			s, e := Open(context.Background(), url)
			if e == nil {
				s.Close()
				t.Fatal("oversized upgrade accepted")
			}
			if !strings.Contains(e.Error(), "operator remediation") {
				t.Fatal(e)
			}
			if snapshot(t, c) != before {
				t.Fatal("failed upgrade modified database")
			}
		})
	}
}
func TestCatalogRejectsUnknown(t *testing.T) {
	for _, change := range []string{"UPDATE platform_catalog SET version=99", "UPDATE platform_catalog SET definition='{}'", "DELETE FROM platform_catalog"} {
		t.Run(change, func(t *testing.T) {
			url, c := migrationDB(t)
			seedLegacy(t, c)
			sqlDo(t, c, change)
			before := snapshot(t, c)
			s, e := Open(context.Background(), url)
			if e == nil {
				s.Close()
				t.Fatal("unknown catalog accepted")
			}
			if snapshot(t, c) != before {
				t.Fatal("rejection modified database")
			}
		})
	}
}
func TestCatalogV3PhysicalMismatch(t *testing.T) {
	url, c := migrationDB(t)
	s, e := Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	sqlDo(t, c, "ALTER TABLE accounts ALTER COLUMN compat_key TYPE varchar(255)")
	before := snapshot(t, c)
	s, e = Open(context.Background(), url)
	if e == nil {
		s.Close()
		t.Fatal("schema drift accepted")
	}
	if snapshot(t, c) != before {
		t.Fatal("schema drift repaired silently")
	}
}
func TestCatalogConcurrentOpen(t *testing.T) {
	url, c := migrationDB(t)
	seedLegacy(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := make(chan error, 2)
	for range 2 {
		go func() {
			s, e := Open(ctx, url)
			if s != nil {
				s.Close()
			}
			results <- e
		}()
	}
	for range 2 {
		if e := <-results; e != nil {
			t.Fatal(e)
		}
	}
	assertV3(t, c)
}

func seedV2(t *testing.T, c *pgx.Conn) {
	t.Helper()
	sqlDo(t, c, schema)
	definition, _ := json.Marshal(catalog)
	sqlDo(t, c, "INSERT INTO platform_catalog VALUES(true,2,$1)", string(definition))
	sqlDo(t, c, "ALTER TABLE accounts ALTER COLUMN compat_key TYPE varchar(80), ALTER COLUMN compat_text TYPE varchar(80)")
	sqlDo(t, c, "INSERT INTO tenants VALUES('first'),('other')")
}
func TestCatalogUpgradeV2ToV3(t *testing.T) {
	url, c := migrationDB(t)
	seedV2(t, c)
	sqlDo(t, c, "INSERT INTO accounts VALUES('first','001000000000001AAA','A',NULL,' AbC '),('other','001000000000002AAA','B',NULL,NULL)")
	s, e := Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	assertV3(t, c)
	var key, canonical string
	if e = c.QueryRow(context.Background(), "SELECT compat_key,compat_key_canonical FROM accounts WHERE tenant_id='first'").Scan(&key, &canonical); e != nil || key != " AbC " || canonical != "abc" {
		t.Fatal(key, canonical, e)
	}
	var null bool
	if e = c.QueryRow(context.Background(), "SELECT compat_key_canonical IS NULL FROM accounts WHERE tenant_id='other'").Scan(&null); e != nil || !null {
		t.Fatal(null, e)
	}
	before := snapshot(t, c)
	s, e = Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	if snapshot(t, c) != before {
		t.Fatal("repeat Open changed data")
	}
}
func TestCatalogV3RefusesCollisions(t *testing.T) {
	for _, sameTenant := range []bool{true, false} {
		t.Run(fmt.Sprint(sameTenant), func(t *testing.T) {
			url, c := migrationDB(t)
			seedV2(t, c)
			tenant := "other"
			if sameTenant {
				tenant = "first"
			}
			sqlDo(t, c, "INSERT INTO accounts VALUES('first','001000000000001AAA','A',NULL,'AbC'),($1,'001000000000002AAA','B',NULL,'abc')", tenant)
			before := snapshot(t, c)
			s, e := Open(context.Background(), url)
			if sameTenant {
				if e == nil {
					s.Close()
					t.Fatal("collision accepted")
				}
				if strings.Contains(e.Error(), "AbC") || strings.Contains(e.Error(), "abc") {
					t.Fatal("values leaked")
				}
				if snapshot(t, c) != before {
					t.Fatal("failed migration changed database")
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				s.Close()
				assertV3(t, c)
			}
		})
	}
}
func TestCatalogV3RefusesWhitespaceOnlyKey(t *testing.T) {
	url, c := migrationDB(t)
	seedV2(t, c)
	sqlDo(t, c, "INSERT INTO accounts VALUES('first','001000000000001AAA','A',NULL,'   ')")
	before := snapshot(t, c)
	s, e := Open(context.Background(), url)
	if e == nil {
		s.Close()
		t.Fatal("blank accepted")
	}
	if snapshot(t, c) != before {
		t.Fatal("failed migration changed database")
	}
}
func TestCatalogV3FoldedLengthBeyond80(t *testing.T) {
	url, c := migrationDB(t)
	seedV2(t, c)
	sqlDo(t, c, "INSERT INTO accounts VALUES('first','001000000000001AAA','A',NULL,$1)", strings.Repeat("ß", 80))
	s, e := Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	var key, canonical string
	if e = c.QueryRow(context.Background(), "SELECT compat_key,compat_key_canonical FROM accounts").Scan(&key, &canonical); e != nil || key != strings.Repeat("ß", 80) || canonical != strings.Repeat("s", 160) {
		t.Fatal("folded length", e)
	}
}

func TestOAuthTablesUpgradeFromV3(t *testing.T) {
	u, c := migrationDB(t)
	ctx := context.Background()
	s, e := Open(ctx, u)
	if e != nil {
		t.Fatal(e)
	}
	p := Principal{Tenant: "upgrade", User: "admin", Role: "admin"}
	if e = s.Provision(ctx, "upgrade-token-123456", p); e != nil {
		t.Fatal(e)
	}
	id, e := s.Create(ctx, p, "Account", map[string]any{"Name": "Survivor"})
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	sqlDo(t, c, "DROP TABLE oauth_access_tokens,oauth_refresh_tokens,oauth_codes,oauth_requests,oauth_jwt_ids,oauth_app_jwt_users,oauth_apps,users CASCADE")
	for range 2 {
		s, e = Open(ctx, u)
		if e != nil {
			t.Fatal(e)
		}
		for _, name := range []string{"users", "oauth_apps", "oauth_app_jwt_users", "oauth_requests", "oauth_codes", "oauth_refresh_tokens", "oauth_access_tokens", "oauth_jwt_ids"} {
			var table *string
			if e = c.QueryRow(ctx, "SELECT to_regclass($1)::text", name).Scan(&table); e != nil || table == nil {
				t.Fatal(name, e)
			}
		}
		var version int
		c.QueryRow(ctx, "SELECT version FROM platform_catalog").Scan(&version)
		if version != 3 {
			t.Fatal(version)
		}
		if _, e = s.Read(ctx, p, "Account", id); e != nil {
			t.Fatal(e)
		}
		if _, e = s.Authenticate(ctx, "upgrade-token-123456"); e != nil {
			t.Fatal(e)
		}
		s.Close()
	}
}
