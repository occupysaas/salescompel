package soql

import (
	"encoding/json"
	"testing"
)

func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"SELECT Id FROM Account",
		"SELECT COUNT() FROM Item__c",
		"SELECT Name, (SELECT LastName FROM Contacts LIMIT 2) FROM Account",
		"SELECT Id FROM Item__c WHERE Name LIKE 'a\\%' AND Num__c IN (1,2)",
		"SELECT TYPEOF What WHEN Account THEN Phone ELSE Name END FROM Event",
		"SELECT Id FROM Item__c WHERE Day__c = LAST_N_DAYS:30",
		"SELECT Id FROM Account WHERE Name = 'x' OR Name = 'y'",
		"SELECT Id FROM Account;",
		"'\\u00e9'",
		"SELECT",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		q, err := Parse(s)
		if err != nil {
			if !malformedCode(err) {
				t.Fatalf("Parse error %v is not MALFORMED_QUERY", err)
			}
			return
		}
		if q == nil {
			t.Fatal("nil query without error")
		}
		if _, err := json.Marshal(q); err != nil {
			t.Fatalf("marshal AST: %v", err)
		}
	})
}
