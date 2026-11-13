package platform

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestLocatorRules(t *testing.T) {
	ctx := context.Background()

	t.Run("replay and foreign principals", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 201)
		first, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c", 200)
		if err != nil {
			t.Fatal(err)
		}
		if first.NextLocator == "" {
			t.Fatal("no locator")
		}
		a, err := f.s.QueryMore(ctx, f.p, first.NextLocator, 200)
		if err != nil {
			t.Fatal(err)
		}
		b, err := f.s.QueryMore(ctx, f.p, first.NextLocator, 200)
		if err != nil {
			t.Fatal(err)
		}
		if !pagesEqual(a, b) {
			t.Fatalf("replay differs:\n%#v\n%#v", a, b)
		}
		sameTenant := f.p
		sameTenant.User = "eve"
		otherTenant := f.other
		otherTenant.User = f.p.User
		for name, p := range map[string]Principal{"same tenant": sameTenant, "other tenant": otherTenant} {
			if _, err := f.s.QueryMore(ctx, p, first.NextLocator, 200); err == nil {
				t.Fatalf("%s locator accepted", name)
			} else if code, _ := apiCode(err); code != "INVALID_QUERY_LOCATOR" {
				t.Fatalf("%s code=%q", name, code)
			}
		}
	})

	t.Run("expired", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 201)
		first, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c", 200)
		if err != nil || first.NextLocator == "" {
			t.Fatal(first, err)
		}
		tenantExec(t, f, "UPDATE sc_query.locators SET expires_at=now()-interval '1 second' WHERE tenant_id=$1 AND id=$2",
			f.p.Tenant, locatorID(t, first.NextLocator))
		_, err = f.s.QueryMore(ctx, f.p, first.NextLocator, 200)
		if code, _ := apiCode(err); code != "INVALID_QUERY_LOCATOR" {
			t.Fatalf("expired code=%q err=%v", code, err)
		}
	})

	t.Run("eleventh evicts oldest", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 201)
		var locators []string
		for i := 0; i < 11; i++ {
			res, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c", 200)
			if err != nil || res.NextLocator == "" {
				t.Fatalf("query %d: %v %#v", i, err, res)
			}
			locators = append(locators, res.NextLocator)
			time.Sleep(2 * time.Millisecond)
		}
		if _, err := f.s.QueryMore(ctx, f.p, locators[0], 200); err == nil {
			t.Fatal("oldest locator survived eviction")
		} else if code, _ := apiCode(err); code != "INVALID_QUERY_LOCATOR" {
			t.Fatalf("evicted code=%q", code)
		}
		if _, err := f.s.QueryMore(ctx, f.p, locators[10], 200); err != nil {
			t.Fatalf("newest locator failed: %v", err)
		}
	})

	t.Run("too many ids", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 50001)
		_, err := f.s.Query(ctx, f.p, "SELECT Id FROM Item__c", 2000)
		if code, _ := apiCode(err); code != "QUERY_TOO_COMPLICATED" {
			t.Fatalf("code=%q err=%v", code, err)
		}
	})

	t.Run("unpageable queries create no locator", func(t *testing.T) {
		f := newTypedFixture(t)
		bulkItems(t, f, 201)
		for _, q := range []string{
			"SELECT COUNT() FROM Item__c",
			"SELECT Stage__c, COUNT(Id) FROM Item__c GROUP BY Stage__c",
			"SELECT Id FROM Item__c ORDER BY Txt__c LIMIT 500 OFFSET 1",
		} {
			res, err := f.s.Query(ctx, f.p, q, 200)
			if err != nil {
				t.Fatalf("%s: %v", q, err)
			}
			if res.NextLocator != "" {
				t.Fatalf("%s created locator %q", q, res.NextLocator)
			}
		}
		var n int
		if err := f.s.pool.QueryRow(ctx, "SELECT count(*) FROM sc_query.locators WHERE tenant_id=$1", f.p.Tenant).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("unpageable queries stored %d locators", n)
		}
	})

	t.Run("malformed path", func(t *testing.T) {
		f := newTypedFixture(t)
		for _, bad := range []string{"garbage", "01g" + strings.Repeat("A", 15), "01g" + strings.Repeat("A", 15) + "-", "01gshort-200"} {
			if _, err := f.s.QueryMore(ctx, f.p, bad, 200); err == nil {
				t.Fatalf("malformed %q accepted", bad)
			} else if code, _ := apiCode(err); code != "INVALID_QUERY_LOCATOR" {
				t.Fatalf("malformed %q code=%q", bad, code)
			}
		}
	})
}

func TestQueryLocatorsRLS(t *testing.T) {
	f := newTypedFixture(t)
	bulkItems(t, f, 201)
	ctx := context.Background()
	first, err := f.s.Query(ctx, f.p, "SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c", 200)
	if err != nil || first.NextLocator == "" {
		t.Fatal(first, err)
	}
	id := locatorID(t, first.NextLocator)

	count := func(tenant string) int {
		t.Helper()
		tx, err := f.s.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if tenant != "" {
			if _, err = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenant); err != nil {
				t.Fatal(err)
			}
		}
		// Exercise the same group role the runtime records login inherits.
		if _, err = tx.Exec(ctx, "SET LOCAL ROLE salescompel_records"); err != nil {
			t.Fatal(err)
		}
		var n int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM sc_query.locators WHERE id=$1", id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(""); n != 0 {
		t.Fatalf("records role without context saw %d locators", n)
	}
	if n := count(f.p.Tenant); n != 1 {
		t.Fatalf("records role saw %d own locators, want 1", n)
	}
	if n := count(f.other.Tenant); n != 0 {
		t.Fatalf("records role saw %d foreign locators", n)
	}
}

func TestLocatorHTTPBatchSize(t *testing.T) {
	f := newTypedFixture(t)
	bulkItems(t, f, 201)
	h := NewHandler(f.s, "")
	token := "token-" + f.p.Tenant
	locatorShape := regexp.MustCompile(`^/services/data/v65\.0/query/01g[A-Za-z0-9]{15}-[0-9]+$`)

	get := func(path, options string) (int, map[string]any) {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		if options != "" {
			r.Header.Set("Sforce-Query-Options", options)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var body any
		if w.Body.Len() > 0 {
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("non-json body %s", w.Body.String())
			}
		}
		// Errors use the REST envelope: a one-element array.
		if list, ok := body.([]any); ok {
			if len(list) != 1 {
				t.Fatalf("unexpected error body %#v", body)
			}
			body = list[0]
		}
		m, _ := body.(map[string]any)
		return w.Code, m
	}
	query := url.QueryEscape("SELECT Id, Txt__c FROM Item__c ORDER BY Txt__c")

	code, body := get(apiBase+"query?q="+query, "batchSize=100")
	if code != 200 {
		t.Fatalf("status %d: %#v", code, body)
	}
	rows := body["records"].([]any)
	if len(rows) != 200 {
		t.Fatalf("batchSize=100 served %d rows, want the 200 minimum", len(rows))
	}
	next, _ := body["nextRecordsUrl"].(string)
	if !locatorShape.MatchString(next) || !strings.HasSuffix(next, "-200") {
		t.Fatalf("nextRecordsUrl=%q", next)
	}
	if body["done"] != false {
		t.Fatalf("done=%v", body["done"])
	}
	code, body = get(next, "batchSize=5000")
	if code != 200 || len(body["records"].([]any)) != 1 || body["done"] != true {
		t.Fatalf("second page status=%d body=%#v", code, body)
	}
	if _, ok := body["nextRecordsUrl"]; ok {
		t.Fatalf("last page carried nextRecordsUrl: %#v", body)
	}

	// The maximum clamp: 2,001 rows at batchSize=5000 serve 2,000 first.
	big := newTypedFixture(t)
	bulkItems(t, big, 2001)
	h = NewHandler(big.s, "")
	token = "token-" + big.p.Tenant
	code, body = get(apiBase+"query?q="+query, "batchSize=5000")
	if code != 200 {
		t.Fatalf("status %d: %#v", code, body)
	}
	if rows := body["records"].([]any); len(rows) != 2000 {
		t.Fatalf("batchSize=5000 served %d rows, want the 2,000 maximum", len(rows))
	}
	next, _ = body["nextRecordsUrl"].(string)
	if !strings.HasSuffix(next, "-2000") {
		t.Fatalf("max nextRecordsUrl=%q", next)
	}

	code, body = get(apiBase+"query/not-a-locator", "")
	if code != 400 {
		t.Fatalf("malformed status=%d", code)
	}
	if body["errorCode"] != "INVALID_QUERY_LOCATOR" {
		t.Fatalf("malformed body=%#v", body)
	}
}
