package platform

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestUpsertPlan9Sequences(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name, first, next string
		same              bool
	}{{"ascii", "AbC", "abc", true}, {"leading-space", "key", " key", true}, {"trailing-space", "key", "key ", true}, {"accent", "é", "e\u0301", true}, {"ascii-i", "I", "i", true}, {"dotless-i", "I", "ı", false}, {"sharp-s", "ß", "ss", true}, {"encoded", "/percent%é", "/percent%e\u0301", true}} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := principals(t, s)
			first, created, e := s.Upsert(ctx, p, tc.first, map[string]any{"Name": "First"})
			if e != nil || !created {
				t.Fatal(first, created, e)
			}
			next, created, e := s.Upsert(ctx, p, tc.next, map[string]any{"Name": "Second"})
			if e != nil || created == tc.same || (next == first) != tc.same {
				t.Fatal(next, created, e)
			}
			row, e := s.Read(ctx, p, "Account", first)
			if e != nil || row["Compat_Key__c"] != tc.first {
				t.Fatal(row, e)
			}
		})
	}
	for _, key := range []string{"AbC", "abc", "different"} {
		p, _ := principals(t, s)
		_, _, e := s.Upsert(ctx, p, "AbC", map[string]any{"Name": "Denied", "compat_key__c": key})
		assertUpsertBodyError(t, e, "INVALID_FIELD")
	}
}
func TestUpsertUpdateKeepsSpelling(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	id, _, e := s.Upsert(ctx, p, "AbC", map[string]any{"Name": "First"})
	if e != nil {
		t.Fatal(e)
	}
	next, created, e := s.Upsert(ctx, p, "abc", map[string]any{"Name": "Second"})
	if e != nil || created || next != id {
		t.Fatal(next, created, e)
	}
	row, e := s.Read(ctx, p, "Account", id)
	if e != nil || row["Compat_Key__c"] != "AbC" || row["Name"] != "Second" {
		t.Fatal(row, e)
	}
}
func TestUpsertWhitespaceOnlyKey(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	before := accountSnapshot(t, s, p)
	w := bodyValidationRequest(t, NewHandler(s, ""), "PATCH", apiBase+"sobjects/Account/Compat_Key__c/%20%20", `{"Name":"Blank"}`, "token-"+p.Tenant)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_FIELD") || accountSnapshot(t, s, p) != before {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestUpsertConcurrentVariants(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	type outcome struct {
		id, key string
		created bool
		err     error
	}
	results := make(chan outcome, 16)
	start := make(chan struct{})
	for i := range 16 {
		go func(i int) {
			key := []string{"AbC", "abc", " abc "}[i%3]
			<-start
			id, created, e := s.Upsert(ctx, p, key, map[string]any{"Name": "Race"})
			results <- outcome{id, key, created, e}
		}(i)
	}
	close(start)
	firstID, spelling := "", ""
	count := 0
	for range 16 {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		if firstID == "" {
			firstID = r.id
		}
		if r.id != firstID {
			t.Fatal("duplicate records")
		}
		if r.created {
			count++
			spelling = r.key
		}
	}
	row, e := s.Read(ctx, p, "Account", firstID)
	if e != nil || count != 1 || row["Compat_Key__c"] != spelling {
		t.Fatal(row, count, e)
	}
	rows, e := selectSQL(t, s, p, "SELECT Id FROM Account")
	if e != nil || len(rows) != 1 {
		t.Fatal(rows, e)
	}
}
func TestUpsertTenantIsolation(t *testing.T) {
	s := testStore(t)
	p, other := principals(t, s)
	ctx := context.Background()
	ids := map[string]bool{}
	for _, principal := range []Principal{p, other} {
		for i, key := range []string{"AbC", "abc", " abc "} {
			id, created, e := s.Upsert(ctx, principal, key, map[string]any{"Name": "Tenant"})
			if e != nil || created != (i == 0) {
				t.Fatal(created, e)
			}
			ids[id] = true
		}
	}
	if len(ids) != 2 {
		t.Fatal(ids)
	}
}
func TestUpsertRacingPost(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	for round := range 30 {
		key := fmt.Sprintf("race-%d", round)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for _, upsert := range []bool{true, false} {
			wg.Add(1)
			go func(upsert bool) {
				defer wg.Done()
				<-start
				var e error
				if upsert {
					_, _, e = s.Upsert(ctx, p, key, map[string]any{"Name": "Upsert"})
				} else {
					_, e = s.Create(ctx, p, "Account", map[string]any{"Name": "Post", "Compat_Key__c": strings.ToUpper(key)})
				}
				if e != nil && !strings.Contains(e.Error(), "DUPLICATE_VALUE") {
					t.Error(e)
				}
			}(upsert)
		}
		close(start)
		wg.Wait()
		rows, e := selectSQL(t, s, p, "SELECT Id FROM Account WHERE Compat_Key__c = '"+key+"'")
		if e != nil || len(rows) != 1 {
			t.Fatal(rows, e)
		}
	}
}
