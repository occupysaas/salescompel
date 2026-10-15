package platform

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL required in CI")
		}
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration")
	}
	s, err := Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
func principals(t *testing.T, s *Store) (Principal, Principal) {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	a := Principal{Tenant: "a" + suffix, User: "alice", Role: "admin"}
	b := Principal{Tenant: "b" + suffix, User: "bob", Role: "admin"}
	for _, p := range []Principal{a, b} {
		if err := s.Provision(context.Background(), "token-"+p.Tenant, p); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, err := s.pool.Exec(context.Background(), "DELETE FROM tenants WHERE id = ANY($1)", []string{a.Tenant, b.Tenant})
		if err != nil {
			t.Error(err)
		}
	})
	return a, b
}
func TestStoreIsolationAndPersistence(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()
	id, err := s.Create(ctx, a, "Account", map[string]any{"Name": "Acme", "Compat_Key__c": "key", "Compat_Text__c": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 18 {
		t.Fatal(id)
	}
	if _, err = s.Read(ctx, b, "Account", id); err == nil {
		t.Fatal("cross tenant read")
	}
	if err = s.Update(ctx, b, "Account", id, map[string]any{"Name": "stolen"}); err == nil {
		t.Fatal("cross tenant update")
	}
	if err = s.Delete(ctx, b, "Account", id); err == nil {
		t.Fatal("cross tenant delete")
	}
	if _, err = s.Create(ctx, b, "Contact", map[string]any{"LastName": "No", "AccountId": id}); err == nil {
		t.Fatal("cross tenant relationship")
	}
	if _, err = s.Create(ctx, b, "Account", map[string]any{"Name": "Other", "Compat_Key__c": "key"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, a, "Account", map[string]any{"Name": "Duplicate", "Compat_Key__c": "key"}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if err = s.Update(ctx, a, "Account", id, map[string]any{"Name": "Changed", "Bogus": "bad"}); err == nil {
		t.Fatal("unknown field")
	}
	row, err := s.Read(ctx, a, "Account", id)
	if err != nil || row["Name"] != "Acme" {
		t.Fatal(row, err)
	}
	if err = s.Update(ctx, a, "Account", id, map[string]any{"Compat_Text__c": nil}); err != nil {
		t.Fatal(err)
	}
	if err = s.Update(ctx, a, "Account", id, map[string]any{"Name": "Still here"}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	row, err = reopened.Read(ctx, a, "Account", id)
	if err != nil || row["Name"] != "Still here" || row["Compat_Text__c"] != nil {
		t.Fatal(row, err)
	}
	p, err := reopened.Authenticate(ctx, "token-"+a.Tenant)
	if err != nil || p != a {
		t.Fatal(p, err)
	}
	if _, err = s.Authenticate(ctx, "wrong"); err == nil {
		t.Fatal("bad token")
	}
}
func TestStoreQueryPolicyAndRelationships(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()
	id, err := s.Create(ctx, a, "Account", map[string]any{"Name": "Parent", "Compat_Text__c": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, a, "Contact", map[string]any{"LastName": "O'Brien", "AccountId": id})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := ParseQuery("SELECT Id,LastName,Account.Name FROM Contact WHERE LastName = 'O\\'Brien'")
	rows, err := s.Select(ctx, a, q)
	if err != nil || len(rows) != 1 || rows[0]["Account"].(map[string]any)["Name"] != "Parent" {
		t.Fatal(rows, err)
	}
	rows, err = s.Select(ctx, b, q)
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	restricted := a
	restricted.Role = "restricted"
	row, err := s.Read(ctx, restricted, "Account", id)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := row["Compat_Text__c"]; ok {
		t.Fatal("field leak")
	}
	for _, sql := range []string{"SELECT Compat_Text__c FROM Account", "SELECT Id FROM Account WHERE Compat_Text__c = 'secret'", "SELECT Id FROM Account ORDER BY Compat_Text__c", "SELECT Id FROM Bogus", "SELECT Wrong FROM Account"} {
		q, _ = ParseQuery(sql)
		if _, err = s.Select(ctx, restricted, q); err == nil {
			t.Fatal("query accepted", sql)
		}
	}
	if err = s.Delete(ctx, a, "Account", id); err == nil {
		t.Fatal("referenced parent deleted")
	}
}
func TestStoreConcurrentUpdatesAndUpsert(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	id, err := s.Create(ctx, a, "Account", map[string]any{"Name": "Before"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for k, v := range map[string]any{"Name": "After", "Compat_Text__c": "Together"} {
		wg.Add(1)
		go func(k string, v any) {
			defer wg.Done()
			if e := s.Update(ctx, a, "Account", id, map[string]any{k: v}); e != nil {
				t.Error(e)
			}
		}(k, v)
	}
	wg.Wait()
	row, err := s.Read(ctx, a, "Account", id)
	if err != nil || row["Name"] != "After" || row["Compat_Text__c"] != "Together" {
		t.Fatal(row, err)
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, e := s.Upsert(ctx, a, "concurrent", map[string]any{"Name": "One"}); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	q, _ := ParseQuery("SELECT Id FROM Account WHERE Compat_Key__c = 'concurrent'")
	rows, err := s.Select(ctx, a, q)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
}
func TestStoreBlankNillableStringsAreNull(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		id, err := s.Create(ctx, a, "Account", map[string]any{"Name": "Blank", "Compat_Key__c": "", "Compat_Text__c": ""})
		if err != nil {
			t.Fatal("blank external IDs must not collide", err)
		}
		row, err := s.Read(ctx, a, "Account", id)
		if err != nil || row["Compat_Key__c"] != nil || row["Compat_Text__c"] != nil {
			t.Fatal(row, err)
		}
	}
	id, err := s.Create(ctx, a, "Contact", map[string]any{"LastName": "Orphan", "AccountId": ""})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := ParseQuery("SELECT Id FROM Contact WHERE AccountId = null")
	rows, err := s.Select(ctx, a, q)
	if err != nil || len(rows) != 1 || rows[0]["Id"] != id {
		t.Fatal(rows, err)
	}
}
