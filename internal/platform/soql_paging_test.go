package platform

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"

	soqleng "salescompel/internal/soql"
)

// bulkItems inserts n Item__c rows directly, with deterministic Name and
// Txt__c values that order the same way as their Id membership.
func bulkItems(t *testing.T, f typedFixture, n int) {
	t.Helper()
	col := f.cols["Item__c.Txt__c"]
	serial := f.cols["Item__c.Serial__c"]
	_, err := f.s.pool.Exec(context.Background(),
		`INSERT INTO `+f.items.TableSQL()+` (tenant_id,id,name,"`+col+`","`+serial+`")
		 SELECT $1,$2||lpad(g::text,15,'0'),'Row-'||lpad(g::text,6,'0'),'T'||lpad(g::text,6,'0'),'BulkS-'||g
		 FROM generate_series(1,$3) g`,
		f.p.Tenant, f.items.Prefix, n)
	if err != nil {
		t.Fatal(err)
	}
}

func bulkAccounts(t *testing.T, f typedFixture, n int) {
	t.Helper()
	_, err := f.s.pool.Exec(context.Background(),
		`INSERT INTO public.accounts (tenant_id,id,name,compat_text)
		 SELECT $1,'001'||lpad(g::text,15,'0'),'Acct-'||lpad(g::text,6,'0'),'secret-'||g
		 FROM generate_series(1,$2) g`, f.p.Tenant, n)
	if err != nil {
		t.Fatal(err)
	}
}

// tenantExec runs one statement with the tenant context set, matching how the
// runtime reaches a forced-RLS table.
func tenantExec(t *testing.T, f typedFixture, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", f.p.Tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func locatorID(t *testing.T, path string) string {
	t.Helper()
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '-' {
			return path[:i]
		}
	}
	t.Fatalf("locator path has no offset: %q", path)
	return ""
}

func pageRecords(t *testing.T, res soqleng.Result) []string {
	t.Helper()
	out := make([]string, len(res.Records))
	for i, rec := range res.Records {
		v, ok := rec["Txt__c"].(string)
		if !ok {
			t.Fatalf("record %d has no Txt__c: %#v", i, rec)
		}
		out[i] = v
	}
	return out
}

func TestPagingBoundaries(t *testing.T) {
	ctx := context.Background()

	t.Run("exactly one page", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 2000)
		res, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c", 2000)
		if err != nil {
			t.Fatal(err)
		}
		if res.TotalSize != 2000 || !res.Done || len(res.Records) != 2000 || res.NextLocator != "" {
			t.Fatalf("one page: total=%d done=%v rows=%d next=%q", res.TotalSize, res.Done, len(res.Records), res.NextLocator)
		}
	})

	t.Run("two pages", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 2001)
		res, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c", 2000)
		if err != nil {
			t.Fatal(err)
		}
		if res.TotalSize != 2001 || res.Done || len(res.Records) != 2000 || res.NextLocator == "" {
			t.Fatalf("first page: total=%d done=%v rows=%d next=%q", res.TotalSize, res.Done, len(res.Records), res.NextLocator)
		}
		more, err := f.s.QueryMore(ctx, f.p, res.NextLocator, 2000)
		if err != nil {
			t.Fatal(err)
		}
		if more.TotalSize != 2001 || !more.Done || len(more.Records) != 1 || more.NextLocator != "" {
			t.Fatalf("second page: total=%d done=%v rows=%d next=%q", more.TotalSize, more.Done, len(more.Records), more.NextLocator)
		}
	})

	t.Run("limit pages at batch", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 5000)
		res, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c LIMIT 4500", 500)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		pages := 0
		for {
			pages++
			if res.TotalSize != 4500 {
				t.Fatalf("page %d totalSize=%d, want 4500", pages, res.TotalSize)
			}
			got = append(got, pageRecords(t, res)...)
			if res.Done {
				break
			}
			if res.NextLocator == "" {
				t.Fatalf("page %d not done but no locator", pages)
			}
			res, err = f.s.QueryMore(ctx, f.p, res.NextLocator, 500)
			if err != nil {
				t.Fatal(err)
			}
		}
		if pages != 9 {
			t.Fatalf("pages=%d want 9", pages)
		}
		if len(got) != 4500 {
			t.Fatalf("rows=%d want 4500", len(got))
		}
		for i, v := range got {
			want := "T" + pad6(i+1)
			if v != want {
				t.Fatalf("row %d=%q want %q", i, v, want)
			}
			if i > 0 && got[i-1] >= v {
				t.Fatalf("rows out of order at %d: %q then %q", i, got[i-1], v)
			}
		}
	})
}

func pad6(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

func TestPagingDeletion(t *testing.T) {
	f := newTypedFixture(t)
	bulkItems(t, f, 3000)
	ctx := context.Background()
	txtCol := f.cols["Item__c.Txt__c"]

	first, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c", 500)
	if err != nil {
		t.Fatal(err)
	}
	if first.TotalSize != 3000 || first.Done || len(first.Records) != 500 {
		t.Fatalf("first page %#v", first)
	}

	// Delete ten rows that belong to page three.
	rows, err := f.s.pool.Query(ctx,
		`SELECT id FROM `+f.items.TableSQL()+` WHERE tenant_id=$1 ORDER BY "`+txtCol+`" LIMIT 10 OFFSET 1100`,
		f.p.Tenant)
	if err != nil {
		t.Fatal(err)
	}
	var deleted []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		deleted = append(deleted, id)
	}
	rows.Close()
	if len(deleted) != 10 {
		t.Fatalf("deleted ids=%d", len(deleted))
	}
	tenantExec(t, f, `DELETE FROM `+f.items.TableSQL()+` WHERE tenant_id=$1 AND id = ANY($2::text[])`, f.p.Tenant, deleted)

	// Update a row that belongs to page two.
	var updated string
	if err = f.s.pool.QueryRow(ctx,
		`SELECT id FROM `+f.items.TableSQL()+` WHERE tenant_id=$1 ORDER BY "`+txtCol+`" LIMIT 1 OFFSET 600`,
		f.p.Tenant).Scan(&updated); err != nil {
		t.Fatal(err)
	}
	tenantExec(t, f, `UPDATE `+f.items.TableSQL()+` SET "`+txtCol+`"='UPDATED' WHERE tenant_id=$1 AND id=$2`, f.p.Tenant, updated)

	page2, err := f.s.QueryMore(ctx, f.p, first.NextLocator, 500)
	if err != nil {
		t.Fatal(err)
	}
	if page2.TotalSize != 3000 || page2.Done {
		t.Fatalf("page2 total=%d done=%v", page2.TotalSize, page2.Done)
	}
	sawUpdate := false
	for _, rec := range page2.Records {
		if rec["Txt__c"] == "UPDATED" {
			sawUpdate = true
		}
	}
	if !sawUpdate {
		t.Fatalf("field update missing from page 2")
	}

	page3, err := f.s.QueryMore(ctx, f.p, page2.NextLocator, 500)
	if err != nil {
		t.Fatal(err)
	}
	if page3.TotalSize != 3000 {
		t.Fatalf("page3 totalSize=%d want 3000", page3.TotalSize)
	}
	if len(page3.Records) != 490 {
		t.Fatalf("page3 rows=%d want 490", len(page3.Records))
	}
	seen := map[string]bool{}
	for _, id := range deleted {
		seen[id] = true
	}
	for _, rec := range page3.Records {
		if id, _ := rec["Id"].(string); seen[id] {
			t.Fatalf("deleted row %s returned on page 3", id)
		}
	}
}

func TestLocatorCatalogChange(t *testing.T) {
	ctx := context.Background()

	t.Run("denied field", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkAccounts(t, f, 201)
		first, err := f.s.Query(ctx, f.p, "SELECT Id, Name, Compat_Text__c FROM Account ORDER BY Name", 200)
		if err != nil {
			t.Fatal(err)
		}
		if first.NextLocator == "" {
			t.Fatal("no locator for a two-page result")
		}
		// The same user, now in the restricted role.
		restricted := f.p
		restricted.Role = "restricted"
		_, err = f.s.QueryMore(ctx, restricted, first.NextLocator, 200)
		if err == nil {
			t.Fatal("restricted page 2 succeeded")
		}
		if code, _ := apiCode(err); code != "INVALID_QUERY_LOCATOR" {
			t.Fatalf("code=%q want INVALID_QUERY_LOCATOR", code)
		}
		if msg := err.Error(); strings.Contains(msg, "Compat_Text__c") || strings.Contains(msg, "secret") {
			t.Fatalf("error leaks the denied field: %q", msg)
		}
	})

	t.Run("removed field", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 201)
		first, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c", 200)
		if err != nil {
			t.Fatal(err)
		}
		if first.NextLocator == "" {
			t.Fatal("no locator for a two-page result")
		}
		tenantExec(t, f,
			"DELETE FROM sc_catalog.fields WHERE tenant_id=$1 AND object_id=$2 AND api_name='Txt__c'",
			f.p.Tenant, f.items.ID)
		tenantExec(t, f, "UPDATE sc_catalog.revisions SET revision=revision+1 WHERE tenant_id=$1", f.p.Tenant)
		_, err = f.s.QueryMore(ctx, f.p, first.NextLocator, 200)
		if err == nil {
			t.Fatal("page 2 succeeded with the field removed")
		}
		if code, _ := apiCode(err); code != "INVALID_QUERY_LOCATOR" {
			t.Fatalf("code=%q want INVALID_QUERY_LOCATOR", code)
		}
		if msg := err.Error(); strings.Contains(msg, "Txt__c") {
			t.Fatalf("error leaks the removed field: %q", msg)
		}
	})
}

// pagesEqual compares two result pages' records for replay.
func pagesEqual(a, b soqleng.Result) bool {
	return a.TotalSize == b.TotalSize && a.Done == b.Done && reflect.DeepEqual(a.Records, b.Records)
}
