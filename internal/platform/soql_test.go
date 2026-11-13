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

// queryBehavior executes one query through the engine and reports the closed
// error code when it fails. The old differential oracle was retired with the
// bounded grammar in Task 9; the 35-case fixture pins byte-identical JSON.
func queryBehavior(t *testing.T, s *Store, p Principal, text string) (string, []byte) {
	t.Helper()
	res, err := s.Query(context.Background(), p, text, 2000)
	if err != nil {
		code, _ := apiCode(err)
		return code, nil
	}
	b, _ := json.Marshal(map[string]any{"totalSize": res.TotalSize, "done": res.Done, "records": res.Records})
	return "", b
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

	t.Run("comparison operators per kind", func(t *testing.T) {
		cases := []struct {
			query string
			count int
		}{
			{"SELECT Id FROM Item__c WHERE Txt__c = 'Alpha'", 1},
			{"SELECT Id FROM Item__c WHERE Txt__c = 'alpha'", 1},
			{"SELECT Id FROM Item__c WHERE Stage__c = 'closed'", 1},
			{"SELECT Id FROM Item__c WHERE Name = 'it-0001'", 1},
			{"SELECT Id FROM Item__c WHERE Txt__c != 'Alpha'", 2},
			{"SELECT Id FROM Item__c WHERE Txt__c LIKE 'al%'", 1},
			{"SELECT Id FROM Item__c WHERE Txt__c LIKE 'a\\%%'", 0},
			{"SELECT Id FROM Item__c WHERE Txt__c IN ('Alpha','Beta')", 2},
			{"SELECT Id FROM Item__c WHERE Txt__c NOT IN ('Alpha')", 2},
			{"SELECT Id FROM Item__c WHERE Txt__c = null", 1},
			{"SELECT Id FROM Item__c WHERE Txt__c != null", 2},
			{"SELECT Id FROM Item__c WHERE Num__c = 42.5", 1},
			{"SELECT Id FROM Item__c WHERE Num__c > 20", 1},
			{"SELECT Id FROM Item__c WHERE Num__c < 42.5", 1},
			{"SELECT Id FROM Item__c WHERE Num__c <= 10", 1},
			{"SELECT Id FROM Item__c WHERE Num__c >= 42.5", 1},
			{"SELECT Id FROM Item__c WHERE Num__c != 42.5", 2},
			{"SELECT Id FROM Item__c WHERE Flag__c = true", 2},
			{"SELECT Id FROM Item__c WHERE Flag__c = false", 1},
			{"SELECT Id FROM Item__c WHERE Day__c = 2026-06-15", 1},
			{"SELECT Id FROM Item__c WHERE Day__c > 2026-01-01", 2},
			{"SELECT Id FROM Item__c WHERE Day__c < 2026-06-15", 1},
			{"SELECT Id FROM Item__c WHERE Day__c <= 2026-06-15", 2},
			{"SELECT Id FROM Item__c WHERE Day__c >= 2026-06-15", 2},
			{"SELECT Id FROM Item__c WHERE Day__c != 2026-06-15", 2},
			{"SELECT Id FROM Item__c WHERE At__c = 2026-06-15T12:00:00Z", 1},
			{"SELECT Id FROM Item__c WHERE At__c > 2026-01-01T00:00:00Z", 1},
			{"SELECT Id FROM Item__c WHERE At__c < 2026-06-15T12:00:00Z", 1},
			{"SELECT Id FROM Item__c WHERE At__c != 2026-06-15T12:00:00Z", 2},
			{"SELECT Id FROM Item__c WHERE Stage__c = 'Open'", 2},
			{"SELECT Id FROM Item__c WHERE Stage__c != 'Open'", 1},
			{"SELECT Id FROM Item__c WHERE Stage__c IN ('Open')", 2},
			{"SELECT Id FROM Item__c WHERE Stage__c NOT IN ('Open')", 1},
			{"SELECT Id FROM Item__c WHERE Multi__c INCLUDES ('A')", 1},
			{"SELECT Id FROM Item__c WHERE Multi__c EXCLUDES ('A')", 2},
			{"SELECT Id FROM Item__c WHERE Multi__c INCLUDES ('A','C')", 2},
			{"SELECT Id FROM Item__c WHERE Multi__c INCLUDES ('A;B','C')", 2},
			{"SELECT Id FROM Item__c WHERE Mail__c = 'a@b.co'", 1},
			{"SELECT Id FROM Item__c WHERE Mail__c LIKE 'a@%'", 1},
		}
		for _, tc := range cases {
			res := soqlQuery(t, s, p, tc.query)
			if res.TotalSize != tc.count {
				t.Fatalf("%s: got %d want %d", tc.query, res.TotalSize, tc.count)
			}
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
		if err := soqlErr(t, s, p, "SELECT Id FROM Item__c WHERE Txt__c = 'Alpha' OR Txt__c = 'Beta' AND Num__c = 10"); err == nil {
			t.Fatal("mixed AND/OR without parens should be rejected")
		}
	})

	t.Run("order by limit offset", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c ASC NULLS LAST LIMIT 10")
		if res.TotalSize != 3 {
			t.Fatal(res.TotalSize)
		}
		if res.Records[2]["Txt__c"] != nil {
			t.Fatal("NULLS LAST should put null last:", res.Records[2])
		}
		res = soqlQuery(t, s, p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c ASC NULLS FIRST LIMIT 10 OFFSET 1")
		if res.TotalSize != 2 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("multi-key order by", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id, Stage__c, Txt__c FROM Item__c ORDER BY Stage__c ASC, Txt__c ASC NULLS LAST")
		if res.TotalSize != 3 {
			t.Fatal(res.TotalSize)
		}
		if res.Records[0]["Stage__c"] != "Closed" {
			t.Fatal("first should be Closed:", res.Records[0])
		}
	})

	t.Run("parent path on contact", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT LastName, Account.Name FROM Contact")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		acc, ok := res.Records[0]["Account"].(map[string]any)
		if !ok || acc["Name"] != "Acme" {
			t.Fatal(res.Records[0])
		}
	})

	t.Run("id filter normalization", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE Id = '"+item1[:15]+"'")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
	})

	t.Run("date literal TODAY with fixed clock", func(t *testing.T) {
		now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
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
		res, err := soqleng.Run(ctx, tx, exec, snap, "SELECT Id FROM Item__c WHERE Day__c = TODAY", soqleng.Options{Now: func() time.Time { return now }})
		if err != nil {
			t.Fatal(err)
		}
		if res.TotalSize != 1 {
			t.Fatal("TODAY with fixed clock should match 2026-06-15:", res.TotalSize)
		}
	})
}

func TestParentPathNulls(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()

	groupObj, err := metadata.CreateObject(ctx, s.pool, p.Tenant, metadata.ObjectInput{Name: "Group__c", Label: "Group", PluralLabel: "Groups"})
	if err != nil {
		t.Fatal(err)
	}
	regionObj, err := metadata.CreateObject(ctx, s.pool, p.Tenant, metadata.ObjectInput{Name: "Region__c", Label: "Region", PluralLabel: "Regions"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+groupObj.TableSQL()+" CASCADE")
		s.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+regionObj.TableSQL()+" CASCADE")
	})
	if _, err = metadata.CreateField(ctx, s.pool, p.Tenant, metadata.FieldInput{Object: "Parent__c", Name: "Group__c", Label: "Group__c", Type: "Lookup", ReferenceTo: "Group__c"}); err != nil {
		t.Fatal(err)
	}
	if _, err = metadata.CreateField(ctx, s.pool, p.Tenant, metadata.FieldInput{Object: "Group__c", Name: "Region__c", Label: "Region__c", Type: "Lookup", ReferenceTo: "Region__c"}); err != nil {
		t.Fatal(err)
	}

	regionID, err := s.Create(ctx, p, "Region__c", map[string]any{"Name": "West"})
	if err != nil {
		t.Fatal(err)
	}
	groupWithRegion, err := s.Create(ctx, p, "Group__c", map[string]any{"Name": "G1", "Region__c": regionID})
	if err != nil {
		t.Fatal(err)
	}
	groupNoRegion, err := s.Create(ctx, p, "Group__c", map[string]any{"Name": "G2"})
	if err != nil {
		t.Fatal(err)
	}
	parentWithGroup, err := s.Create(ctx, p, "Parent__c", map[string]any{"Name": "P1", "Group__c": groupWithRegion})
	if err != nil {
		t.Fatal(err)
	}
	parentNoGroup, err := s.Create(ctx, p, "Parent__c", map[string]any{"Name": "P2", "Group__c": groupNoRegion})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = s.Create(ctx, p, "Item__c", map[string]any{"Txt__c": "FullChain", "Parent__c": parentWithGroup}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, p, "Item__c", map[string]any{"Txt__c": "NullRegion", "Parent__c": parentNoGroup}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, p, "Item__c", map[string]any{"Txt__c": "NoParent"}); err != nil {
		t.Fatal(err)
	}

	t.Run("three-level chain", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Txt__c, Parent__r.Group__r.Region__r.Name FROM Item__c ORDER BY Txt__c")
		if res.TotalSize != 3 {
			t.Fatal(res.TotalSize)
		}
		byKey := map[string]map[string]any{}
		for _, rec := range res.Records {
			byKey[rec["Txt__c"].(string)] = rec
		}
		full := byKey["FullChain"]
		parent, ok := full["Parent__r"].(map[string]any)
		if !ok {
			t.Fatal("FullChain missing Parent__r:", full)
		}
		grp, ok := parent["Group__r"].(map[string]any)
		if !ok {
			t.Fatal("FullChain missing Group__r:", parent)
		}
		reg, ok := grp["Region__r"].(map[string]any)
		if !ok || reg["Name"] != "West" {
			t.Fatal("FullChain missing Region__r.Name:", grp)
		}
		nr := byKey["NullRegion"]
		p2, ok := nr["Parent__r"].(map[string]any)
		if !ok {
			t.Fatal("NullRegion missing Parent__r:", nr)
		}
		g2, ok := p2["Group__r"].(map[string]any)
		if !ok {
			t.Fatal("NullRegion missing Group__r:", p2)
		}
		if g2["Region__r"] != nil {
			t.Fatal("NullRegion Region__r should be nil:", g2)
		}
		if byKey["NoParent"]["Parent__r"] != nil {
			t.Fatal("NoParent Parent__r should be nil:", byKey["NoParent"])
		}
	})

	t.Run("order by parent nulls first", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Txt__c, Parent__r.Name FROM Item__c ORDER BY Parent__r.Name")
		if res.TotalSize != 3 {
			t.Fatal(res.TotalSize)
		}
		if res.Records[0]["Parent__r"] != nil {
			t.Fatal("null parent should sort first (ASC NULLS FIRST):", res.Records[0])
		}
	})

	t.Run("standard contact null order", func(t *testing.T) {
		accWith, _ := s.Create(ctx, p, "Account", map[string]any{"Name": "HasContact"})
		_, _ = s.Create(ctx, p, "Contact", map[string]any{"LastName": "WithAcc", "AccountId": accWith})
		_, _ = s.Create(ctx, p, "Contact", map[string]any{"LastName": "NoAcc"})
		res := soqlQuery(t, s, p, "SELECT LastName, Account.Name FROM Contact ORDER BY Account.Name")
		if res.TotalSize < 2 {
			t.Fatal(res.TotalSize)
		}
		if res.Records[0]["Account"] != nil {
			t.Fatal("null parent should sort first:", res.Records[0])
		}
	})

	t.Run("forged parent id yields null path", func(t *testing.T) {
		// Create Parent__c metadata for f.other so we get a real foreign ID.
		obj, err := metadata.CreateObject(ctx, s.pool, f.other.Tenant, metadata.ObjectInput{Name: "Parent__c", Label: "Parent", PluralLabel: "Parents"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+obj.TableSQL()+" CASCADE") })
		foreignParent, err := s.Create(ctx, f.other, "Parent__c", map[string]any{"Name": "Foreign"})
		if err != nil {
			t.Fatal(err)
		}
		// Create item with a valid parent, then swap the parent_id to the
		// foreign ID via raw SQL (bypassing FK) so the path must come back null.
		validParent, err := s.Create(ctx, p, "Parent__c", map[string]any{"Name": "Valid"})
		if err != nil {
			t.Fatal(err)
		}
		itemID, err := s.Create(ctx, p, "Item__c", map[string]any{"Txt__c": "Forged", "Parent__c": validParent})
		if err != nil {
			t.Fatal(err)
		}
		parentCol := f.cols["Item__c.Parent__c"]
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, "SET LOCAL session_replication_role = 'replica'"); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "UPDATE "+f.items.TableSQL()+" SET \""+parentCol+"\" = $1 WHERE tenant_id = $2 AND id = $3", foreignParent, p.Tenant, itemID); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		res := soqlQuery(t, s, p, "SELECT Txt__c, Parent__r.Name FROM Item__c WHERE Txt__c = 'Forged'")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		if res.Records[0]["Parent__r"] != nil {
			t.Fatal("cross-tenant parent should yield null path:", res.Records[0])
		}
	})
}

func TestSOQLExistingBehavior(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()

	name := "Plan16 oracle"
	accID, err := s.Create(ctx, a, "Account", map[string]any{"Name": name, "Compat_Key__c": "owned-key"})
	if err != nil {
		t.Fatal(err)
	}
	contactID, err := s.Create(ctx, a, "Contact", map[string]any{"LastName": "Contact", "AccountId": accID})
	if err != nil {
		t.Fatal(err)
	}
	nullContactID, err := s.Create(ctx, a, "Contact", map[string]any{"LastName": "Unlinked"})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = s.Create(ctx, a, "Account", map[string]any{"Name": "MixedCase"})
	_, _ = s.Create(ctx, a, "Account", map[string]any{"Name": "\u00e9"})
	_, _ = s.Create(ctx, a, "Account", map[string]any{"Name": "I"})
	// Accounts with varying Compat_Text__c for null-ordering replay.
	_, _ = s.Create(ctx, a, "Account", map[string]any{"Name": "T1", "Compat_Text__c": "alpha"})
	_, _ = s.Create(ctx, a, "Account", map[string]any{"Name": "T2", "Compat_Text__c": "beta"})
	_, _ = s.Create(ctx, a, "Account", map[string]any{"Name": "T3"})
	_, _ = s.Create(ctx, b, "Account", map[string]any{"Name": name})

	rp := strings.NewReplacer("$accountId", accID, "$contactId", contactID, "$nullContactId", nullContactID, "$name", name)

	queries := []string{
		// foundation_query_test.go executed queries
		"select id, name from account",
		"select account.name from contact",
		"select id from account where name = 'MixedCase'",
		"select id from account where name = 'mixedcase'",
		"select id from account where name = 'I'",
		"select id from account where id = '" + accID[:15] + "'",
		"select id from contact where accountid = '" + accID[:15] + "'",
		"select id from account where name = 'absent'",
		// foundation_query_test.go null-ordering queries
		"select id, compat_text__c from account order by compat_text__c ASC",
		"select id, compat_text__c from account order by compat_text__c ASC NULLS FIRST",
		"select id, compat_text__c from account order by compat_text__c ASC NULLS LAST",
		"select id, compat_text__c from account order by compat_text__c DESC",
		"select id, compat_text__c from account order by compat_text__c DESC NULLS FIRST",
		"select id, compat_text__c from account order by compat_text__c DESC NULLS LAST",
		// query_test.go
		"SELECT Id,Account.Name FROM Contact WHERE LastName = 'Contact' ORDER BY LastName DESC LIMIT 2",
		// column fixture successful cases
		"SELECT Id,Name,Compat_Text__c,Compat_Key__c FROM Account WHERE Id = '" + accID + "'",
		"SELECT Id,FirstName,LastName,AccountId FROM Contact WHERE Id = '" + contactID + "'",
		"SELECT Id,Name FROM Account WHERE Name = 'absent'",
		"SELECT Id,AccountId FROM Contact WHERE LastName = 'absent'",
		"SELECT Name,Id,Name FROM Account WHERE Id = '" + accID + "'",
		"SELECT Compat_Text__c FROM Account WHERE Id = '" + accID + "'",
		"SELECT Compat_Key__c FROM Account WHERE Id = '" + accID + "'",
		"SELECT FirstName,AccountId FROM Contact WHERE Id = '" + nullContactID + "'",
		"SELECT Id,Name FROM Account WHERE Id = '" + accID + "'",
		"SELECT Id,Account.Name FROM Contact WHERE Id = '" + contactID + "'",
		"SELECT Id,Account.Name FROM Contact WHERE Id = '" + nullContactID + "'",
		"SELECT name FROM Account WHERE Id = '" + accID + "'",
	}
	errQueries := []string{
		"SELECT Id FROM Account WHERE Id = 'bad'",
		"SELECT Id FROM Nope",
		"SELECT Missing FROM Account",
		"SELECT FROM Account",
	}
	for _, qtext := range queries {
		t.Run(qtext[:min(40, len(qtext))], func(t *testing.T) {
			if code, _ := queryBehavior(t, s, a, rp.Replace(qtext)); code != "" {
				t.Fatalf("query %q failed with %s", qtext, code)
			}
		})
	}
	wantErr := map[string]string{
		"SELECT Id FROM Account WHERE Id = 'bad'": "INVALID_QUERY_FILTER_OPERATOR",
		"SELECT Id FROM Nope":                     "INVALID_TYPE",
		"SELECT Missing FROM Account":             "INVALID_FIELD",
		"SELECT FROM Account":                     "MALFORMED_QUERY",
	}
	for _, qtext := range errQueries {
		t.Run(qtext[:min(40, len(qtext))], func(t *testing.T) {
			code, _ := queryBehavior(t, s, a, rp.Replace(qtext))
			if code != wantErr[qtext] {
				t.Fatalf("query %q error code = %q, want %q", qtext, code, wantErr[qtext])
			}
		})
	}
	t.Run("cross-tenant-empty", func(t *testing.T) {
		code, body := queryBehavior(t, s, b, "SELECT Id,Name FROM Account WHERE Id = '"+accID+"'")
		if code != "" {
			t.Fatalf("cross-tenant query failed with %s", code)
		}
		var result struct{ TotalSize int }
		if err := json.Unmarshal(body, &result); err != nil || result.TotalSize != 0 {
			t.Fatalf("cross-tenant result = %s (%v)", body, err)
		}
	})
}

func TestSOQLCrossTenant(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()

	accA, err := s.Create(ctx, a, "Account", map[string]any{"Name": "AccA"})
	if err != nil {
		t.Fatal(err)
	}
	accB, err := s.Create(ctx, b, "Account", map[string]any{"Name": "AccB"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, a, "Contact", map[string]any{"LastName": "ContactA", "AccountId": accA}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, b, "Contact", map[string]any{"LastName": "ContactB", "AccountId": accB}); err != nil {
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
		if res := soqlQuery(t, s, a, "SELECT LastName FROM Contact WHERE AccountId = '"+accB+"'"); res.TotalSize != 0 {
			t.Fatal("cross-tenant filter returned rows:", res.TotalSize)
		}
	})

	t.Run("other tenant cannot see tenant A rows", func(t *testing.T) {
		if res := soqlQuery(t, s, b, "SELECT LastName FROM Contact WHERE LastName = 'ContactA'"); res.TotalSize != 0 {
			t.Fatal("cross-tenant read:", res.TotalSize)
		}
		if res := soqlQuery(t, s, b, "SELECT Name FROM Account WHERE Name = 'AccA'"); res.TotalSize != 0 {
			t.Fatal("cross-tenant account read:", res.TotalSize)
		}
	})
}

func TestParentPathCompoundAndAntiJoinNull(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()

	acc1, err := s.Create(ctx, p, "Account", map[string]any{"Name": "Acme", "BillingCity": "Lyon"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, p, "Account", map[string]any{"Name": "Beta"}); err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, p, "Contact", map[string]any{"LastName": "Smith", "AccountId": acc1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, p, "Contact", map[string]any{"LastName": "NullAcc"}); err != nil {
		t.Fatal(err)
	}

	t.Run("parent path compound field no panic", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT LastName, Account.BillingAddress FROM Contact WHERE LastName = 'Smith'")
		if res.TotalSize != 1 {
			t.Fatal(res.TotalSize)
		}
		acc, ok := res.Records[0]["Account"].(map[string]any)
		if !ok {
			t.Fatal("expected Account nested object:", res.Records[0])
		}
		addr, ok := acc["BillingAddress"].(map[string]any)
		if !ok || addr["city"] != "Lyon" {
			t.Fatal("expected BillingAddress shaped:", acc["BillingAddress"])
		}
	})

	t.Run("anti join null ref row", func(t *testing.T) {
		// Anti-join: accounts NOT referenced by any contact. Beta has no contacts.
		// Contact NullAcc has NULL AccountId — must not poison the anti-join.
		res := soqlQuery(t, s, p, "SELECT Name FROM Account WHERE Id NOT IN (SELECT AccountId FROM Contact)")
		if res.TotalSize != 1 {
			t.Fatal("anti-join should return 1 unreferenced account:", res.TotalSize)
		}
		if res.Records[0]["Name"] != "Beta" {
			t.Fatal("expected Beta:", res.Records[0])
		}
	})
}
