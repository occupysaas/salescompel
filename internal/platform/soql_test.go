package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	soqleng "salescompel/internal/soql"
)

func soqlQuery(t *testing.T, s *Store, p Principal, text string) soqleng.Result {
	t.Helper()
	res, err := s.Query(context.Background(), p, text, 2000)
	if err != nil {
		t.Fatalf("query %q: %v", text, err)
	}
	return res
}

func soqlErr(t *testing.T, s *Store, p Principal, text string) error {
	t.Helper()
	_, err := s.Query(context.Background(), p, text, 2000)
	return err
}

func TestSOQLRecordQueries(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()

	accID, err := s.Create(ctx, p, "Account", map[string]any{"Name": "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, p, "Contact", map[string]any{"LastName": "Smith", "FirstName": "John", "AccountId": accID, "Email": "j@x.co"})
	if err != nil {
		t.Fatal(err)
	}

	item1, err := s.Create(ctx, p, "Item__c", map[string]any{
		"Txt__c": "Alpha", "Num__c": json.Number("42.5"), "Flag__c": true,
		"Day__c": "2026-06-15", "At__c": "2026-06-15T12:00:00Z",
		"Stage__c": "Closed", "Multi__c": "A;B", "Mail__c": "a@b.co",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, p, "Item__c", map[string]any{
		"Txt__c": "Beta", "Num__c": json.Number("10"), "Flag__c": false,
		"Day__c": "2026-01-01", "At__c": "2026-01-01T00:00:00Z",
		"Stage__c": "Open", "Multi__c": "C", "Mail__c": "c@d.co",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, p, "Item__c", map[string]any{
		"Txt__c": nil, "Num__c": nil, "Flag__c": true,
		"Day__c": "2026-12-25", "At__c": nil,
		"Stage__c": "Open", "Multi__c": nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("text equality case-insensitive", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Txt__c = 'alpha'")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Stage__c = 'closed'")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("autonumber equality case-insensitive", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Name = 'it-0001'")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize, res.Records)
		}
	})

	t.Run("numeric comparison", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Num__c > 20")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Num__c <= 42.5")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("bool equality", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Flag__c = true")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("null tests", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Txt__c = null")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Txt__c != null")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("like", func(t *testing.T) {
		res := soqlQuery(t, s, p, `SELECT Id FROM Item__c WHERE Txt__c LIKE 'al%'`)
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, `SELECT Id FROM Item__c WHERE Txt__c LIKE 'a\\%%'`)
		if res.TotalSize != 0 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("in and not in", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Stage__c IN ('Open','Closed')")
		if res.TotalSize != 3 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Txt__c NOT IN ('Alpha')")
		if res.TotalSize != 2 {
			t.Fatal("NOT IN should include null rows:", res.TotalSize)
		}
	})

	t.Run("includes and excludes", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Multi__c INCLUDES ('A;B')")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Multi__c INCLUDES ('C')")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Multi__c EXCLUDES ('A')")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("boolean precedence", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE (Txt__c = 'Alpha' OR Txt__c = 'Beta') AND Num__c = 10")
		if res.TotalSize != 1 {
			t.Fatal("parenthesized OR+AND:", res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE NOT (Txt__c = 'Alpha')")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
		err := soqlErr(t, s, p, "SELECT Id FROM Item__c WHERE Txt__c = 'Alpha' OR Txt__c = 'Beta' AND Num__c = 10")
		if err == nil {
			t.Fatal("mixed AND and OR without parens should be rejected")
		}
	})

	t.Run("date literals", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Day__c = 2026-06-15")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Day__c > 2026-01-01")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE At__c >= 2026-06-15T00:00:00Z")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("date literal TODAY with fixed clock via RunAST", func(t *testing.T) {
		now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
		clock := func() time.Time { return now }
		ctx := context.Background()
		tx, err := f.s.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", f.p.Tenant); err != nil {
			t.Fatal(err)
		}
		snap, err := metadata.Load(ctx, tx, f.p.Tenant)
		if err != nil {
			t.Fatal(err)
		}
		exec, err := policy.ForAPI(f.p, "65.0")
		if err != nil {
			t.Fatal(err)
		}
		res, err := soqleng.Run(ctx, tx, exec, snap, "SELECT Id FROM Item__c WHERE Day__c = TODAY", soqleng.Options{Now: clock})
		if err != nil {
			t.Fatal(err)
		}
		if res.TotalSize != 1 {
			t.Fatal("TODAY with fixed clock should match 2026-06-15:", res.TotalSize)
		}
	})

	t.Run("order by limit offset", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c ASC NULLS LAST LIMIT 10")
		if res.TotalSize != 3 {
			t.Fatal(res.TotalSize)
		}
		last := res.Records[2]
		if last["Txt__c"] != nil {
			t.Fatal("NULLS LAST should put null last:", last)
		}
		res = soqlQuery(t, s, p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c ASC NULLS FIRST LIMIT 10 OFFSET 1")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("parent path on contact", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT LastName, Account.Name FROM Contact")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		rec := res.Records[0]
		acc, ok := rec["Account"].(map[string]any)
		if !ok || acc["Name"] != "Acme" {
			t.Fatal(rec)
		}
	})

	t.Run("id filter normalization", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Id = '"+item1[:15]+"'")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
	})
}

func TestParentPathNulls(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()

	// Create a three-level chain: Item__c -> Parent__c -> Group__c -> Region__c
	// The typed fixture has Parent__c. Add Group__c and Region__c via metadata.
	// For simplicity, use standard Account -> Contact path which already exists.
	accWith, _ := s.Create(ctx, p, "Account", map[string]any{"Name": "HasContact"})
	accWithout, _ := s.Create(ctx, p, "Account", map[string]any{"Name": "NoContact"})
	_ = accWithout
	_, _ = s.Create(ctx, p, "Contact", map[string]any{"LastName": "WithAcc", "AccountId": accWith})
	_, _ = s.Create(ctx, p, "Contact", map[string]any{"LastName": "NoAcc"})

	t.Run("null parent at first level", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT LastName, Account.Name FROM Contact ORDER BY LastName")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
		withAcc := res.Records[0]
		noAcc := res.Records[1]
		if withAcc["LastName"] != "NoAcc" {
			t.Fatal("ORDER BY LastName should sort NoAcc first:", withAcc)
		}
		if noAcc["LastName"] != "WithAcc" {
			t.Fatal(noAcc)
		}
		if withAcc["Account"] != nil {
			t.Fatal("null parent should be nil:", withAcc["Account"])
		}
		acc, ok := noAcc["Account"].(map[string]any)
		if !ok || acc["Name"] != "HasContact" {
			t.Fatal(noAcc)
		}
	})

	t.Run("order by parent puts nulls first", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT LastName, Account.Name FROM Contact ORDER BY Account.Name")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
		if res.Records[0]["Account"] != nil {
			t.Fatal("null parent should sort first (ASC NULLS FIRST):", res.Records[0])
		}
	})

	t.Run("custom chain with null intermediate", func(t *testing.T) {
		// Item__c has Parent__c (lookup to Parent__c).
		// Create items with and without parents.
		parentID, _ := s.Create(ctx, p, "Parent__c", map[string]any{"Name": "P1"})
		_, _ = s.Create(ctx, p, "Item__c", map[string]any{"Txt__c": "WithParent", "Parent__c": parentID})
		_, _ = s.Create(ctx, p, "Item__c", map[string]any{"Txt__c": "NoParent"})

		res := soqlQuery(t, s, p, "SELECT Txt__c, Parent__r.Name FROM Item__c ORDER BY Txt__c")
		if res.TotalSize < 2 {
			t.Fatal(res.TotalSize)
		}
		var foundWith, foundWithout bool
		for _, rec := range res.Records {
			switch rec["Txt__c"] {
			case "WithParent":
				foundWith = true
				parent, ok := rec["Parent__r"].(map[string]any)
				if !ok || parent["Name"] != "P1" {
					t.Fatal(rec)
				}
			case "NoParent":
				foundWithout = true
				if rec["Parent__r"] != nil {
					t.Fatal("null parent should be nil:", rec["Parent__r"])
				}
			}
		}
		if !foundWith || !foundWithout {
			t.Fatal(res.Records)
		}
	})
}

func TestSOQLExistingBehavior(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()
	_, _ = s.Create(ctx, p, "Account", map[string]any{"Name": "Behavior"})
	_, _ = s.Create(ctx, p, "Contact", map[string]any{"LastName": "Behave", "FirstName": "Test"})

	queries := []string{
		"SELECT Id, Name FROM Account",
		"SELECT Id, Name FROM Account WHERE Name = 'Behavior'",
		"SELECT Id, Name FROM Account WHERE Name = 'behavior'",
		"SELECT Id, LastName FROM Contact",
		"SELECT Id, LastName FROM Contact WHERE LastName = 'Behave'",
		"SELECT Id, Name FROM Account ORDER BY Name ASC LIMIT 10",
		"SELECT Id, Name FROM Account ORDER BY Name DESC NULLS LAST LIMIT 10",
		"select id, name from account",
	}

	for _, qtext := range queries {
		t.Run(qtext, func(t *testing.T) {
			res := soqlQuery(t, s, p, qtext)
			// Verify shape: each record has attributes with type and url
			for _, rec := range res.Records {
				attrs, ok := rec["attributes"].(map[string]any)
				if !ok {
					t.Fatal("missing attributes:", rec)
				}
				if attrs["type"] == nil {
					t.Fatal("missing type:", attrs)
				}
				if id, hasID := rec["Id"].(string); hasID {
					url, hasURL := attrs["url"].(string)
					if !hasURL || !strings.Contains(url, id) {
						t.Fatal("missing or wrong url:", attrs)
					}
				}
			}
		})
	}
}

func TestSOQLCrossTenant(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()

	// Both tenants use standard Account/Contact.
	accA, err := s.Create(ctx, a, "Account", map[string]any{"Name": "AccA"})
	if err != nil {
		t.Fatal(err)
	}
	accB, err := s.Create(ctx, b, "Account", map[string]any{"Name": "AccB"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, a, "Contact", map[string]any{"LastName": "ContactA", "AccountId": accA})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, b, "Contact", map[string]any{"LastName": "ContactB", "AccountId": accB})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("parent from other tenant invisible", func(t *testing.T) {
		res := soqlQuery(t, s, a, "SELECT LastName, Account.Name FROM Contact WHERE LastName = 'ContactA'")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		acc, ok := res.Records[0]["Account"].(map[string]any)
		if !ok || acc["Name"] != "AccA" {
			t.Fatal(res.Records[0])
		}
	})

	t.Run("forged parent id filtered", func(t *testing.T) {
		res := soqlQuery(t, s, a, "SELECT LastName FROM Contact WHERE AccountId = '"+accB+"'")
		if res.TotalSize != 0 {
			t.Fatal("cross-tenant filter returned rows:", res.TotalSize)
		}
	})

	t.Run("other tenant cannot see tenant A rows", func(t *testing.T) {
		res := soqlQuery(t, s, b, "SELECT LastName FROM Contact WHERE LastName = 'ContactA'")
		if res.TotalSize != 0 {
			t.Fatal("cross-tenant read:", res.TotalSize)
		}
		res = soqlQuery(t, s, b, "SELECT Name FROM Account WHERE Name = 'AccA'")
		if res.TotalSize != 0 {
			t.Fatal("cross-tenant account read:", res.TotalSize)
		}
	})
}
