package soql

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"salescompel/internal/contract"
)

func apiCodeOf(err error) (string, []string) {
	var api *contract.APIError
	if errors.As(err, &api) {
		return api.Code, api.Fields
	}
	return "", nil
}

func TestLocatorPathParsing(t *testing.T) {
	id := "01g" + strings.Repeat("a", 15)
	got, offset, err := ParseLocatorPath(id + "-2000")
	if err != nil || got != id || offset != 2000 {
		t.Fatalf("parse = %q,%d,%v", got, offset, err)
	}
	for _, bad := range []string{
		"",
		"garbage",
		id,
		id + "-",
		id + "-x",
		id + "--1",
		"01gshort-200",
		"01g" + strings.Repeat("a", 15) + "!-200",
		"01g" + strings.Repeat("a", 16) + "-200",
	} {
		if _, _, err := ParseLocatorPath(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		} else if code, _ := apiCodeOf(err); code != "INVALID_QUERY_LOCATOR" {
			t.Fatalf("%q code=%q", bad, code)
		}
	}
}

// TestLocatorCanonicalRoundTrip proves the stored canonical text re-parses to
// the same AST for every construct the parser accepts.
func TestLocatorCanonicalRoundTrip(t *testing.T) {
	for _, text := range []string{
		"SELECT Id, Name FROM Account",
		"SELECT Id, Name, Account.Name FROM Contact WHERE LastName LIKE 'A%' AND Email != null ORDER BY LastName DESC NULLS LAST LIMIT 10 OFFSET 2",
		"SELECT Stage__c, COUNT(Id) total, SUM(Amount__c), AVG(Amount__c), MIN(Day__c), MAX(At__c) FROM Item__c WHERE Name IN ('A','B') GROUP BY Stage__c HAVING COUNT(Id) > 1 ORDER BY total DESC LIMIT 10",
		"SELECT toLabel(Stage__c) FROM Item__c WHERE toLabel(Stage__c) = 'Closed label'",
		"SELECT Name, (SELECT LastName FROM Contacts WHERE LastName != 'X' ORDER BY LastName LIMIT 2) FROM Account WHERE Id IN (SELECT AccountId FROM Contact)",
		"SELECT CALENDAR_MONTH(Day__c) m, COUNT(Id) FROM Item__c WHERE DAY_ONLY(At__c) = 2026-03-09 GROUP BY CALENDAR_MONTH(Day__c) ORDER BY m",
		"SELECT Id FROM Item__c WHERE Flag__c = true AND (Multi__c INCLUDES ('A;B') OR Txt__c = 'can\\'t')",
		"SELECT FIELDS(ALL) FROM Item__c LIMIT 200",
		"SELECT Id FROM Item__c WHERE At__c > 2026-06-15T12:00:00Z",
		"SELECT Id FROM Item__c WHERE Day__c = LAST_N_DAYS:7",
	} {
		first, err := Parse(text)
		if err != nil {
			t.Fatalf("parse %q: %v", text, err)
		}
		canonical := CanonicalQuery(first)
		second, err := Parse(canonical)
		if err != nil {
			t.Fatalf("reparse %q: %v", canonical, err)
		}
		a, _ := json.Marshal(first)
		b, _ := json.Marshal(second)
		if string(a) != string(b) {
			t.Fatalf("round trip differs for %q:\ncanonical: %s\nwant: %s\ngot:  %s", text, canonical, a, b)
		}
	}
}
