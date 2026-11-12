package soql

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGolden(t *testing.T) {
	dir := "testdata/parse"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".soql") {
			continue
		}
		seen++
		name := strings.TrimSuffix(e.Name(), ".soql")
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		q, perr := Parse(string(raw))
		var got []byte
		if perr != nil {
			if !malformedCode(perr) {
				t.Errorf("%s: error %v is not MALFORMED_QUERY", name, perr)
			}
			got = []byte(`{"error":"MALFORMED_QUERY"}`)
		} else {
			got, err = json.MarshalIndent(q, "", " ")
			if err != nil {
				t.Fatalf("%s: marshal: %v", name, err)
			}
		}
		got = append(got, '\n')
		path := filepath.Join(dir, name+".json")
		if os.Getenv("UPDATE_GOLDEN") != "" {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil || strings.TrimSpace(string(want)) != strings.TrimSpace(string(got)) {
			t.Errorf("%s differs from %s (%v); rerun with UPDATE_GOLDEN=1 and review\ngot:\n%s", name, path, err, got)
		}
	}
	if seen < 50 {
		t.Fatalf("corpus too small: %d cases", seen)
	}
}

func TestParseSemantics(t *testing.T) {
	q, err := Parse("SELECT Stage__c, COUNT(Id) total, SUM(Amount__c) FROM Item__c WHERE Name LIKE 'A%' AND Num__c > 5 GROUP BY Stage__c HAVING COUNT(Id) > 1 ORDER BY total DESC NULLS LAST LIMIT 10 OFFSET 2 WITH SECURITY_ENFORCED")
	if err != nil {
		t.Fatal(err)
	}
	if q.From != "Item__c" || len(q.Select) != 3 || q.Limit == nil || *q.Limit != 10 || q.Offset == nil || *q.Offset != 2 || q.With != "SECURITY_ENFORCED" {
		t.Fatalf("query shape wrong: %#v", q)
	}
	if _, ok := q.Select[1].(*Aggregate); !ok {
		t.Fatalf("select[1] = %T want *Aggregate", q.Select[1])
	}
	if len(q.OrderBy) != 1 || !q.OrderBy[0].Desc || q.OrderBy[0].Nulls != NullLast {
		t.Fatalf("order by wrong: %#v", q.OrderBy)
	}
}

func TestParseMixedAndOr(t *testing.T) {
	for _, s := range []string{
		"SELECT Id FROM Item__c WHERE Name = 'a' AND Num__c > 5 OR Name = 'b'",
		"SELECT Id FROM Item__c WHERE Name = 'a' OR Name = 'b' AND Num__c > 5",
	} {
		if _, err := Parse(s); !malformedCode(err) {
			t.Errorf("Parse(%q) error = %v want MALFORMED_QUERY", s, err)
		}
	}
	if _, err := Parse("SELECT Id FROM Item__c WHERE (Name = 'a' AND Num__c > 5) OR Name = 'b'"); err != nil {
		t.Errorf("parenthesized mixed AND/OR rejected: %v", err)
	}
}
