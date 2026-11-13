package platform

import (
	"context"
	"testing"
)

// TestLikeBackslashEscapes distinguishes a literal backslash before a wildcard
// (`\\%`) from an escaped literal wildcard (`\%`). spec: soql-03
func TestLikeBackslashEscapes(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()
	for _, txt := range []string{"a%", `a\zz`, `a\%x`} {
		if _, err := s.Create(ctx, p, "Item__c", map[string]any{"Txt__c": txt}); err != nil {
			t.Fatal(err)
		}
	}

	// 'a\%' is an escaped literal percent: only "a%" matches.
	res := soqlQuery(t, s, p, `SELECT Id FROM Item__c WHERE Txt__c LIKE 'a\%'`)
	if res.TotalSize != 1 {
		t.Fatalf("LIKE escaped percent = %d, want 1", res.TotalSize)
	}

	// 'a\\%' is a literal backslash followed by the wildcard: both rows that
	// start with "a\" match, and "a%" does not.
	res = soqlQuery(t, s, p, `SELECT Id FROM Item__c WHERE Txt__c LIKE 'a\\%'`)
	if res.TotalSize != 2 {
		t.Fatalf("LIKE literal backslash + wildcard = %d, want 2", res.TotalSize)
	}
}
