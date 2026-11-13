package platform

import (
	"context"
	"testing"
)

// TestOffsetTotalSize checks the post-LIMIT/OFFSET totalSize. page-01 forbids
// a locator for an OFFSET query, so the single page carries at most one batch
// and has no nextRecordsUrl; totalSize still reports the full result size.
// spec: page-01
func TestOffsetTotalSize(t *testing.T) {
	ctx := context.Background()
	f := newTypedFixture(t)
	bulkItems(t, f, 450)

	cases := []struct {
		query     string
		limit     int
		wantTotal int
		wantRows  int
	}{
		{"SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c LIMIT 1000 OFFSET 50", 200, 400, 200},
		{"SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c LIMIT 400 OFFSET 25", 200, 400, 200},
		{"SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c LIMIT 100 OFFSET 400", 200, 50, 50},
		{"SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c OFFSET 1000", 200, 0, 0},
	}
	for _, tc := range cases {
		res, err := f.s.Query(ctx, f.p, tc.query, tc.limit)
		if err != nil {
			t.Fatalf("%s: %v", tc.query, err)
		}
		if res.TotalSize != tc.wantTotal || len(res.Records) != tc.wantRows || !res.Done || res.NextLocator != "" {
			t.Fatalf("%s: total=%d rows=%d done=%v next=%q, want total=%d rows=%d done=true next=\"\"",
				tc.query, res.TotalSize, len(res.Records), res.Done, res.NextLocator, tc.wantTotal, tc.wantRows)
		}
	}

	var locators int
	if err := f.s.pool.QueryRow(ctx, "SELECT count(*) FROM sc_query.locators WHERE tenant_id=$1", f.p.Tenant).Scan(&locators); err != nil {
		t.Fatal(err)
	}
	if locators != 0 {
		t.Fatalf("OFFSET queries stored %d locators", locators)
	}
}
