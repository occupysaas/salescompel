package soql

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"salescompel/internal/contract"
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

// TestParseNodePositions pins the rule that a node's Pos is the first token
// of that node.
func TestParseNodePositions(t *testing.T) {
	t.Run("field ref and aggregate", func(t *testing.T) {
		const text = "SELECT Id, COUNT(Total__c) FROM Account"
		q, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		fr, ok := q.Select[0].(*FieldRef)
		if !ok {
			t.Fatalf("select[0] = %T want *FieldRef", q.Select[0])
		}
		if want := strings.Index(text, "Id"); fr.Pos.Offset != want {
			t.Errorf("FieldRef Pos = %d want %d", fr.Pos.Offset, want)
		}
		agg, ok := q.Select[1].(*Aggregate)
		if !ok {
			t.Fatalf("select[1] = %T want *Aggregate", q.Select[1])
		}
		if want := strings.Index(text, "COUNT"); agg.Pos.Offset != want {
			t.Errorf("Aggregate Pos = %d want %d", agg.Pos.Offset, want)
		}
	})

	t.Run("and", func(t *testing.T) {
		const text = "SELECT Id FROM Account WHERE Name = 'a' AND Num__c > 1"
		q, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		and, ok := q.Where.(*And)
		if !ok {
			t.Fatalf("Where = %T want *And", q.Where)
		}
		first := strings.Index(text, "Name")
		if and.Pos.Offset != first {
			t.Errorf("And Pos = %d want %d", and.Pos.Offset, first)
		}
		c0, ok := and.Items[0].(*Compare)
		if !ok {
			t.Fatalf("Items[0] = %T want *Compare", and.Items[0])
		}
		if c0.Pos.Offset != first {
			t.Errorf("first Compare Pos = %d want %d", c0.Pos.Offset, first)
		}
		if fr, ok := c0.Left.(*FieldRef); !ok || fr.Pos.Offset != first {
			t.Errorf("first Compare left Pos = %v want %d", c0.Left, first)
		}
		second := strings.Index(text, "Num__c")
		if c1, ok := and.Items[1].(*Compare); !ok || c1.Pos.Offset != second {
			t.Errorf("second Compare Pos = %v want %d", and.Items[1], second)
		}
	})

	t.Run("or", func(t *testing.T) {
		const text = "SELECT Id FROM Account WHERE Name = 'a' OR Num__c > 1"
		q, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		or, ok := q.Where.(*Or)
		if !ok {
			t.Fatalf("Where = %T want *Or", q.Where)
		}
		if want := strings.Index(text, "Name"); or.Pos.Offset != want {
			t.Errorf("Or Pos = %d want %d", or.Pos.Offset, want)
		}
	})

	t.Run("in", func(t *testing.T) {
		const text = "SELECT Id FROM Account WHERE Name IN ('a', 'b')"
		q, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		in, ok := q.Where.(*In)
		if !ok {
			t.Fatalf("Where = %T want *In", q.Where)
		}
		if want := strings.Index(text, "Name"); in.Pos.Offset != want {
			t.Errorf("In Pos = %d want %d", in.Pos.Offset, want)
		}
	})

	t.Run("call", func(t *testing.T) {
		const text = "SELECT CALENDAR_YEAR(At__c) FROM Account"
		q, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		call, ok := q.Select[0].(*Call)
		if !ok {
			t.Fatalf("select[0] = %T want *Call", q.Select[0])
		}
		if want := strings.Index(text, "CALENDAR_YEAR"); call.Pos.Offset != want {
			t.Errorf("Call Pos = %d want %d", call.Pos.Offset, want)
		}
	})
}

func TestParseErrorMessages(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"unknown function", "SELECT FOO(Id) FROM Item__c", "FOO"},
		{"typeof without branch", "SELECT TYPEOF What FROM Event", "TYPEOF"},
		{"mixed and/or", "SELECT Id FROM Account WHERE Name = 'supersecret' AND Num__c > 5 OR Name = 'x'", "AND"},
		{"unterminated subquery", "SELECT Name, (SELECT Id FROM Contacts", "subquery"},
		{"trailing input", "SELECT Id FROM Account 5", "trailing"},
		{"unexpected character", "SELECT Id FROM Account;", "unexpected character"},
		{"unsupported with", "SELECT Id FROM Account WITH FOO", "WITH"},
		{"order by bare nulls", "SELECT Id FROM Account ORDER BY Name NULLS", "NULLS FIRST or NULLS LAST"},
		{"dangling conjunction", "SELECT Id FROM Account WHERE Name = 'sekrit' AND", "expected"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := parseErrorMessage(t, c.query)
			if !strings.Contains(msg, c.want) {
				t.Errorf("Parse(%q) message %q does not name %q", c.query, msg, c.want)
			}
			for _, value := range []string{"supersecret", "sekrit"} {
				if strings.Contains(msg, value) {
					t.Errorf("Parse(%q) message %q echoes a query value", c.query, msg)
				}
			}
		})
	}
}

func parseErrorMessage(t *testing.T, query string) string {
	t.Helper()
	_, err := Parse(query)
	if err == nil {
		t.Fatalf("Parse(%q) succeeded, want MALFORMED_QUERY", query)
	}
	var ae *contract.APIError
	if !errors.As(err, &ae) || ae.Code != "MALFORMED_QUERY" {
		t.Fatalf("Parse(%q) error = %v want MALFORMED_QUERY", query, err)
	}
	return ae.Message
}
