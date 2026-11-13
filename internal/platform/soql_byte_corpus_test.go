package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	soqleng "salescompel/internal/soql"
)

// byteCase is one committed wire-identity corpus query. Now selects the
// direct engine path with a fixed clock; zero Now goes through the HTTP route.
type byteCase struct {
	ID     string
	Query  string
	Now    time.Time
	Status int
}

type byteGolden struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

type byteFixture struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Expected      map[string]byteGolden `json:"expected"`
}

const byteCorpusPath = "testdata/query-byte-corpus.json"

func byteCorpusCases() []byteCase {
	fixed := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	return []byteCase{
		{ID: "txt-equality", Query: "SELECT Txt__c FROM Item__c WHERE Txt__c = 'Alpha'"},
		{ID: "txt-case-insensitive", Query: "SELECT Txt__c FROM Item__c WHERE Txt__c = 'alpha'"},
		{ID: "num-gte", Query: "SELECT Txt__c FROM Item__c WHERE Num__c >= 10 ORDER BY Txt__c ASC"},
		{ID: "bool-equality", Query: "SELECT Txt__c FROM Item__c WHERE Flag__c = true ORDER BY Txt__c ASC NULLS LAST"},
		{ID: "date-equality", Query: "SELECT Txt__c FROM Item__c WHERE Day__c = 2026-06-15"},
		{ID: "datetime-gte", Query: "SELECT Txt__c FROM Item__c WHERE At__c >= 2026-01-01T00:00:00Z ORDER BY Txt__c ASC"},
		{ID: "operator-precedence", Query: "SELECT Txt__c FROM Item__c WHERE (Txt__c = 'Alpha' OR Txt__c = 'Beta') AND Num__c = 10"},
		{ID: "equals-null", Query: "SELECT Txt__c FROM Item__c WHERE Txt__c = null"},
		{ID: "not-equals-null", Query: "SELECT Txt__c FROM Item__c WHERE Txt__c != null ORDER BY Txt__c ASC"},
		{ID: "in-list", Query: "SELECT Txt__c FROM Item__c WHERE Txt__c IN ('Alpha','Beta') ORDER BY Txt__c ASC"},
		{ID: "not-in-null-row", Query: "SELECT Txt__c FROM Item__c WHERE Txt__c NOT IN ('Alpha') ORDER BY Txt__c ASC NULLS LAST"},
		{ID: "like-prefix", Query: "SELECT Txt__c FROM Item__c WHERE Txt__c LIKE 'Al%'"},
		{ID: "like-escaped-percent", Query: `SELECT Txt__c FROM Item__c WHERE Txt__c LIKE 'a\\%%'`},
		{ID: "includes", Query: "SELECT Txt__c FROM Item__c WHERE Multi__c INCLUDES ('A') ORDER BY Txt__c ASC"},
		{ID: "excludes", Query: "SELECT Txt__c FROM Item__c WHERE Multi__c EXCLUDES ('A') ORDER BY Txt__c ASC NULLS LAST"},
		{ID: "order-asc-nulls-last", Query: "SELECT Txt__c FROM Item__c ORDER BY Txt__c ASC NULLS LAST"},
		{ID: "order-desc", Query: "SELECT Txt__c FROM Item__c ORDER BY Txt__c DESC"},
		{ID: "parent-path", Query: "SELECT Txt__c, Parent__r.Name FROM Item__c WHERE Txt__c = 'Alpha'"},
		{ID: "standard-parent-path", Query: "SELECT LastName, Account.Name FROM Contact WHERE LastName = 'Smith'"},
		{ID: "compound-person-name", Query: "SELECT Name FROM Contact WHERE LastName = 'Smith'"},
		{ID: "compound-address", Query: "SELECT MailingAddress FROM Contact WHERE LastName = 'Smith'"},
		{ID: "id-and-url", Query: "SELECT Id, Txt__c FROM Item__c WHERE Txt__c = 'Alpha'"},
		{ID: "fixed-clock-today", Query: "SELECT Txt__c FROM Item__c WHERE Day__c = TODAY ORDER BY Txt__c ASC", Now: fixed},
		{ID: "malformed", Query: "SELECT FROM Account", Status: 400},
		{ID: "unknown-object", Query: "SELECT Id FROM Bogus", Status: 400},
		{ID: "unknown-field", Query: "SELECT Missing FROM Account", Status: 400},
		{ID: "bad-id-filter", Query: "SELECT Id FROM Account WHERE Id = 'bad'", Status: 400},
	}
}

func TestSOQLByteCorpus(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()
	create := func(object string, data map[string]any) string {
		t.Helper()
		id, err := s.Create(ctx, p, object, data)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	parent := create("Parent__c", map[string]any{"Name": "P"})
	item1 := create("Item__c", map[string]any{
		"Txt__c": "Alpha", "Num__c": json.Number("42.5"), "Flag__c": true,
		"Day__c": "2026-06-15", "At__c": "2026-06-15T12:00:00Z",
		"Stage__c": "Closed", "Multi__c": "A;B", "Mail__c": "a@b.co", "Parent__c": parent,
	})
	item2 := create("Item__c", map[string]any{
		"Txt__c": "Beta", "Num__c": json.Number("10"), "Flag__c": false,
		"Day__c": "2026-01-01", "At__c": "2026-01-01T00:00:00Z",
		"Stage__c": "Open", "Multi__c": "C",
	})
	item3 := create("Item__c", map[string]any{
		"Txt__c": nil, "Num__c": nil, "Flag__c": true,
		"Day__c": "2026-12-25", "At__c": nil, "Stage__c": "Open", "Multi__c": nil,
	})
	account := create("Account", map[string]any{
		"Name": "Acme", "BillingStreet": "1 Main", "BillingCity": "Springfield",
		"BillingState": "IL", "BillingPostalCode": "62701", "BillingCountry": "US",
	})
	contact := create("Contact", map[string]any{
		"FirstName": "John", "LastName": "Smith", "AccountId": account,
		"MailingStreet": "9 Oak", "MailingCity": "Portland", "MailingState": "OR",
		"MailingPostalCode": "97201", "MailingCountry": "US",
	})
	unlinked := create("Contact", map[string]any{"LastName": "Unlinked"})

	fromGolden := strings.NewReplacer(
		"$parent", parent, "$item1", item1, "$item2", item2, "$item3", item3,
		"$account", account, "$contact", contact, "$unlinked", unlinked,
	)
	toGolden := strings.NewReplacer(
		parent, "$parent", item1, "$item1", item2, "$item2", item3, "$item3",
		account, "$account", contact, "$contact", unlinked, "$unlinked",
	)
	h := NewHandler(s, "")
	wire := func(c byteCase) (int, string) {
		t.Helper()
		if !c.Now.IsZero() {
			return 200, fixedClockBody(t, s, p, c.Query, c.Now)
		}
		r := httptest.NewRequest("GET", "/services/data/v65.0/query?q="+url.QueryEscape(fromGolden.Replace(c.Query)), nil)
		r.Header.Set("Authorization", "Bearer token-"+p.Tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}

	cases := byteCorpusCases()
	if os.Getenv("UPDATE_GOLDEN") != "" {
		got := map[string]byteGolden{}
		for _, c := range cases {
			status, body := wire(c)
			got[c.ID] = byteGolden{Status: status, Body: toGolden.Replace(body)}
		}
		raw, err := json.MarshalIndent(byteFixture{SchemaVersion: 1, Expected: got}, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(byteCorpusPath, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(byteCorpusPath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture byteFixture
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || len(fixture.Expected) != len(cases) {
		t.Fatalf("incomplete byte corpus: version=%d cases=%d/%d", fixture.SchemaVersion, len(fixture.Expected), len(cases))
	}
	for _, c := range cases {
		want, ok := fixture.Expected[c.ID]
		if !ok {
			t.Fatalf("missing byte corpus case %s", c.ID)
		}
		if c.Status != 0 && want.Status != c.Status {
			t.Fatalf("%s: fixture status %d, case pins %d", c.ID, want.Status, c.Status)
		}
		status, body := wire(c)
		wantBody := fromGolden.Replace(want.Body)
		if status != want.Status || body != wantBody {
			t.Fatalf("%s: status %d want %d\nbody %q\nwant %q", c.ID, status, want.Status, body, wantBody)
		}
	}
}

// fixedClockBody runs the direct engine path with a fixed clock and encodes
// the same envelope the query route emits.
func fixedClockBody(t *testing.T, s *Store, p Principal, text string, now time.Time) string {
	t.Helper()
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", p.Tenant); err != nil {
		t.Fatal(err)
	}
	snap, err := metadata.Load(ctx, tx, p.Tenant)
	if err != nil {
		t.Fatal(err)
	}
	exec, err := policy.ForAPI(p, "65.0")
	if err != nil {
		t.Fatal(err)
	}
	res, err := soqleng.Run(ctx, tx, exec, snap, text, soqleng.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(map[string]any{"totalSize": res.TotalSize, "done": res.Done, "records": res.Records}); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
