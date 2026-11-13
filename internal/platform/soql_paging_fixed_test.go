package platform

import (
	"context"
	"testing"
	"time"

	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	soqleng "salescompel/internal/soql"
)

// runAtClock executes a query with an injected clock and commits the
// transaction so any locator it stores is visible to QueryMore.
func runAtClock(t *testing.T, f typedFixture, text string, now time.Time, batch int) soqleng.Result {
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
	snap, err := metadata.Load(ctx, tx, f.p.Tenant)
	if err != nil {
		t.Fatal(err)
	}
	exec, err := policy.ForAPI(f.p, "65.0")
	if err != nil {
		t.Fatal(err)
	}
	res, err := soqleng.Run(ctx, tx, exec, snap, text,
		soqleng.Options{BatchSize: batch, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return res
}

// TestPagingFixedMembership proves later pages use the captured IDs only: a
// row that no longer matches the original WHERE (including a TODAY literal
// evaluated against a different clock) does not vanish on page two, and no
// row repeats. spec: page-01
func TestPagingFixedMembership(t *testing.T) {
	ctx := context.Background()
	f := newTypedFixture(t)
	bulkItems(t, f, 400)
	dayCol := f.cols["Item__c.Day__c"]
	txtCol := f.cols["Item__c.Txt__c"]
	tenantExec(t, f, `UPDATE `+f.items.TableSQL()+` SET "`+dayCol+`"='2026-06-15' WHERE tenant_id=$1`, f.p.Tenant)

	// The injected TODAY is far from the real clock NextPage re-binds with.
	first := runAtClock(t, f,
		"SELECT Id, Txt__c, Day__c FROM Item__c WHERE Day__c = TODAY ORDER BY Txt__c",
		time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC), 200)
	if first.TotalSize != 400 || first.Done || len(first.Records) != 200 || first.NextLocator == "" {
		t.Fatalf("first page = total %d done %v rows %d next %q", first.TotalSize, first.Done, len(first.Records), first.NextLocator)
	}

	// Five rows belonging to page two stop matching the original filter.
	rows, err := f.s.pool.Query(ctx,
		`SELECT id FROM `+f.items.TableSQL()+` WHERE tenant_id=$1 ORDER BY "`+txtCol+`" LIMIT 5 OFFSET 205`,
		f.p.Tenant)
	if err != nil {
		t.Fatal(err)
	}
	var moved []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		moved = append(moved, id)
	}
	rows.Close()
	if len(moved) != 5 {
		t.Fatalf("moved ids = %d", len(moved))
	}
	tenantExec(t, f, `UPDATE `+f.items.TableSQL()+` SET "`+dayCol+`"='2026-06-16', "`+txtCol+`"='MOVED' WHERE tenant_id=$1 AND id = ANY($2::text[])`,
		f.p.Tenant, moved)

	second, err := f.s.QueryMore(ctx, f.p, first.NextLocator, 200)
	if err != nil {
		t.Fatal(err)
	}
	if second.TotalSize != 400 || !second.Done || len(second.Records) != 200 || second.NextLocator != "" {
		t.Fatalf("second page = total %d done %v rows %d next %q", second.TotalSize, second.Done, len(second.Records), second.NextLocator)
	}
	seen := map[string]bool{}
	for _, rec := range first.Records {
		seen[rec["Id"].(string)] = true
	}
	movedSeen := 0
	wantMoved := map[string]bool{}
	for _, id := range moved {
		wantMoved[id] = true
	}
	for _, rec := range second.Records {
		id, _ := rec["Id"].(string)
		if seen[id] {
			t.Fatalf("row %s repeated across pages", id)
		}
		seen[id] = true
		if wantMoved[id] {
			movedSeen++
			if rec["Txt__c"] != "MOVED" {
				t.Fatalf("row %s did not show its current value: %#v", id, rec)
			}
		}
	}
	if movedSeen != 5 {
		t.Fatalf("moved rows on page two = %d, want 5 (captured membership)", movedSeen)
	}
	if len(seen) != 400 {
		t.Fatalf("distinct rows across pages = %d, want 400", len(seen))
	}
}
