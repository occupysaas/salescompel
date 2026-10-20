package platform

import (
	"context"
	"errors"
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

// The observed cases isolate the body key; array coverage is local robustness only.
func upsertBodyCases() []struct {
	name, code string
	present    bool
	value      any
} {
	return []struct {
		name, code string
		present    bool
		value      any
	}{
		{"omitted", "", false, nil},
		{"null", "", true, nil},
		{"identical", "INVALID_FIELD", true, "body-key"},
		{"variant", "INVALID_FIELD", true, "BODY-KEY"},
		{"different", "INVALID_FIELD", true, "other-key"},
		{"empty", "INVALID_FIELD", true, ""},
		{"number42", "INVALID_FIELD", true, float64(42)},
		{"object", "JSON_PARSER_ERROR", true, map[string]any{"nested": "value"}},
		{"array-local-robustness", "JSON_PARSER_ERROR", true, []any{"value"}},
	}
}

func TestUpsertRejectsBodyKeyWithoutDatabase(t *testing.T) {
	for _, tc := range upsertBodyCases() {
		if tc.code == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]any{"Name": "Requested", "Compat_Key__c": tc.value}
			before := stringJSON(data)
			defer func() {
				if v := recover(); v != nil {
					t.Errorf("body key validation panicked without a database: %v", v)
				}
				if after := stringJSON(data); after != before {
					t.Errorf("caller input changed: %s -> %s", before, after)
				}
			}()
			id, created, err := (&Store{}).Upsert(context.Background(), Principal{Role: "admin"}, "body-key", data)
			if id != "" || created {
				t.Errorf("rejected upsert returned id=%q created=%v", id, created)
			}
			assertUpsertBodyError(t, err, tc.code)
		})
	}
}

func assertUpsertBodyError(t *testing.T, err error, code string) {
	t.Helper()
	var api *APIError
	if !errors.As(err, &api) || api.Status != 400 || api.Code != code || len(api.Fields) != 0 {
		t.Fatalf("error = %#v; want 400 %s without fields", err, code)
	}
	wantMessage := "External ID value must be supplied in the upsert URL"
	if code == "JSON_PARSER_ERROR" {
		wantMessage = "External ID requires a scalar value or null"
	}
	if api.Message != wantMessage {
		t.Errorf("message = %q, want %q", api.Message, wantMessage)
	}
}

// Seed the peer's matching key and the target's different body key, so rejected
// writes cannot silently change either tenant or redirect to the body key.
func upsertBodyFixture(t *testing.T, s *Store, existing, secondTenant bool) (Principal, Principal, string) {
	t.Helper()
	p, peer := principals(t, s)
	if secondTenant {
		p, peer = peer, p
	}
	ctx := context.Background()
	for _, seed := range []struct {
		p   Principal
		key string
	}{{peer, "body-key"}, {peer, "other-key"}, {p, "other-key"}} {
		if _, err := s.Create(ctx, seed.p, "Account", map[string]any{"Name": "Untouched", "Compat_Key__c": seed.key, "Compat_Text__c": "Sentinel"}); err != nil {
			t.Fatal(err)
		}
	}
	var id string
	if existing {
		var err error
		id, err = s.Create(ctx, p, "Account", map[string]any{"Name": "Original", "Compat_Key__c": "body-key", "Compat_Text__c": "Retained"})
		if err != nil {
			t.Fatal(err)
		}
	}
	return p, peer, id
}

func accountSnapshot(t *testing.T, s *Store, p Principal) string {
	t.Helper()
	rows, err := s.Select(context.Background(), p, Query{Object: "Account", Fields: []string{"Id", "Name", "Compat_Key__c", "Compat_Text__c"}, Order: "Id", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return stringJSON(rows)
}

func assertUpsertBodySuccess(t *testing.T, s *Store, p Principal, id, oldID string, created, retained bool) {
	t.Helper()
	if id == "" || created != (oldID == "") || (oldID != "" && id != oldID) {
		t.Fatalf("upsert id=%q created=%v, prior id=%q", id, created, oldID)
	}
	row, err := s.Read(context.Background(), p, "Account", id)
	if err != nil || row["Name"] != "Requested" || row["Compat_Key__c"] != "body-key" {
		t.Fatalf("upsert row = %v, error %v", row, err)
	}
	if retained && row["Compat_Text__c"] != "Retained" {
		t.Errorf("unrelated field changed: %v", row)
	}
	rows, err := s.Select(context.Background(), p, Query{Object: "Account", Fields: []string{"Id"}, HasWhere: true, WhereField: "Compat_Key__c", WhereValue: stringPointer("body-key"), Limit: 100})
	if err != nil || len(rows) != 1 || rows[0]["Id"] != id {
		t.Fatalf("expected exactly one path-key row: %v, %v", rows, err)
	}
}

func stringPointer(v string) *string { return &v }

func TestStoreUpsertBodyValidation(t *testing.T) {
	s := testStore(t)
	for _, secondTenant := range []bool{false, true} {
		for _, existing := range []bool{false, true} {
			for _, tc := range upsertBodyCases() {
				t.Run(fmt.Sprintf("tenantB=%v/existing=%v/%s", secondTenant, existing, tc.name), func(t *testing.T) {
					p, peer, oldID := upsertBodyFixture(t, s, existing, secondTenant)
					before, peerBefore := accountSnapshot(t, s, p), accountSnapshot(t, s, peer)
					data := map[string]any{"Name": "Requested"}
					if tc.present {
						data["Compat_Key__c"] = tc.value
					}
					inputBefore := stringJSON(data)
					id, created, err := s.Upsert(context.Background(), p, "body-key", data)
					if stringJSON(data) != inputBefore {
						t.Error("caller input changed")
					}
					if tc.code == "" {
						if err != nil {
							t.Fatal(err)
						}
						assertUpsertBodySuccess(t, s, p, id, oldID, created, existing)
						if !existing {
							updatedID, updateCreated, err := s.Upsert(context.Background(), p, "body-key", data)
							if err != nil {
								t.Fatal(err)
							}
							assertUpsertBodySuccess(t, s, p, updatedID, id, updateCreated, false)
							if stringJSON(data) != inputBefore {
								t.Error("caller input changed on second upsert")
							}
						}
					} else {
						if id != "" || created {
							t.Errorf("rejected upsert returned id=%q created=%v", id, created)
						}
						if after := accountSnapshot(t, s, p); after != before {
							t.Errorf("rejected write changed rows: %s -> %s", before, after)
						}
						assertUpsertBodyError(t, err, tc.code)
					}
					if after := accountSnapshot(t, s, peer); after != peerBefore {
						t.Errorf("peer tenant changed: %s -> %s", peerBefore, after)
					}
				})
			}
		}
	}
}
