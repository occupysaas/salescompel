package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func sub31(method, path, ref string, body map[string]any) CompositeSubrequest {
	return CompositeSubrequest{Method: method, URL: apiBase + path, ReferenceID: ref, Body: body}
}
func partial(subs ...CompositeSubrequest) CompositeRequest {
	return CompositeRequest{CompositeRequest: subs}
}
func entryCode(t *testing.T, e CompositeSubresponse) string {
	t.Helper()
	list, ok := e.Body.([]*APIError)
	if !ok || len(list) != 1 {
		t.Fatalf("not an error entry: %#v", e)
	}
	return list[0].Code
}
func countNamed(t *testing.T, s *Store, p Principal, name string) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(context.Background(), "SELECT count(*) FROM accounts WHERE tenant_id=$1 AND name=$2", p.Tenant, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCompositePartialSuccess(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	out, err := s.Composite(ctx, a, partial(
		sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "P31 parent"}),
		sub31("POST", "sobjects/Contact", "bad", map[string]any{"Bogus": true}),
		sub31("POST", "sobjects/Contact", "dependent", map[string]any{"LastName": "Never", "AccountId": "@{bad.id}"}),
		sub31("POST", "sobjects/Contact", "ok", map[string]any{"LastName": "Kept", "AccountId": "@{acct.id}"}),
	))
	if err != nil {
		t.Fatal(err)
	}
	r := out.CompositeResponse
	if r[0].HTTPStatusCode != 201 || r[1].HTTPStatusCode != 400 || entryCode(t, r[1]) != "INVALID_FIELD" ||
		r[2].HTTPStatusCode != 400 || entryCode(t, r[2]) != "PROCESSING_HALTED" || r[3].HTTPStatusCode != 201 {
		t.Fatalf("statuses: %+v", r)
	}
	for _, e := range r {
		if len(e.HTTPHeaders) != 0 {
			t.Fatal("httpHeaders must stay empty")
		}
	}
	contact := r[3].Body.(map[string]any)["id"].(string)
	row, err := s.Read(ctx, a, "Contact", contact)
	if err != nil || row["AccountId"] != r[0].Body.(map[string]any)["id"] {
		t.Fatal("independent subrequests not committed", err)
	}
	if countNamed(t, s, a, "P31 parent") != 1 {
		t.Fatal("parent not committed")
	}
}

func TestCompositePartialRecovery(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	// A real foreign-key violation inside a savepoint leaves the transaction usable.
	out, err := s.Composite(ctx, a, partial(
		sub31("POST", "sobjects/Contact", "fk", map[string]any{"LastName": "BadFK", "AccountId": "001000000000000AAA"}),
		sub31("POST", "sobjects/Account", "after", map[string]any{"Name": "P31 after FK"}),
	))
	if err != nil || entryCode(t, out.CompositeResponse[0]) != "INVALID_CROSS_REFERENCE_KEY" || out.CompositeResponse[1].HTTPStatusCode != 201 {
		t.Fatalf("recovery: %v %+v", err, out)
	}
	if countNamed(t, s, a, "P31 after FK") != 1 {
		t.Fatal("write after recovered savepoint lost")
	}
	// A savepoint that cannot be opened is a storage failure: outer 500, nothing kept.
	snapshot, err := s.Catalog(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	a.Catalog = &snapshot
	begin := func(ctx context.Context) (pgx.Tx, error) {
		tx, e := s.pool.Begin(ctx)
		return &failingSavepointTx{Tx: tx, after: 1}, e
	}
	res, err := executeComposite(ctx, s, a, partial(
		sub31("POST", "sobjects/Account", "one", map[string]any{"Name": "P31 storage"}),
		sub31("POST", "sobjects/Account", "two", map[string]any{"Name": "P31 storage"}),
	), begin)
	var api *APIError
	if !errors.As(err, &api) || api.Status != 500 || len(res.CompositeResponse) != 0 || countNamed(t, s, a, "P31 storage") != 0 {
		t.Fatalf("storage failure: %v %+v", err, res)
	}
}

// failingSavepointTx fails every savepoint after the first `after`.
type failingSavepointTx struct {
	pgx.Tx
	after, opened int
}

func (t *failingSavepointTx) Begin(ctx context.Context) (pgx.Tx, error) {
	if t.opened++; t.opened > t.after {
		return nil, errors.New("private_marker")
	}
	return t.Tx.Begin(ctx)
}

func TestCompositeReadQueryUpsert(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	key := fmt.Sprintf("p31-%s", a.Tenant)
	out, err := s.Composite(ctx, a, partial(
		sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "P31 read"}),
		sub31("GET", "sobjects/Account/@{acct.id}?fields=Name", "read", nil),
		sub31("GET", "query?q=SELECT+Id,+Name+FROM+Account+WHERE+Id+=+'@{acct.id}'", "q", nil),
		sub31("PATCH", "sobjects/Account/Compat_Key__c/"+key, "up", map[string]any{"Name": "P31 upsert"}),
		sub31("POST", "sobjects/Contact", "c", map[string]any{"LastName": "@{read.Name}", "AccountId": "@{q.records[0].Id}"}),
		sub31("PATCH", "sobjects/Account/Compat_Key__c/"+key, "up2", map[string]any{"Name": "P31 upsert again"}),
	))
	if err != nil {
		t.Fatal(err)
	}
	r := out.CompositeResponse
	for i, want := range []int{201, 200, 200, 201, 201, 200} {
		if r[i].HTTPStatusCode != want {
			t.Fatalf("entry %d: %d %#v", i, r[i].HTTPStatusCode, r[i].Body)
		}
	}
	id := r[0].Body.(map[string]any)["id"].(string)
	standalone, err := s.Read(ctx, a, "Account", id, "Name")
	if err != nil || stringJSON(standalone) != stringJSON(r[1].Body) {
		t.Fatalf("read body differs: %v %s", err, stringJSON(r[1].Body))
	}
	q := r[2].Body.(map[string]any)
	if q["totalSize"] != 1 || q["done"] != true {
		t.Fatalf("query body: %v", q)
	}
	up := r[3].Body.(map[string]any)
	if up["created"] != true || up["success"] != true {
		t.Fatalf("upsert body: %v", up)
	}
	if r[5].Body.(map[string]any)["created"] != false || r[5].Body.(map[string]any)["id"] != up["id"] {
		t.Fatal("second upsert must update the first")
	}
	row, err := s.Read(ctx, a, "Contact", r[4].Body.(map[string]any)["id"].(string))
	if err != nil || row["LastName"] != "P31 read" || row["AccountId"] != id {
		t.Fatalf("references from read and query: %v %v", err, row)
	}
}

func TestCompositeAllOrNoneMixed(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	existing, err := s.Create(ctx, a, "Account", map[string]any{"Name": "P31 existing"})
	if err != nil {
		t.Fatal(err)
	}
	key := "p31-aon-" + a.Tenant
	req := partial(
		sub31("GET", "sobjects/Account/"+existing, "read", nil),
		sub31("GET", "query?q=SELECT+Id+FROM+Account", "q", nil),
		sub31("PATCH", "sobjects/Account/Compat_Key__c/"+key, "up", map[string]any{"Name": "P31 aon upsert"}),
		sub31("PATCH", "sobjects/Account/"+existing, "upd", map[string]any{"Name": "P31 changed"}),
		sub31("POST", "sobjects/Account", "bad", map[string]any{"Bogus": 1}),
	)
	req.AllOrNone = true
	out, err := s.Composite(ctx, a, req)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range out.CompositeResponse {
		want := "PROCESSING_HALTED"
		if i == 4 {
			want = "INVALID_FIELD"
		}
		if e.HTTPStatusCode != 400 || entryCode(t, e) != want {
			t.Fatalf("entry %d: %+v", i, e)
		}
	}
	row, _ := s.Read(ctx, a, "Account", existing)
	if row["Name"] != "P31 existing" || countNamed(t, s, a, "P31 aon upsert") != 0 {
		t.Fatal("all-or-none left effects")
	}
}

func TestCompositeRefReroute(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	tricky := "x/../../query?q=SELECT"
	out, err := s.Composite(ctx, a, partial(
		sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": tricky}),
		sub31("GET", "sobjects/Account/@{acct.id}?fields=Name", "r", nil),
		sub31("GET", "sobjects/Account/@{r.Name}", "read", nil),
		sub31("DELETE", "sobjects/Account/@{r.Name}", "del", nil),
		sub31("PATCH", "sobjects/Account/Compat_Key__c/@{r.Name}", "up", map[string]any{"Name": "P31 tricky key"}),
	))
	if err != nil {
		t.Fatal(err)
	}
	r := out.CompositeResponse
	if r[2].HTTPStatusCode != 404 || r[3].HTTPStatusCode != 404 || r[4].HTTPStatusCode != 201 {
		t.Fatalf("reroute: %+v", r)
	}
	var stored string
	if err := s.pool.QueryRow(ctx, "SELECT compat_key FROM accounts WHERE tenant_id=$1 AND name='P31 tricky key'", a.Tenant).Scan(&stored); err != nil || stored != tricky {
		t.Fatalf("external ID must be the literal value: %q %v", stored, err)
	}
}

func TestCompositeRefQueryScope(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()
	if _, err := s.Create(ctx, b, "Account", map[string]any{"Name": "P31 other tenant"}); err != nil {
		t.Fatal(err)
	}
	// The referenced value closes the quote and widens the filter.
	out, err := s.Composite(ctx, a, partial(
		sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "x' OR Name != '"}),
		sub31("GET", "sobjects/Account/@{acct.id}?fields=Name", "r", nil),
		sub31("GET", "query?q=SELECT+Name+FROM+Account+WHERE+Name+=+'@{r.Name}'", "q", nil),
	))
	if err != nil || out.CompositeResponse[2].HTTPStatusCode != 200 {
		t.Fatalf("%v %+v", err, out)
	}
	if strings.Contains(stringJSON(out.CompositeResponse[2].Body), "P31 other tenant") {
		t.Fatal("substituted query crossed tenants")
	}
	// A restricted caller cannot read a denied field through a substituted query.
	p := a
	p.Role = "restricted"
	out, err = s.Composite(ctx, p, partial(
		sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "Compat_Text__c"}),
		sub31("GET", "sobjects/Account/@{acct.id}?fields=Name", "r", nil),
		sub31("GET", "query?q=SELECT+Id,+@{r.Name}+FROM+Account", "q", nil),
	))
	if err != nil || out.CompositeResponse[2].HTTPStatusCode != 400 || strings.Contains(stringJSON(out.CompositeResponse[2].Body), "secret") {
		t.Fatalf("restricted field through reference: %v %+v", err, out.CompositeResponse[2])
	}
}

func TestCompositeLimitsAndHeaders(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()
	queries := func(n, reads int) CompositeRequest {
		var subs []CompositeSubrequest
		subs = append(subs, sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "P31 limits"}))
		for i := 0; i < n; i++ {
			subs = append(subs, sub31("GET", "query?q=SELECT+Id+FROM+Account", fmt.Sprintf("q%d", i), nil))
		}
		for i := 0; i < reads; i++ {
			subs = append(subs, sub31("GET", "sobjects/Account/@{acct.id}", fmt.Sprintf("r%d", i), nil))
		}
		return partial(subs...)
	}
	if _, err := s.Composite(ctx, a, queries(5, 10)); err != nil {
		t.Fatal("5 queries and 10 record reads must be allowed:", err)
	}
	if _, err := s.Composite(ctx, a, queries(6, 0)); err == nil {
		t.Fatal("6 queries accepted")
	}
	h := sub31("POST", "sobjects/Account", "h", map[string]any{"Name": "P31 header"})
	h.HTTPHeaders = map[string]string{"If-Match": "x"}
	out, err := s.Composite(ctx, a, partial(h, sub31("POST", "sobjects/Account", "ok", map[string]any{"Name": "P31 header ok"})))
	if err != nil || out.CompositeResponse[0].HTTPStatusCode != 400 || out.CompositeResponse[1].HTTPStatusCode != 201 {
		t.Fatalf("unsupported header: %v %+v", err, out)
	}
	h.HTTPHeaders = map[string]string{"content-type": "text/plain"}
	if _, err := s.Composite(ctx, a, partial(h)); err == nil {
		t.Fatal("forbidden header accepted")
	}
	// Other tenant and unknown objects are item errors.
	other, err := s.Create(ctx, b, "Account", map[string]any{"Name": "P31 B"})
	if err != nil {
		t.Fatal(err)
	}
	out, err = s.Composite(ctx, a, partial(
		sub31("GET", "sobjects/Account/"+other, "read", nil),
		sub31("DELETE", "sobjects/Account/"+other, "del", nil),
		sub31("POST", "sobjects/Nope__c", "nope", map[string]any{"Name": "x"}),
		sub31("GET", "sobjects/Account/describe", "describe", nil),
	))
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range out.CompositeResponse {
		if e.HTTPStatusCode != 404 {
			t.Fatalf("entry %d: %+v", i, e)
		}
	}
	if _, err := s.Read(ctx, b, "Account", other); err != nil {
		t.Fatal("other tenant record changed")
	}
}
