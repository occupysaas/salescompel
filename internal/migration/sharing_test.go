package migration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"salescompel/internal/metadata"
	"salescompel/internal/fieldtype"
)

// A Plan 33 database with users and records upgrades to Plan 34 storage:
// owners backfilled to the default owner, Salesforce's new-org defaults
// (no object_settings rows), one Organization group per tenant holding every
// active user, and the derived closure (shr-13).
func TestMigrationSharingUpgrade(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := up(ctx, p, manifest()[:1]); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Exec(ctx, `INSERT INTO tenants VALUES('t1'),('t2'),('empty');
		INSERT INTO users(id,tenant_id,username,username_canonical,password_hash,role) VALUES
		('005000000000002AAA','t1','restricted@example.com','restricted@example.com','h','restricted'),
		('005000000000009AAA','t1','admin@example.com','admin@example.com','h','admin'),
		('005000000000003AAA','t2','solo@example.com','solo@example.com','h','restricted')`); e != nil {
		t.Fatal(e)
	}
	if e := up(ctx, p, manifest()[:7]); e != nil {
		t.Fatal(e)
	}
	id := metadata.Field{Name: "Id", Column: "id", Options: metadata.Options{Kind: fieldtype.Id}}
	name := metadata.Field{Name: "Name", Column: "name", Length: 80, Createable: true, Updateable: true, NameField: true, Options: metadata.Options{Kind: fieldtype.Text}}
	for _, tenant := range []string{"t1", "empty"} {
		table := "o_" + strings.Repeat(tenant[:1], 32)
		stmts := []string{
			"SELECT set_config('salescompel.tenant_id','" + tenant + "',false)",
			"CREATE TABLE sc_records." + table + "(tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,id varchar(18) NOT NULL,name varchar(80) NOT NULL,PRIMARY KEY(tenant_id,id))",
			"INSERT INTO sc_catalog.objects VALUES('" + table + "','" + tenant + "','Project__c','Project','Projects',false,'sc_records','" + table + "','a00')",
			"INSERT INTO accounts(tenant_id,id,name) VALUES('" + tenant + "','001000000000001AAA','Acme')",
			"INSERT INTO contacts(tenant_id,id,last_name,account_id) VALUES('" + tenant + "','003000000000001AAA','Ada','001000000000001AAA')",
			"INSERT INTO sc_records." + table + " VALUES('" + tenant + "','a00000000000001AAA','Alpha')",
		}
		for _, s := range stmts {
			if _, e := p.Exec(ctx, s); e != nil {
				t.Fatalf("%s: %v", s, e)
			}
		}
		for i, f := range []metadata.Field{id, name} {
			raw, _ := metadata.EncodeDefinition(metadata.Complete(f))
			if _, e := p.Exec(ctx, "INSERT INTO sc_catalog.fields VALUES($1,$2,$3,$4,$5,$6,$7)", fmt.Sprintf("%s-%d", tenant, i), tenant, table, f.Name, f.Column, string(raw), i); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	owners := func(tenant string) string {
		var out string
		if e := p.QueryRow(ctx, `SELECT concat_ws(',',(SELECT owner_id FROM accounts WHERE tenant_id=$1),(SELECT owner_id FROM contacts WHERE tenant_id=$1),
			(SELECT owner_id FROM sc_records.`+"o_"+strings.Repeat(tenant[:1], 32)+` WHERE tenant_id=$1))`, tenant).Scan(&out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	// The System Administrator wins over a lower ID (D34-31).
	if got := owners("t1"); got != "005000000000009AAA,005000000000009AAA,005000000000009AAA" {
		t.Fatalf("t1 owners %q", got)
	}
	// A tenant with no users keeps null owners (fail closed).
	if got := owners("empty"); got != "" {
		t.Fatalf("empty owners %q", got)
	}
	var settings, orgGroups, closure, principals int
	if e := p.QueryRow(ctx, `SELECT (SELECT count(*) FROM sc_sharing.object_settings),
		(SELECT count(*) FROM sc_setup.groups WHERE type='Organization'),
		(SELECT count(*) FROM sc_sharing.user_groups ug JOIN sc_setup.groups g ON g.tenant_id=ug.tenant_id AND g.id=ug.group_id AND g.type='Organization'),
		(SELECT count(*) FROM sc_sharing.user_principals)`).Scan(&settings, &orgGroups, &closure, &principals); e != nil {
		t.Fatal(e)
	}
	if settings != 0 || orgGroups != 3 || closure != 3 || principals != 6 {
		t.Fatalf("settings=%d org groups=%d closure=%d principals=%d", settings, orgGroups, closure, principals)
	}
	var access string
	if e := p.QueryRow(ctx, "SELECT default_account_access||','||default_contact_access FROM sc_setup.organization WHERE tenant_id='t1'").Scan(&access); e != nil || access != "Edit,ControlledByParent" {
		t.Fatalf("organization defaults %q %v", access, e)
	}
	var fields int
	if e := p.QueryRow(ctx, `SELECT count(*) FROM sc_catalog.fields f JOIN sc_catalog.objects o ON o.tenant_id=f.tenant_id AND o.id=f.object_id
		WHERE f.api_name='OwnerId' AND f.tenant_id='t1'`).Scan(&fields); e != nil || fields != 3 {
		t.Fatalf("OwnerId fields %d %v", fields, e)
	}
}

// Every sc_sharing table forces row security under the tenant policy and is
// owned by the owner group.
func TestMigrationSharingRLS(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	rows, e := p.Query(ctx, `SELECT c.relname,c.relrowsecurity,c.relforcerowsecurity,(SELECT count(*) FROM pg_policy WHERE polrelid=c.oid),pg_get_userbyid(c.relowner)
		FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='sc_sharing' AND c.relkind='r' ORDER BY 1`)
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
			t.Errorf("sc_sharing.%s rls=%v force=%v policies=%d owner=%s", name, rls, force, policies, owner)
		}
		tables = append(tables, name)
	}
	rows.Close()
	if strings.Join(tables, ",") != "object_settings,owner_rules,shares,user_groups,user_principals" {
		t.Fatalf("sc_sharing tables = %v", tables)
	}
}
