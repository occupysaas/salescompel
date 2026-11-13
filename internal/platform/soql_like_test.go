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

// TestNonLikeBackslashIsLiteral checks that wildcard escapes are interpreted
// only by LIKE: equality keeps the literal backslash and percent from the
// source text. spec: soql-03
func TestNonLikeBackslashIsLiteral(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()
	for _, txt := range []string{`a\%x`, "a%x"} {
		if _, err := s.Create(ctx, p, "Item__c", map[string]any{"Txt__c": txt}); err != nil {
			t.Fatal(err)
		}
	}
	res := soqlQuery(t, s, p, `SELECT Txt__c FROM Item__c WHERE Txt__c = 'a\%x'`)
	if res.TotalSize != 1 || res.Records[0]["Txt__c"] != `a\%x` {
		t.Fatalf("equality with literal backslash = %#v, want exactly a\\%%x", res.Records)
	}
}
