package platform

import (
	"context"
	"errors"
	"strings"
	"testing"

	"salescompel/internal/metadata"
)

func childResult(t *testing.T, record map[string]any, key string) (int, bool, []map[string]any) {
	t.Helper()
	group, ok := record[key].(map[string]any)
	if !ok {
		t.Fatalf("missing child result %q in %#v", key, record)
	}
	total, ok := group["totalSize"].(int)
	if !ok {
		t.Fatalf("child totalSize is %T", group["totalSize"])
	}
	done, ok := group["done"].(bool)
	if !ok {
		t.Fatalf("child done is %T", group["done"])
	}
	rows, ok := group["records"].([]map[string]any)
	if !ok {
		t.Fatalf("child records is %T", group["records"])
	}
	return total, done, rows
}

func TestChildSubqueries(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()
	if _, err := metadata.CreateField(ctx, s.pool, p.Tenant, metadata.FieldInput{
		Object: "Item__c", Name: "Account__c", Label: "Account", Type: "Lookup",
		ReferenceTo: "Account", ChildRelationshipName: "Typeditems__r",
	}); err != nil {
		t.Fatal(err)
	}

	emptyID, err := s.Create(ctx, p, "Account", map[string]any{"Name": "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	parentID, err := s.Create(ctx, p, "Account", map[string]any{"Name": "Parent"})
	if err != nil {
		t.Fatal(err)
	}
	for _, last := range []string{"Zulu", "Alpha", "Beta"} {
		if _, err = s.Create(ctx, p, "Contact", map[string]any{"LastName": last, "AccountId": parentID}); err != nil {
			t.Fatal(err)
		}
	}
	for _, stage := range []string{"Open", "Closed"} {
		if _, err = s.Create(ctx, p, "Item__c", map[string]any{"Stage__c": stage, "Account__c": parentID}); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("grouping null order and per-parent limit", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Name, (SELECT LastName FROM Contacts ORDER BY LastName LIMIT 2) FROM Account ORDER BY Name")
		if res.TotalSize != 2 || !res.Done {
			t.Fatalf("parent result %#v", res)
		}
		if res.Records[0]["Contacts"] != nil {
			t.Fatalf("empty child must be null: %#v", res.Records[0])
		}
		total, done, rows := childResult(t, res.Records[1], "Contacts")
		if total != 2 || !done || len(rows) != 2 {
			t.Fatalf("child result total=%d done=%v rows=%#v", total, done, rows)
		}
		if rows[0]["LastName"] != "Alpha" || rows[1]["LastName"] != "Beta" {
			t.Fatalf("child ORDER BY/LIMIT not applied per parent: %#v", rows)
		}
	})

	t.Run("custom relationship with filter", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Name, (SELECT Name FROM Typeditems__r WHERE Stage__c = 'Open') FROM Account WHERE Id = '"+parentID+"'")
		total, done, rows := childResult(t, res.Records[0], "Typeditems__r")
		if total != 1 || !done || len(rows) != 1 {
			t.Fatalf("custom child result %#v", res.Records[0])
		}
	})

	t.Run("parent path inside child", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Name, (SELECT Account.Name FROM Contacts WHERE LastName = 'Alpha') FROM Account WHERE Id = '"+parentID+"'")
		_, _, rows := childResult(t, res.Records[0], "Contacts")
		account, ok := rows[0]["Account"].(map[string]any)
		if !ok || account["Name"] != "Parent" {
			t.Fatalf("child parent path %#v", rows[0])
		}
	})

	t.Run("duplicate child key", func(t *testing.T) {
		err := soqlErr(t, s, p, "SELECT (SELECT Id FROM Contacts), (SELECT Id FROM Contacts) FROM Account")
		if code, _ := apiCode(err); code != "MALFORMED_QUERY" {
			t.Fatalf("duplicate child error %v", err)
		}
	})

	t.Run("cross tenant child never appears", func(t *testing.T) {
		foreignContact, err := s.Create(ctx, f.other, "Contact", map[string]any{"LastName": "Foreign"})
		if err != nil {
			t.Fatal(err)
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, "SET LOCAL session_replication_role = 'replica'"); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "UPDATE contacts SET account_id=$1 WHERE tenant_id=$2 AND id=$3", parentID, f.other.Tenant, foreignContact); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		res := soqlQuery(t, s, p, "SELECT Id, (SELECT Id FROM Contacts WHERE LastName = 'Foreign') FROM Account WHERE Id = '"+parentID+"'")
		if res.Records[0]["Contacts"] != nil {
			t.Fatalf("cross-tenant child leaked: %#v", res.Records[0])
		}
	})

	t.Run("child cap", func(t *testing.T) {
		_, err = s.pool.Exec(ctx, `INSERT INTO contacts(tenant_id,id,last_name,account_id)
			SELECT $1,'003'||lpad((200000000000000+g)::text,15,'0'),'Bulk',$2
			FROM generate_series(1,2001) g`, p.Tenant, emptyID)
		if err != nil {
			t.Fatal(err)
		}
		err = soqlErr(t, s, p, "SELECT Id, (SELECT Id FROM Contacts ORDER BY Id) FROM Account WHERE Id = '"+emptyID+"'")
		var api *APIError
		if !errors.As(err, &api) || api.Code != "QUERY_TOO_COMPLICATED" ||
			api.Message != "child relationship exceeds candidate limit; add a LIMIT to the subquery" {
			t.Fatalf("child cap error %v", err)
		}
	})
}

func TestSemiJoins(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	matched, err := s.Create(ctx, a, "Account", map[string]any{"Name": "Matched"})
	if err != nil {
		t.Fatal(err)
	}
	unmatched, err := s.Create(ctx, a, "Account", map[string]any{"Name": "Unmatched"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, a, "Contact", map[string]any{"LastName": "Anderson", "AccountId": matched}); err != nil {
		t.Fatal(err)
	}
	// A null AccountId demonstrates that the anti-join is NOT EXISTS, not NOT IN.
	if _, err = s.Create(ctx, a, "Contact", map[string]any{"LastName": "Abbott"}); err != nil {
		t.Fatal(err)
	}

	res := soqlQuery(t, s, a, "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE LastName LIKE 'A%')")
	if res.TotalSize != 1 || res.Records[0]["Id"] != matched {
		t.Fatalf("semi-join result %#v", res)
	}
	res = soqlQuery(t, s, a, "SELECT Id FROM Account WHERE Id NOT IN (SELECT AccountId FROM Contact WHERE LastName LIKE 'A%')")
	if res.TotalSize != 1 || res.Records[0]["Id"] != unmatched {
		t.Fatalf("anti-join result %#v", res)
	}

	for name, query := range map[string]string{
		"under OR":         "SELECT Id FROM Account WHERE Name = 'Matched' OR Id IN (SELECT AccountId FROM Contact)",
		"nested":           "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE Id IN (SELECT Id FROM Contact))",
		"non Id subselect": "SELECT Id FROM Account WHERE Id IN (SELECT LastName FROM Contact)",
	} {
		t.Run(name, func(t *testing.T) {
			err := soqlErr(t, s, a, query)
			if code, _ := apiCode(err); code != "MALFORMED_QUERY" || strings.Contains(err.Error(), "Matched") {
				t.Fatalf("semi-join restriction error %v", err)
			}
		})
	}
}
