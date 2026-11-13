package platform

import (
	"context"
	"strings"
	"testing"
	"time"

	"salescompel/internal/metadata"
	soqleng "salescompel/internal/soql"
)

func selectSQL(t *testing.T, s *Store, p Principal, sql string) ([]map[string]any, error) {
	t.Helper()
	res, e := s.Query(context.Background(), p, sql, 2000)
	if e != nil {
		return nil, e
	}
	return res.Records, nil
}

// columnMetadataFor binds text against the standard catalog and computes the
// REST column metadata used by the columns=true route.
func columnMetadataFor(t *testing.T, p Principal, text string) ([]map[string]any, error) {
	t.Helper()
	q, e := soqleng.Parse(text)
	if e != nil {
		return nil, e
	}
	snapshot := metadata.Snapshot{Tenant: p.Tenant, Objects: metadata.StandardObjects()}
	b, e := soqleng.Bind(q, snapshot, p, time.Now)
	if e != nil {
		return nil, e
	}
	return soqleng.ColumnMetadata(b), nil
}
func TestStoreQueryCaseInsensitiveNames(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	id, e := s.Create(ctx, p, "Account", map[string]any{"Name": "Parent"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(ctx, p, "Contact", map[string]any{"LastName": "Child", "AccountId": id}); e != nil {
		t.Fatal(e)
	}
	rows, e := selectSQL(t, s, p, "select id, name from account")
	if e != nil || len(rows) != 1 || rows[0]["Name"] != "Parent" || rows[0]["Id"] != id || rows[0]["attributes"].(map[string]any)["type"] != "Account" {
		t.Fatal(rows, e)
	}
	rows, e = selectSQL(t, s, p, "select account.name from contact")
	if e != nil || rows[0]["Account"].(map[string]any)["Name"] != "Parent" {
		t.Fatal(rows, e)
	}
	cols, e := columnMetadataFor(t, p, "select id, account.name from contact")
	if e != nil || stringJSON(cols) != `[{"columnName":"Id"},{"columnName":"Account","joinColumns":[{"columnName":"Name"}]}]` {
		t.Fatal(cols, e)
	}
	p.Role = "restricted"
	for _, sql := range []string{"select compat_text__c from account", "select id from account where compat_text__c = 'x'", "select id from account order by compat_text__c"} {
		if _, e = selectSQL(t, s, p, sql); e == nil {
			t.Fatal(sql)
		}
	}
}
func TestStoreQueryTextEquality(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	for _, name := range []string{"MixedCase", "é", "I"} {
		if _, e := s.Create(context.Background(), p, "Account", map[string]any{"Name": name}); e != nil {
			t.Fatal(e)
		}
	}
	for _, tc := range []struct {
		value string
		count int
	}{{"mixedcase", 1}, {"É", 1}, {"e\u0301", 0}, {"ı", 0}} {
		rows, e := selectSQL(t, s, p, "select id from account where name = '"+tc.value+"'")
		if e != nil || len(rows) != tc.count {
			t.Fatal(tc, rows, e)
		}
	}
}
func TestStoreQueryIDFilters(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	id, e := s.Create(ctx, p, "Account", map[string]any{"Name": "Parent"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(ctx, p, "Contact", map[string]any{"LastName": "Child", "AccountId": id}); e != nil {
		t.Fatal(e)
	}
	for _, sql := range []string{"select id from account where id = '" + id[:15] + "'", "select id from contact where accountid = '" + id[:15] + "'"} {
		rows, e := selectSQL(t, s, p, sql)
		if e != nil || len(rows) != 1 {
			t.Fatal(rows, e)
		}
	}
	for _, sql := range []string{"select id from account where id = 'bad'", "select id from contact where accountid = '003000000000000'"} {
		_, e := selectSQL(t, s, p, sql)
		if e == nil || !strings.Contains(e.Error(), "INVALID_QUERY_FILTER_OPERATOR") {
			t.Fatal(e)
		}
	}
	rows, e := selectSQL(t, s, p, "select id from account where id = '"+strings.ToLower(id[:15])+"'")
	if e != nil || len(rows) != 0 {
		t.Fatal(rows, e)
	}
}
func TestStoreQueryNullOrdering(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	for _, v := range []any{nil, "a", "a", nil, "z"} {
		if _, e := s.Create(ctx, p, "Account", map[string]any{"Name": "Order", "Compat_Text__c": v}); e != nil {
			t.Fatal(e)
		}
	}
	// Tie ordering follows PostgreSQL's configured ID collation.
	ordered, e := s.pool.Query(ctx, "SELECT id FROM accounts WHERE tenant_id=$1 ORDER BY id", p.Tenant)
	if e != nil {
		t.Fatal(e)
	}
	rank := map[string]int{}
	for ordered.Next() {
		var id string
		if e = ordered.Scan(&id); e != nil {
			t.Fatal(e)
		}
		rank[id] = len(rank)
	}
	if e = ordered.Err(); e != nil {
		t.Fatal(e)
	}
	ordered.Close()
	for _, direction := range []string{"ASC", "DESC"} {
		for _, placement := range []string{"", " NULLS FIRST", " NULLS LAST"} {
			rows, e := selectSQL(t, s, p, "select id, compat_text__c from account order by compat_text__c "+direction+placement)
			if e != nil || len(rows) != 5 {
				t.Fatal(rows, e)
			}
			nullIndex := 0
			if placement == " NULLS LAST" {
				nullIndex = 3
			}
			if rows[nullIndex]["Compat_Text__c"] != nil || rows[nullIndex+1]["Compat_Text__c"] != nil {
				t.Fatal(rows)
			}
			for i := 1; i < len(rows); i++ {
				if rows[i]["Compat_Text__c"] == rows[i-1]["Compat_Text__c"] && rank[rows[i]["Id"].(string)] < rank[rows[i-1]["Id"].(string)] {
					t.Fatal("unstable ties", rows)
				}
			}
		}
	}
}
