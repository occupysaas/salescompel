package soql

import "testing"

// TestLikePatternLiteral pins the decoded literal value against the LIKE
// pattern encoding, so a literal backslash before a wildcard stays
// distinguishable from an escaped wildcard. spec: soql-03
func TestLikePatternLiteral(t *testing.T) {
	for _, tt := range []struct {
		text    string
		value   string
		pattern string
	}{
		{`SELECT Id FROM Item__c WHERE Txt__c LIKE 'a\%'`, `a\%`, `a\%`},
		{`SELECT Id FROM Item__c WHERE Txt__c LIKE 'a\\%'`, `a\%`, `a\\%`},
		{`SELECT Id FROM Item__c WHERE Txt__c = 'a\\b'`, `a\b`, `a\\b`},
		{`SELECT Id FROM Item__c WHERE Txt__c = 'a\%x'`, `a\%x`, `a\%x`},
	} {
		q, err := Parse(tt.text)
		if err != nil {
			t.Fatalf("Parse(%s): %v", tt.text, err)
		}
		cmp, ok := q.Where.(*Compare)
		if !ok {
			t.Fatalf("Parse(%s) where = %#v", tt.text, q.Where)
		}
		if cmp.Right.Text != tt.value || cmp.Right.Pattern != tt.pattern {
			t.Fatalf("Parse(%s) literal = text %q pattern %q, want %q / %q",
				tt.text, cmp.Right.Text, cmp.Right.Pattern, tt.value, tt.pattern)
		}
	}
}
