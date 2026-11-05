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

func TestParseQueryNullPlacement(t *testing.T) {
	for _, tc := range []struct {
		sql  string
		want NullPlacement
	}{{"SELECT Id FROM Account ORDER BY Name NULLS FIRST", NullFirst}, {"SELECT Id FROM Account ORDER BY Name DESC NULLS LAST", NullLast}, {"select id from account order by name nulls first", NullFirst}} {
		q, e := ParseQuery(tc.sql)
		if e != nil || q.Nulls != tc.want {
			t.Fatal(q, e)
		}
	}
	for _, sql := range []string{"SELECT Id FROM Account ORDER BY Name NULLS", "SELECT Id FROM Account ORDER BY Name NULLS MIDDLE"} {
		if _, e := ParseQuery(sql); e == nil {
			t.Fatal(sql)
		}
	}
}
