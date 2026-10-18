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
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL required")
		}
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	c, e := pgx.Connect(ctx, databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("migration_%d", time.Now().UnixNano())
	ident := pgx.Identifier{name}.Sanitize()
	if _, e = c.Exec(ctx, "CREATE SCHEMA "+ident); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Exec(ctx, "DROP SCHEMA "+ident+" CASCADE"); c.Close(ctx) })
	parsed, e := url.Parse(databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	query := parsed.Query()
	query.Set("search_path", name)
	parsed.RawQuery = query.Encode()
	isolated := parsed.String()
	db, e := pgx.Connect(ctx, isolated)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(ctx) })
	return isolated, db
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
func assertV2(t *testing.T, c *pgx.Conn) {
	t.Helper()
	var version int
	var definition string
	if e := c.QueryRow(context.Background(), "SELECT version,definition::text FROM platform_catalog").Scan(&version, &definition); e != nil {
		t.Fatal(e)
	}
	if version != 2 {
		t.Fatalf("version %d", version)
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
func TestCatalogFreshV2(t *testing.T) {
	url, c := migrationDB(t)
	s, e := Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	assertV2(t, c)
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
	assertV2(t, c)
	var a, b map[string]any
	json.Unmarshal([]byte(before), &a)
	json.Unmarshal([]byte(snapshot(t, c)), &b)
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
func TestCatalogV2PhysicalMismatch(t *testing.T) {
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
	assertV2(t, c)
}
