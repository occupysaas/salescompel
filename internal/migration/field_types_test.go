package migration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Id and Name definitions in the version-1 shape, shared by child-name seeds.
const idDef = `{"v":2,"field":{"name":"Id","label":"Id","type":"id","soapType":"tns:ID","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":0,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Id"}}`
const nameDef = `{"v":2,"field":{"name":"Name","label":"Name","type":"string","soapType":"xsd:string","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":80,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":true,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}`

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

func TestMigrationChildRelationshipBackfill(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := up(ctx, p, manifest()[:1]); e != nil {
		t.Fatal(e)
	}
	p.Exec(ctx, "INSERT INTO tenants(id) VALUES('t1')")
	if e := up(ctx, p, manifest()[:4]); e != nil {
		t.Fatal(e)
	}
	// Two custom objects share the plural label; each holds one lookup to Account.
	rel := "Acct__r"
	lookupDef := func(column string) string {
		f := metadata.Complete(metadata.Field{
			Name: "Acct__c", Label: "Acct", Column: column, Nillable: true, Createable: true, Updateable: true, Length: 18,
			ReferenceTo: []string{"Account"}, RelationshipName: &rel,
			Options: metadata.Options{Kind: fieldtype.Lookup, DeleteBehavior: "SetNull"},
		})
		raw, e := metadata.EncodeDefinition(f)
		if e != nil {
			t.Fatal(e)
		}
		return string(raw)
	}
	stmts := []struct {
		sql  string
		args []any
	}{
		{"SET ROLE salescompel_owner", nil},
		{"SELECT set_config('salescompel.tenant_id','t1',false)", nil},
		{"CREATE TABLE sc_records.o_alpha000000000000000000000001(tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,id varchar(18) NOT NULL,name varchar(80) NOT NULL,f_alpha000000000000000000000001 varchar(18),PRIMARY KEY(tenant_id,id))", nil},
		{"CREATE TABLE sc_records.o_beta0000000000000000000000001(tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,id varchar(18) NOT NULL,name varchar(80) NOT NULL,f_beta0000000000000000000000001 varchar(18),PRIMARY KEY(tenant_id,id))", nil},
		{`INSERT INTO sc_catalog.objects VALUES('alpha00000000000000000000000001','t1','Alpha__c','Alpha','Widgets',false,'sc_records','o_alpha000000000000000000000001','a00')`, nil},
		{`INSERT INTO sc_catalog.objects VALUES('beta00000000000000000000000001','t1','Beta__c','Beta','Widgets',false,'sc_records','o_beta0000000000000000000000001','b00')`, nil},
		{`INSERT INTO sc_catalog.fields VALUES('alpha-id','t1','alpha00000000000000000000000001','Id','id',$1::jsonb,0)`, []any{idDef}},
		{`INSERT INTO sc_catalog.fields VALUES('alpha-name','t1','alpha00000000000000000000000001','Name','name',$1::jsonb,1)`, []any{nameDef}},
		{`INSERT INTO sc_catalog.fields VALUES('alpha-acct','t1','alpha00000000000000000000000001','Acct__c','f_alpha000000000000000000000001',$1::jsonb,2)`, []any{lookupDef("f_alpha000000000000000000000001")}},
		{`INSERT INTO sc_catalog.fields VALUES('beta-id','t1','beta00000000000000000000000001','Id','id',$1::jsonb,0)`, []any{idDef}},
		{`INSERT INTO sc_catalog.fields VALUES('beta-name','t1','beta00000000000000000000000001','Name','name',$1::jsonb,1)`, []any{nameDef}},
		{`INSERT INTO sc_catalog.fields VALUES('beta-acct','t1','beta00000000000000000000000001','Acct__c','f_beta0000000000000000000000001',$1::jsonb,2)`, []any{lookupDef("f_beta0000000000000000000000001")}},
		{"UPDATE sc_catalog.revisions SET revision=2 WHERE tenant_id='t1'", nil},
		{"INSERT INTO sc_records.o_alpha000000000000000000000001 VALUES('t1','a00000000000001AAA','Alpha',NULL)", nil},
		{"INSERT INTO sc_records.o_beta0000000000000000000000001 VALUES('t1','a00000000000002AAA','Beta',NULL)", nil},
		{"INSERT INTO accounts(tenant_id,id,name) VALUES('t1','001000000000001AAA','Acme')", nil},
		{"INSERT INTO contacts(tenant_id,id,last_name,account_id) VALUES('t1','003000000000001AAA','L','001000000000001AAA')", nil},
		{"RESET ROLE", nil},
	}
	for _, st := range stmts {
		if _, e := p.Exec(ctx, st.sql, st.args...); e != nil {
			t.Fatalf("%s: %v", st.sql, e)
		}
	}
	snapshot := func() string {
		var out string
		p.QueryRow(ctx, `SELECT (SELECT jsonb_agg(jsonb_build_array(id,name) ORDER BY id) FROM accounts)::text||
			(SELECT jsonb_agg(jsonb_build_array(id,last_name,account_id) ORDER BY id) FROM contacts)::text||
			(SELECT jsonb_agg(jsonb_build_array(tenant_id,id,api_name,physical_column,ordinal) ORDER BY id) FROM sc_catalog.fields WHERE api_name IN ('Id','Name','Acct__c','AccountId'))::text||
			(SELECT jsonb_agg(jsonb_build_array(tenant_id,id,api_name,plural_label) ORDER BY api_name) FROM sc_catalog.objects WHERE NOT standard)::text`).Scan(&out)
		return out
	}
	before := snapshot()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if after := snapshot(); after != before {
		t.Fatalf("0005 changed values or IDs:\n%s\n%s", before, after)
	}
	names := map[string]string{}
	rows, e := p.Query(ctx, `SELECT o.api_name||'.'||f.api_name, f.definition->'options'->>'childRelationshipName' FROM sc_catalog.fields f JOIN sc_catalog.objects o ON o.tenant_id=f.tenant_id AND o.id=f.object_id WHERE f.tenant_id='t1' AND f.definition->'options'->>'kind' IN ('Lookup','MasterDetail') ORDER BY o.api_name,f.ordinal`)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var key, nm string
		rows.Scan(&key, &nm)
		names[key] = nm
	}
	rows.Close()
	if names["Alpha__c.Acct__c"] != "Widgets__r" {
		t.Fatalf("Alpha name %q", names["Alpha__c.Acct__c"])
	}
	if names["Beta__c.Acct__c"] != "Widgets__r2" {
		t.Fatalf("Beta name %q", names["Beta__c.Acct__c"])
	}
	if names["Contact.AccountId"] != "Contacts" {
		t.Fatalf("Contact.AccountId name %q", names["Contact.AccountId"])
	}
	var raw []byte
	p.QueryRow(ctx, "SELECT definition FROM sc_catalog.fields WHERE id='alpha-acct'").Scan(&raw)
	f, e := metadata.DecodeDefinition(raw)
	if e != nil || f.Options.ChildRelationshipName != "Widgets__r" || f.Options.Kind != fieldtype.Lookup || f.Name != "Acct__c" {
		t.Fatalf("alpha definition after 0005: %v %#v", e, f)
	}
}

func TestMigrationChildRelationshipMinting(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := up(ctx, p, manifest()[:1]); e != nil {
		t.Fatal(e)
	}
	p.Exec(ctx, "INSERT INTO tenants(id) VALUES('t1'),('t2')")
	if e := up(ctx, p, manifest()[:4]); e != nil {
		t.Fatal(e)
	}
	rel := "Acct__r"
	lookupDef := func(field, column, preName string) string {
		opts := metadata.Options{Kind: fieldtype.Lookup, DeleteBehavior: "SetNull"}
		if preName != "" {
			opts.ChildRelationshipName = preName
		}
		f := metadata.Complete(metadata.Field{
			Name: field, Label: field, Column: column, Nillable: true, Createable: true, Updateable: true, Length: 18,
			ReferenceTo: []string{"Account"}, RelationshipName: &rel,
			Options: opts,
		})
		raw, e := metadata.EncodeDefinition(f)
		if e != nil {
			t.Fatal(e)
		}
		return string(raw)
	}
	// Literal wants pin the documented mint; MintChildRelationshipName must
	// agree with each of them. The last two rows share a plural across tenants.
	seeds := []struct {
		tenant, objID, objAPI, plural, table, fldID, fldAPI, column, preName, want string
	}{
		{"t1", "typed000000000000000000000001", "Typed__c", "Typed items", "o_typed", "typed-lk", "Lk__c", "f_typed_lk", "", "Typeditems__r"},
		{"t1", "cafe000000000000000000000001", "Cafe__c", "Cafés", "o_cafe", "cafe-lk", "Lk__c", "f_cafe_lk", "", "Cafs__r"},
		{"t1", "digits000000000000000000000001", "Digits__c", "123 Widgets", "o_digits", "digits-lk", "DigitLk__c", "f_digits_lk", "", "RelDigitLkc"},
		{"t1", "greek000000000000000000000001", "Greek__c", "παιδιά", "o_greek", "greek-lk", "GreekLk__c", "f_greek_lk", "", "RelGreekLkc"},
		{"t1", "bang000000000000000000000001", "Bang__c", "!!!", "o_bang", "bang-lk", "BangLk__c", "f_bang_lk", "", "RelBangLkc"},
		{"t1", "long0000000000000000000000001", "Long__c", strings.Repeat("A", 41), "o_long", "long-lk", "LongLk__c", "f_long_lk", "", "RelLongLkc"},
		{"t1", "named000000000000000000000001", "Named__c", "Nameds", "o_named", "named-lk", "Lk__c", "f_named_lk", "KeepMe", "KeepMe"},
		{"t1", "wga00000000000000000000000001", "WidgetsA__c", "Widgets", "o_wga", "wga-lk", "Lk__c", "f_wga_lk", "", "Widgets__r"},
		{"t2", "wgb00000000000000000000000001", "WidgetsB__c", "Widgets", "o_wgb", "wgb-lk", "Lk__c", "f_wgb_lk", "", "Widgets__r"},
	}
	before := map[string]string{}
	for i, s := range seeds {
		stmts := []struct {
			sql  string
			args []any
		}{
			{"SET ROLE salescompel_owner", nil},
			{"SELECT set_config('salescompel.tenant_id',$1,false)", []any{s.tenant}},
			{fmt.Sprintf("CREATE TABLE sc_records.%s(tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,id varchar(18) NOT NULL,name varchar(80) NOT NULL,%s varchar(18),PRIMARY KEY(tenant_id,id))", s.table, s.column), nil},
			{`INSERT INTO sc_catalog.objects VALUES($1,$2,$3,$4,$5,false,'sc_records',$6,$7)`, []any{s.objID, s.tenant, s.objAPI, s.objAPI, s.plural, s.table, fmt.Sprintf("p%02d", i)}},
			{`INSERT INTO sc_catalog.fields VALUES($1,$2,$3,'Id','id',$4::jsonb,0)`, []any{s.objID + "-id", s.tenant, s.objID, idDef}},
			{`INSERT INTO sc_catalog.fields VALUES($1,$2,$3,'Name','name',$4::jsonb,1)`, []any{s.objID + "-name", s.tenant, s.objID, nameDef}},
			{`INSERT INTO sc_catalog.fields VALUES($1,$2,$3,$4,$5,$6::jsonb,2)`, []any{s.fldID, s.tenant, s.objID, s.fldAPI, s.column, lookupDef(s.fldAPI, s.column, s.preName)}},
			{"RESET ROLE", nil},
		}
		for _, st := range stmts {
			if _, e := p.Exec(ctx, st.sql, st.args...); e != nil {
				t.Fatalf("%s: %v", st.sql, e)
			}
		}
		var def string
		if e := p.QueryRow(ctx, "SELECT definition::text FROM sc_catalog.fields WHERE id=$1", s.fldID).Scan(&def); e != nil {
			t.Fatal(e)
		}
		before[s.fldID] = def
	}
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	for _, s := range seeds {
		if s.preName == "" {
			if m := metadata.MintChildRelationshipName(metadata.Object{Name: s.objAPI, PluralLabel: s.plural}, s.fldAPI); m != s.want {
				t.Errorf("MintChildRelationshipName(%q,%q) = %q, want %q", s.plural, s.fldAPI, m, s.want)
			}
		}
		var nm, behavior string
		var stripped, whole bool
		e := p.QueryRow(ctx, `SELECT definition->'options'->>'childRelationshipName', definition->'options'->>'deleteBehavior',
			(definition #- '{options,childRelationshipName}')=$2::jsonb, definition=$2::jsonb
			FROM sc_catalog.fields WHERE id=$1`, s.fldID, before[s.fldID]).Scan(&nm, &behavior, &stripped, &whole)
		if e != nil {
			t.Fatal(e)
		}
		if nm != s.want {
			t.Errorf("%s.%s minted %q, want %q", s.objAPI, s.fldAPI, nm, s.want)
		}
		if behavior != "SetNull" {
			t.Errorf("%s.%s delete behavior %q", s.objAPI, s.fldAPI, behavior)
		}
		if s.preName != "" {
			if !whole {
				t.Errorf("%s.%s pre-named definition changed", s.objAPI, s.fldAPI)
			}
		} else if !stripped {
			t.Errorf("%s.%s definition changed beyond childRelationshipName", s.objAPI, s.fldAPI)
		}
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
