package migration

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestQueryLocatorsMigration(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}

	rows, e := p.Query(ctx, `SELECT column_name,data_type,is_nullable
		FROM information_schema.columns
		WHERE table_schema='sc_query' AND table_name='locators' ORDER BY ordinal_position`)
	if e != nil {
		t.Fatal(e)
	}
	type spec struct{ typ, nullable string }
	want := []struct {
		name string
		spec spec
	}{
		{"tenant_id", spec{"text", "NO"}},
		{"id", spec{"text", "NO"}},
		{"user_id", spec{"text", "NO"}},
		{"query_sha256", spec{"text", "NO"}},
		{"plan", spec{"jsonb", "NO"}},
		{"ids", spec{"ARRAY", "NO"}},
		{"created_at", spec{"timestamp with time zone", "NO"}},
		{"expires_at", spec{"timestamp with time zone", "NO"}},
	}
	var got []string
	i := 0
	for rows.Next() {
		var name string
		var s spec
		if e = rows.Scan(&name, &s.typ, &s.nullable); e != nil {
			t.Fatal(e)
		}
		got = append(got, name+" "+s.typ+" "+s.nullable)
		if i < len(want) {
			if name != want[i].name || s != want[i].spec {
				t.Fatalf("column %d = %s %s %s, want %s %s %s", i, name, s.typ, s.nullable, want[i].name, want[i].spec.typ, want[i].spec.nullable)
			}
		}
		i++
	}
	rows.Close()
	if i != len(want) {
		t.Fatalf("columns: %v", got)
	}

	var pk string
	if e = p.QueryRow(ctx, `SELECT string_agg(a.attname,',' ORDER BY a.attnum)
		FROM pg_index x JOIN pg_class c ON c.oid=x.indrelid
		JOIN pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=ANY(x.indkey)
		WHERE n.nspname='sc_query' AND c.relname='locators' AND x.indisprimary`).Scan(&pk); e != nil {
		t.Fatal(e)
	}
	if pk != "tenant_id,id" {
		t.Fatalf("primary key = %q", pk)
	}

	var rls, force bool
	var policies int
	if e = p.QueryRow(ctx, `SELECT relrowsecurity,relforcerowsecurity,
		(SELECT count(*) FROM pg_policy WHERE polrelid=c.oid)
		FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname='sc_query' AND c.relname='locators'`).Scan(&rls, &force, &policies); e != nil {
		t.Fatal(e)
	}
	if !rls || !force || policies != 1 {
		t.Fatalf("rls=%v force=%v policies=%d", rls, force, policies)
	}
	var policy string
	if e = p.QueryRow(ctx, `SELECT pg_get_expr(polqual,polrelid) FROM pg_policy
		WHERE polrelid='sc_query.locators'::regclass`).Scan(&policy); e != nil {
		t.Fatal(e)
	}
	if policy == "" {
		t.Fatal("empty locator policy")
	}

	var sel, ins, del, upd bool
	if e = p.QueryRow(ctx, `SELECT has_table_privilege('salescompel_records','sc_query.locators','SELECT'),
		has_table_privilege('salescompel_records','sc_query.locators','INSERT'),
		has_table_privilege('salescompel_records','sc_query.locators','DELETE'),
		has_table_privilege('salescompel_records','sc_query.locators','UPDATE')`).Scan(&sel, &ins, &del, &upd); e != nil {
		t.Fatal(e)
	}
	if !sel || !ins || !del || upd {
		t.Fatalf("records privileges select=%v insert=%v delete=%v update=%v", sel, ins, del, upd)
	}

	tenant := fmt.Sprintf("locator_%d", time.Now().UnixNano())
	if _, e = p.Exec(ctx, "INSERT INTO public.tenants(id) VALUES($1)", tenant); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { p.Exec(context.Background(), "DELETE FROM public.tenants WHERE id=$1", tenant) })
	if _, e = p.Exec(ctx, `INSERT INTO sc_query.locators(tenant_id,id,user_id,query_sha256,plan,ids,expires_at)
		VALUES($1,'01gAAAAAAAAAAAAAAA','alice','sha','{}'::jsonb,'{}'::text[],now()+interval '2 days')`, tenant); e != nil {
		t.Fatal(e)
	}

	count := func(tenantCtx string) int {
		t.Helper()
		tx, e := p.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if tenantCtx != "" {
			if _, e = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenantCtx); e != nil {
				t.Fatal(e)
			}
		}
		if _, e = tx.Exec(ctx, "SET LOCAL ROLE salescompel_records"); e != nil {
			t.Fatal(e)
		}
		var n int
		if e = tx.QueryRow(ctx, "SELECT count(*) FROM sc_query.locators WHERE id='01gAAAAAAAAAAAAAAA'").Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	if n := count(""); n != 0 {
		t.Fatalf("records without context saw %d locators", n)
	}
	if n := count(tenant); n != 1 {
		t.Fatalf("records in tenant saw %d locators", n)
	}
	if n := count("other-tenant"); n != 0 {
		t.Fatalf("records saw %d foreign locators", n)
	}
}
