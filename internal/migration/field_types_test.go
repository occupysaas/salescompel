package migration

import (
	"context"
	"net/url"
	"os"
	"salescompel/internal/metadata"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func catalogRows(t *testing.T, p *pgxpool.Pool, tenant string) []string {
	t.Helper()
	rows, e := p.Query(context.Background(), `SELECT o.api_name||'.'||f.api_name||'|'||f.physical_column||'|'||f.ordinal||'|'||f.definition::text FROM sc_catalog.fields f JOIN sc_catalog.objects o ON o.tenant_id=f.tenant_id AND o.id=f.object_id WHERE f.tenant_id=$1 AND o.standard ORDER BY o.api_name,f.ordinal`, tenant)
	if e != nil {
		t.Fatal(e)
	}
	out, e := pgx.CollectRows(rows, pgx.RowTo[string])
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func TestCatalogSeedParity(t *testing.T) {
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
	p.Exec(ctx, "INSERT INTO tenants(id) VALUES('new')")
	tx, _ := p.Begin(ctx)
	if e := metadata.SeedTenant(ctx, tx, "new"); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	old, fresh := catalogRows(t, p, "old"), catalogRows(t, p, "new")
	if len(old) < 40 || len(old) != len(fresh) {
		t.Fatalf("catalog sizes %d vs %d", len(old), len(fresh))
	}
	for i := range old {
		if old[i] != fresh[i] {
			t.Fatalf("migrated and seeded differ:\n%s\n%s", old[i], fresh[i])
		}
	}
	for _, tenant := range []string{"old", "new"} {
		var code string
		if e := p.QueryRow(ctx, "SELECT currency_iso_code FROM sc_catalog.settings WHERE tenant_id=$1", tenant).Scan(&code); e != nil || code != "USD" {
			t.Fatalf("%s currency %q %v", tenant, code, e)
		}
	}
}

func TestMigrationFieldTypesUpgrade(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := up(ctx, p, manifest()[:1]); e != nil {
		t.Fatal(e)
	}
	p.Exec(ctx, "INSERT INTO tenants(id) VALUES('t1'),('t2')")
	if e := up(ctx, p, manifest()[:3]); e != nil {
		t.Fatal(e)
	}
	// A Plan 27 custom object with a Text field, in the version-1 definition shape.
	for _, sql := range []string{
		"SET ROLE salescompel_owner",
		"SELECT set_config('salescompel.tenant_id','t1',false)",
		"CREATE TABLE sc_records.o_0123456789abcdef0123456789abcdef(tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,id varchar(18) NOT NULL,name varchar(80) NOT NULL,f_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa varchar(80),PRIMARY KEY(tenant_id,id))",
		`INSERT INTO sc_catalog.objects VALUES('0123456789abcdef0123456789abcdef','t1','Project__c','Project','Projects',false,'sc_records','o_0123456789abcdef0123456789abcdef','a00')`,
		`INSERT INTO sc_catalog.fields VALUES('p-id','t1','0123456789abcdef0123456789abcdef','Id','id','{"name":"Id","label":"Id","type":"id","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":0,"referenceTo":[]}',0)`,
		`INSERT INTO sc_catalog.fields VALUES('p-name','t1','0123456789abcdef0123456789abcdef','Name','name','{"name":"Name","label":"Name","type":"string","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":80,"referenceTo":[]}',1)`,
		`INSERT INTO sc_catalog.fields VALUES('p-notes','t1','0123456789abcdef0123456789abcdef','Notes__c','f_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','{"name":"Notes__c","label":"Notes","type":"string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":80,"referenceTo":[]}',2)`,
		"UPDATE sc_catalog.revisions SET revision=2 WHERE tenant_id='t1'",
		"INSERT INTO sc_records.o_0123456789abcdef0123456789abcdef VALUES('t1','a00000000000001AAA','Alpha','note')",
		"INSERT INTO accounts(tenant_id,id,name,compat_key,compat_key_canonical) VALUES('t1','001000000000001AAA','Acme','K1','k1')",
		"INSERT INTO contacts(tenant_id,id,first_name,last_name,account_id) VALUES('t1','003000000000001AAA','Ada','Lovelace','001000000000001AAA'),('t1','003000000000002AAA',NULL,'Solo',NULL)",
		"RESET ROLE",
	} {
		if _, e := p.Exec(ctx, sql); e != nil {
			t.Fatalf("%s: %v", sql, e)
		}
	}
	snapshot := func() string {
		var out string
		p.QueryRow(ctx, `SELECT (SELECT jsonb_agg(jsonb_build_array(id,name,compat_key,compat_key_canonical) ORDER BY id) FROM accounts)::text||
			(SELECT jsonb_agg(jsonb_build_array(id,first_name,last_name,account_id) ORDER BY id) FROM contacts)::text||
			(SELECT jsonb_agg(jsonb_build_array(id,name,f_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa) ORDER BY id) FROM sc_records.o_0123456789abcdef0123456789abcdef)::text||
			(SELECT jsonb_agg(jsonb_build_array(tenant_id,id,api_name,physical_column,ordinal) ORDER BY id) FROM sc_catalog.fields WHERE ordinal<4)::text||
			(SELECT jsonb_agg(jsonb_build_array(tenant_id,revision) ORDER BY tenant_id) FROM sc_catalog.revisions)::text`).Scan(&out)
		return out
	}
	before := snapshot()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if after := snapshot(); after != before {
		t.Fatalf("upgrade changed existing state:\n%s\n%s", before, after)
	}
	var names []string
	rows, _ := p.Query(ctx, "SELECT coalesce(name,'<null>') FROM contacts ORDER BY id")
	names, _ = pgx.CollectRows(rows, pgx.RowTo[string])
	if len(names) != 2 || names[0] != "Ada Lovelace" || names[1] != "Solo" {
		t.Fatalf("Contact.Name = %v", names)
	}
	var nulls bool
	p.QueryRow(ctx, "SELECT phone IS NULL AND industry IS NULL AND annual_revenue IS NULL AND billing_city IS NULL FROM accounts").Scan(&nulls)
	if !nulls {
		t.Fatal("new standard columns not null")
	}
	var raw []byte
	if e := p.QueryRow(ctx, "SELECT definition FROM sc_catalog.fields WHERE id='p-notes'").Scan(&raw); e != nil {
		t.Fatal(e)
	}
	f, e := metadata.DecodeDefinition(raw)
	if e != nil || f.Options.Kind != "Text" || f.Length != 80 || f.Label != "Notes" {
		t.Fatalf("custom definition after upgrade: %v %#v", e, f)
	}
	if e := p.QueryRow(ctx, "SELECT definition FROM sc_catalog.fields WHERE tenant_id='t1' AND api_name='AccountId'").Scan(&raw); e != nil {
		t.Fatal(e)
	}
	f, e = metadata.DecodeDefinition(raw)
	if e != nil || f.Options.Kind != "Lookup" || f.Options.DeleteBehavior != "Restrict" || f.RelationshipName == nil || *f.RelationshipName != "Account" {
		t.Fatalf("AccountId after upgrade: %v %#v", e, f)
	}
	tx, _ := p.Begin(ctx)
	defer tx.Rollback(ctx)
	if _, e := metadata.Load(ctx, tx, "t1"); e != nil {
		t.Fatalf("load after upgrade: %v", e)
	}
}

func TestFieldTypesRLS(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := up(ctx, p, manifest()[:1]); e != nil {
		t.Fatal(e)
	}
	p.Exec(ctx, "INSERT INTO tenants(id) VALUES('t1'),('t2')")
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Exec(ctx, "INSERT INTO sc_catalog.auto_numbers SELECT f.tenant_id,f.id,1 FROM sc_catalog.fields f JOIN sc_catalog.objects o ON o.tenant_id=f.tenant_id AND o.id=f.object_id WHERE o.api_name='Account' AND f.api_name='Name'"); e != nil {
		t.Fatal(e)
	}
	records := os.Getenv("TEST_RECORDS_URL")
	if records == "" {
		t.Fatal("TEST_RECORDS_URL required")
	}
	u, _ := url.Parse(records)
	u.Path = "/" + p.Config().ConnConfig.Database
	r, e := pgxpool.New(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	var n int
	for _, table := range []string{"settings", "auto_numbers"} {
		if e := r.QueryRow(ctx, "SELECT count(*) FROM sc_catalog."+table).Scan(&n); e != nil || n != 0 {
			t.Fatalf("%s visible without tenant context: %d %v", table, n, e)
		}
	}
	tx, _ := r.Begin(ctx)
	defer tx.Rollback(ctx)
	tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id','t1',true)")
	if e := tx.QueryRow(ctx, "SELECT count(*) FROM sc_catalog.auto_numbers").Scan(&n); e != nil || n != 1 {
		t.Fatalf("t1 counters %d %v", n, e)
	}
	tag, e := tx.Exec(ctx, "UPDATE sc_catalog.auto_numbers SET next_value=next_value+1 WHERE tenant_id='t2'")
	if e != nil || tag.RowsAffected() != 0 {
		t.Fatalf("cross-tenant counter update %v %v", tag, e)
	}
	if _, e := tx.Exec(ctx, "UPDATE sc_catalog.settings SET currency_iso_code='EUR'"); e == nil {
		t.Fatal("records role may change settings")
	}
}
