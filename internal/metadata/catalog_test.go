package metadata_test

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"salescompel/internal/fieldtype"
	. "salescompel/internal/metadata"
	"salescompel/internal/migration"
	"strings"
	"testing"
	"time"
)

func catalogPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	root, e := pgxpool.New(ctx, os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("catalog_%d", time.Now().UnixNano())
	if _, e = root.Exec(ctx, "CREATE DATABASE "+name); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.Database = name
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { p.Close(); root.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); root.Close() })
	if e = migration.Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSchemaInputRules(t *testing.T) {
	for _, name := range []string{"__c", "X___c", "A__B__c", "A'__c", "é__c", strings.Repeat("A", 41) + "__c"} {
		if ValidAPIName(name) {
			t.Fatalf("accepted %q", name)
		}
	}
	for _, name := range []string{"X__c", strings.Repeat("A", 40) + "__c", "Project_Notes__c"} {
		if !ValidAPIName(name) {
			t.Fatalf("rejected %q", name)
		}
	}
}
func TestTenantCatalogNames(t *testing.T) {
	ctx := context.Background()
	p := catalogPool(t)
	tenant := fmt.Sprintf("catalog%d", time.Now().UnixNano())
	p.Exec(ctx, "INSERT INTO tenants(id) VALUES($1)", tenant)
	defer p.Exec(ctx, "DELETE FROM tenants WHERE id=$1", tenant)
	tx, _ := p.Begin(ctx)
	if e := SeedTenant(ctx, tx, tenant); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	obj, e := CreateObject(ctx, p, tenant, ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Exec(ctx, "DROP TABLE "+obj.TableSQL())
	f, e := CreateField(ctx, p, tenant, FieldInput{Object: "Project__c", Name: "Notes__c", Label: "Notes", Type: "Text", Length: 80})
	if e != nil || f.Length != 80 {
		t.Fatal(f, e)
	}
	if _, e = CreateObject(ctx, p, tenant, ObjectInput{Name: "project__c", Label: "Other", PluralLabel: "Others"}); e == nil {
		t.Fatal("duplicate accepted")
	}
	tx, _ = p.Begin(ctx)
	defer tx.Rollback(ctx)
	snapshot, e := Load(ctx, tx, tenant)
	if e != nil {
		t.Fatal(e)
	}
	project, e := snapshot.Object("pRoJeCt__c")
	if e != nil || project.Name != "Project__c" || len(project.Fields) != 3 {
		t.Fatal(project, e)
	}
	if snapshot.Revision != 3 {
		t.Fatal(snapshot.Revision)
	}
}

func seed(t *testing.T, p *pgxpool.Pool, tenant string) {
	t.Helper()
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "INSERT INTO tenants VALUES($1)", tenant); e != nil {
		t.Fatal(e)
	}
	if e = SeedTenant(ctx, tx, tenant); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
}
func TestSchemaAtomicDDL(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "atomic")
	o, e := CreateObject(ctx, p, "atomic", ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	if e != nil {
		t.Fatal(e)
	}
	count := func(sql string) int {
		var n int
		if e := p.QueryRow(ctx, sql).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	tables := count("SELECT count(*) FROM pg_tables WHERE schemaname='sc_records'")
	if _, e = CreateObject(ctx, p, "atomic", ObjectInput{Name: "Project__c", Label: "Duplicate", PluralLabel: "Duplicates"}); e == nil {
		t.Fatal("accepted duplicate")
	}
	if count("SELECT count(*) FROM pg_tables WHERE schemaname='sc_records'") != tables {
		t.Fatal("failed object creation leaked DDL")
	}
	columns := count("SELECT count(*) FROM information_schema.columns WHERE table_schema='sc_records' AND table_name='" + o.Table + "'")
	if _, e = p.Exec(ctx, `CREATE FUNCTION public.reject_field() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected field insert failure'; END $$;CREATE TRIGGER reject_field BEFORE INSERT ON sc_catalog.fields FOR EACH ROW EXECUTE FUNCTION public.reject_field()`); e != nil {
		t.Fatal(e)
	}
	if _, e = CreateField(ctx, p, "atomic", FieldInput{Object: "Project__c", Name: "Break__c", Label: "Break", Type: "Text", Length: 80}); e == nil {
		t.Fatal("ignored insert failure")
	}
	if count("SELECT count(*) FROM information_schema.columns WHERE table_schema='sc_records' AND table_name='"+o.Table+"'") != columns {
		t.Fatal("failed field creation leaked DDL")
	}
	var revision int
	if e = p.QueryRow(ctx, "SELECT revision FROM sc_catalog.revisions WHERE tenant_id='atomic'").Scan(&revision); e != nil || revision != 2 {
		t.Fatal(revision, e)
	}
}
func TestCatalogRejectsUnsupportedEffectiveMetadata(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "unknown")
	for _, change := range []string{`jsonb_set(jsonb_set(definition,'{field,type}','"picklist"'),'{options,kind}','"Formula"')`, `jsonb_set(definition,'{field,unknownSecurityMode}','"grant"')`, `jsonb_set(definition,'{options,unknownSecurityMode}','"grant"')`} {
		var original string
		p.QueryRow(ctx, "SELECT definition::text FROM sc_catalog.fields WHERE tenant_id='unknown' AND api_name='Name'").Scan(&original)
		if _, e := p.Exec(ctx, "UPDATE sc_catalog.fields SET definition="+change+" WHERE tenant_id='unknown' AND api_name='Name'"); e != nil {
			t.Fatal(e)
		}
		tx, _ := p.Begin(ctx)
		_, e := Load(ctx, tx, "unknown")
		tx.Rollback(ctx)
		if e == nil {
			t.Fatal("unsupported active metadata accepted: " + change)
		}
		p.Exec(ctx, "UPDATE sc_catalog.fields SET definition=$1::jsonb WHERE tenant_id='unknown' AND api_name='Name'", original)
		tx, _ = p.Begin(ctx)
		_, e = Load(ctx, tx, "unknown")
		tx.Rollback(ctx)
		if e != nil {
			t.Fatal("restored definition rejected: ", e)
		}
	}
}
func TestSchemaPrefixAllocation(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "prefix")
	a, e := CreateObject(ctx, p, "prefix", ObjectInput{Name: "One__c", Label: "One", PluralLabel: "Ones"})
	if e != nil || a.Prefix != "a00" {
		t.Fatal(a, e)
	}
	b, e := CreateObject(ctx, p, "prefix", ObjectInput{Name: "Two__c", Label: "Two", PluralLabel: "Twos"})
	if e != nil || b.Prefix != "a01" {
		t.Fatal(b, e)
	}
	tx, _ := p.Begin(ctx)
	snapshot, e := Load(ctx, tx, "prefix")
	tx.Rollback(ctx)
	if e != nil {
		t.Fatal(e)
	}
	o, e := snapshot.Object("One__c")
	if e != nil || o.Prefix != "a00" {
		t.Fatal(o, e)
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if _, e = p.Exec(ctx, `INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) SELECT md5(g::text),'prefix','Synthetic_'||g||'__c','Synthetic','Synthetic',false,'sc_records','o_'||md5(g::text),'a'||substr($1,g/62+1,1)||substr($1,g%62+1,1) FROM generate_series(2,3843)g`, alphabet); e != nil {
		t.Fatal(e)
	}
	if _, e = CreateObject(ctx, p, "prefix", ObjectInput{Name: "Extra__c", Label: "Extra", PluralLabel: "Extras"}); e == nil || !strings.Contains(e.Error(), "exhausted") {
		t.Fatal("prefix exhaustion not enforced", e)
	}
}
func TestCatalogConcurrentVisibility(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "visibility")
	o, e := CreateObject(ctx, p, "visibility", ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	if e != nil {
		t.Fatal(e)
	}
	owner, _ := p.Begin(ctx)
	defer owner.Rollback(ctx)
	if _, e = owner.Exec(ctx, "SELECT pg_advisory_xact_lock(739102)"); e != nil {
		t.Fatal(e)
	}
	if _, e = owner.Exec(ctx, "ALTER TABLE "+o.TableSQL()+" ADD COLUMN f_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa varchar(80)"); e != nil {
		t.Fatal(e)
	}
	waiting, _ := EncodeDefinition(Complete(Field{Name: "Waiting__c", Label: "Waiting", Nillable: true, Createable: true, Updateable: true, Length: 80, Options: Options{Kind: fieldtype.Text}}))
	if _, e = owner.Exec(ctx, `INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal)VALUES('waiting','visibility',$1,'Waiting__c','f_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',$2,2)`, o.ID, string(waiting)); e != nil {
		t.Fatal(e)
	}
	reader, _ := p.Begin(ctx)
	reader.Exec(ctx, "SET LOCAL lock_timeout='50ms'")
	if _, e = Load(ctx, reader, "visibility"); e == nil {
		t.Fatal("catalog read bypassed in-flight schema lock")
	}
	reader.Rollback(ctx)
	if _, e = owner.Exec(ctx, "UPDATE sc_catalog.revisions SET revision=3 WHERE tenant_id='visibility'"); e != nil {
		t.Fatal(e)
	}
	if e = owner.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	reader, _ = p.Begin(ctx)
	snapshot, e := Load(ctx, reader, "visibility")
	if e != nil {
		t.Fatal(e)
	}
	f, e := snapshot.Field("Project__c", "Waiting__c")
	if e != nil || f.Column != "f_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatal(f, e)
	}
	if _, e = reader.Exec(ctx, "SELECT "+f.Column+" FROM "+o.TableSQL()); e != nil {
		t.Fatal("visible field lacks physical storage", e)
	}
	reader.Rollback(ctx)
}
