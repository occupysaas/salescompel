package platform

import "testing"

func TestParseQuery(t *testing.T) {
	q, err := ParseQuery("SELECT Id,Account.Name FROM Contact WHERE LastName = 'O\\'Brien' ORDER BY LastName DESC LIMIT 2")
	if err != nil || q.Object != "Contact" || q.WhereValue == nil || *q.WhereValue != "O'Brien" || !q.Desc || q.Limit != 2 || len(q.Fields) != 2 {
		t.Fatal(q, err)
	}
}
func TestRejectUnsupportedQuery(t *testing.T) {
	for _, s := range []string{"SELECT * FROM Account", "SELECT Id FROM Account; DROP TABLE accounts", "SELECT Id FROM Account WHERE Name = 'x' OR Name = 'y'", "SELECT Id FROM Account LIMIT 0", "SELECT Id FROM Account LIMIT 2001", "SELECT Id FROM Account WHERE Name = 'oops", "SELECT Id FROM Account LIMIT 2 garbage", "SELECT count(Id) FROM Account", "SELECT Id FROM Account WHERE Name = 3"} {
		t.Run(s, func(t *testing.T) {
			if _, err := ParseQuery(s); err == nil {
				t.Fatal("accepted unsupported syntax")
			}
		})
	}
}
func TestRejectUndeliverableLiteral(t *testing.T) {
	for _, s := range []string{"SELECT Id FROM Account WHERE Name = '\xff'", "SELECT Id FROM Account WHERE Name = 'a\x00'"} {
		if _, err := ParseQuery(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
