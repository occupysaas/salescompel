package platform

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"salescompel/internal/recordid"
)

type collHarness struct {
	t     *testing.T
	s     *Store
	h     http.Handler
	a, b  Principal
	token string
}

func newColl(t *testing.T) collHarness {
	s := testStore(t)
	a, b := principals(t, s)
	return collHarness{t: t, s: s, h: NewHandler(s, ""), a: a, b: b, token: "token-" + a.Tenant}
}

func (c collHarness) call(method, path, body string) (int, any) {
	c.t.Helper()
	code, out, _ := apiCall(c.t, c.h, method, apiBase+path, body, c.token)
	return code, out
}

// items decodes a 200 Collections write response.
func (c collHarness) items(code int, out any) []map[string]any {
	c.t.Helper()
	list, ok := out.([]any)
	if code != 200 || !ok {
		c.t.Fatalf("collections: %d %v", code, out)
	}
	res := make([]map[string]any, len(list))
	for i, x := range list {
		res[i], _ = x.(map[string]any)
	}
	return res
}

func itemCode(item map[string]any) string {
	errs, _ := item["errors"].([]any)
	if len(errs) != 1 {
		return ""
	}
	m, _ := errs[0].(map[string]any)
	code, _ := m["statusCode"].(string)
	return code
}

func (c collHarness) named(p Principal, name string) int { return countNamed(c.t, c.s, p, name) }

func TestCollectionsCreateUpdateDelete(t *testing.T) {
	c := newColl(t)
	code, out := c.call("POST", "composite/sobjects", `{"records":[
		{"attributes":{"type":"Account"},"Name":"C31 one"},
		{"attributes":{"type":"Account"},"Bogus":1},
		{"attributes":{"type":"Contact"},"LastName":"C31 contact"}]}`)
	got := c.items(code, out)
	if got[0]["success"] != true || got[1]["success"] != false || itemCode(got[1]) != "INVALID_FIELD" || got[2]["success"] != true {
		t.Fatalf("partial create: %v", got)
	}
	if _, hasID := got[1]["id"]; hasID {
		t.Fatal("failed item must not carry an id")
	}
	if fields, ok := got[1]["errors"].([]any)[0].(map[string]any)["fields"].([]any); !ok || fields == nil {
		t.Fatal("fields must always be present")
	}
	acct, contact := got[0]["id"].(string), got[2]["id"].(string)
	if c.named(c.a, "C31 one") != 1 {
		t.Fatal("create not committed")
	}

	code, out = c.call("PATCH", "composite/sobjects", fmt.Sprintf(`{"allOrNone":false,"records":[
		{"attributes":{"type":"Account"},"id":%q,"Name":"C31 renamed"},
		{"attributes":{"type":"Contact"},"Id":"003000000000000AAA","LastName":"x"}]}`, acct))
	got = c.items(code, out)
	if got[0]["success"] != true || got[0]["id"] != acct || got[1]["success"] != false {
		t.Fatalf("partial update: %v", got)
	}
	if c.named(c.a, "C31 renamed") != 1 {
		t.Fatal("update not committed")
	}

	other, err := c.s.Create(context.Background(), c.b, "Account", map[string]any{"Name": "C31 other"})
	if err != nil {
		t.Fatal(err)
	}
	code, out = c.call("DELETE", "composite/sobjects?ids="+acct+","+contact+",bad,"+other, "")
	got = c.items(code, out)
	if got[0]["success"] != true || got[1]["success"] != true || itemCode(got[2]) != "MALFORMED_ID" || got[3]["success"] != false {
		t.Fatalf("delete: %v", got)
	}
	if _, err := c.s.Read(context.Background(), c.b, "Account", other); err != nil {
		t.Fatal("deleted another tenant's record")
	}
	if _, err := c.s.Read(context.Background(), c.a, "Contact", contact); err == nil {
		t.Fatal("contact not deleted")
	}
}

func TestCollectionsAllOrNone(t *testing.T) {
	c := newColl(t)
	records := []string{}
	for i := 0; i < 5; i++ {
		field := `"Name":"C31 aon"`
		if i == 2 {
			field = `"Bogus":1`
		}
		records = append(records, `{"attributes":{"type":"Account"},`+field+`}`)
	}
	code, out := c.call("POST", "composite/sobjects", `{"allOrNone":true,"records":[`+strings.Join(records, ",")+`]}`)
	got := c.items(code, out)
	for i, item := range got {
		want := "ALL_OR_NONE_OPERATION_ROLLED_BACK"
		if i == 2 {
			want = "INVALID_FIELD"
		}
		if item["success"] != false || itemCode(item) != want {
			t.Fatalf("item %d: %v", i, item)
		}
		_, hasID := item["id"]
		if hasID != (i < 2) {
			t.Fatalf("item %d id presence: %v", i, item)
		}
	}
	if c.named(c.a, "C31 aon") != 0 {
		t.Fatal("all-or-none persisted records")
	}
	// Delete all-or-none: one bad ID keeps every record.
	id, _ := c.s.Create(context.Background(), c.a, "Account", map[string]any{"Name": "C31 keep"})
	code, out = c.call("DELETE", "composite/sobjects?ids="+id+",bad&allOrNone=true", "")
	got = c.items(code, out)
	if got[0]["success"] != false || itemCode(got[0]) != "ALL_OR_NONE_OPERATION_ROLLED_BACK" || c.named(c.a, "C31 keep") != 1 {
		t.Fatalf("delete all-or-none: %v", got)
	}
}

func TestCollectionsUpsert(t *testing.T) {
	c := newColl(t)
	k1, k2 := "c31-"+c.a.Tenant+"-1", "c31-"+c.a.Tenant+"-2"
	body := fmt.Sprintf(`{"records":[
		{"attributes":{"type":"Account"},"Compat_Key__c":%q,"Name":"C31 up one"},
		{"attributes":{"type":"Account"},"Compat_Key__c":%q,"Name":"C31 up two"},
		{"attributes":{"type":"Account"},"Compat_Key__c":%q,"Name":"C31 up one again"},
		{"attributes":{"type":"Account"},"Name":"C31 no key"}]}`, k1, k2, k1)
	got := c.items(c.call("PATCH", "composite/sobjects/Account/Compat_Key__c", body))
	if got[0]["created"] != true || got[1]["created"] != true || got[2]["created"] != false || got[2]["id"] != got[0]["id"] {
		t.Fatalf("upsert results: %v", got)
	}
	if itemCode(got[3]) != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("missing key: %v", got[3])
	}
	if c.named(c.a, "C31 up one again") != 1 || c.named(c.a, "C31 up one") != 0 {
		t.Fatal("duplicate keys must run in order")
	}
	again := c.items(c.call("PATCH", "composite/sobjects/Account/compat_key__c", body))
	if again[0]["created"] != false || again[1]["created"] != false {
		t.Fatalf("second run must update: %v", again)
	}
}

func TestCollectionsUpsertLockOrder(t *testing.T) {
	c := newColl(t)
	var keys []string
	for i := 0; i < 30; i++ {
		keys = append(keys, fmt.Sprintf("c31-lock-%s-%02d", c.a.Tenant, i))
	}
	batch := func(order []string) string {
		var recs []string
		for _, k := range order {
			recs = append(recs, fmt.Sprintf(`{"attributes":{"type":"Account"},"Compat_Key__c":%q,"Name":"C31 lock"}`, k))
		}
		return `{"records":[` + strings.Join(recs, ",") + `]}`
	}
	reversed := make([]string, len(keys))
	for i, k := range keys {
		reversed[len(keys)-1-i] = k
	}
	for round := 0; round < 4; round++ {
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i, order := range [][]string{keys, reversed} {
			wg.Add(1)
			go func(i int, body string) {
				defer wg.Done()
				code, _, _ := apiCall(t, c.h, "PATCH", apiBase+"composite/sobjects/Account/Compat_Key__c", body, c.token)
				codes[i] = code
			}(i, batch(order))
		}
		wg.Wait()
		if codes[0] != 200 || codes[1] != 200 {
			t.Fatalf("round %d: concurrent upserts failed %v", round, codes)
		}
	}
	if c.named(c.a, "C31 lock") != len(keys) {
		t.Fatal("duplicate records from concurrent upserts")
	}
}

func TestCollectionsRetrieve(t *testing.T) {
	c := newColl(t)
	ctx := context.Background()
	one, _ := c.s.Create(ctx, c.a, "Account", map[string]any{"Name": "C31 r1", "Compat_Text__c": "secret"})
	two, _ := c.s.Create(ctx, c.a, "Account", map[string]any{"Name": "C31 r2"})
	contact, _ := c.s.Create(ctx, c.a, "Contact", map[string]any{"LastName": "C31 c"})
	other, _ := c.s.Create(ctx, c.b, "Account", map[string]any{"Name": "C31 b"})
	missing, _ := recordid.New("001")
	ids := []string{two, "bad", contact, missing, other, one[:15], two}
	quoted := `"` + strings.Join(ids, `","`) + `"`
	code, out := c.call("POST", "composite/sobjects/Account", `{"ids":[`+quoted+`],"fields":["name"]}`)
	list, ok := out.([]any)
	if code != 200 || !ok || len(list) != len(ids) {
		t.Fatalf("retrieve: %d %v", code, out)
	}
	names := []any{"C31 r2", nil, nil, nil, nil, "C31 r1", "C31 r2"}
	for i, want := range names {
		if want == nil {
			if list[i] != nil {
				t.Fatalf("entry %d must be null: %v", i, list[i])
			}
			continue
		}
		m, _ := list[i].(map[string]any)
		if m["Name"] != want || m["Id"] == nil || m["attributes"] == nil {
			t.Fatalf("entry %d: %v", i, list[i])
		}
		if _, extra := m["Compat_Text__c"]; extra {
			t.Fatal("unrequested field")
		}
	}
	code, out = c.call("GET", "composite/sobjects/Account?ids="+one+","+two+"&fields=Name,Compat_Text__c", "")
	if list, _ := out.([]any); code != 200 || len(list) != 2 || list[0].(map[string]any)["Compat_Text__c"] != "secret" {
		t.Fatalf("GET retrieve: %d %v", code, out)
	}
	for _, bad := range []string{`{"ids":["` + one + `"],"fields":["Nope"]}`, `{"ids":["` + one + `"]}`, `{"ids":[],"fields":["Name"]}`, `{"ids":["` + one + `"],"fields":["Name"],"x":1}`} {
		code, out = c.call("POST", "composite/sobjects/Account", bad)
		if code != 400 {
			t.Fatalf("%s: %d %v", bad, code, out)
		}
	}
	p := c.a
	p.Role, p.User = "restricted", "reader"
	restricted := "restricted-coll-" + c.a.Tenant
	if err := c.s.Provision(ctx, restricted, p); err != nil {
		t.Fatal(err)
	}
	code, out, _ = apiCall(t, c.h, "POST", apiBase+"composite/sobjects/Account", `{"ids":["`+one+`"],"fields":["Compat_Text__c"]}`, restricted)
	if code != 400 || errorCode(out) != "INVALID_FIELD" {
		t.Fatalf("restricted field: %d %v", code, out)
	}
}

func TestCollectionsRetrieveChunks(t *testing.T) {
	c := newColl(t)
	ctx := context.Background()
	real := map[int]string{}
	for _, at := range []int{0, 999, 1000, 1999} {
		id, err := c.s.Create(ctx, c.a, "Account", map[string]any{"Name": fmt.Sprintf("C31 chunk %d", at)})
		if err != nil {
			t.Fatal(err)
		}
		real[at] = id
	}
	ids := make([]string, 2000)
	for i := range ids {
		if id, ok := real[i]; ok {
			ids[i] = id
		} else {
			ids[i], _ = recordid.New("001")
		}
	}
	code, out := c.call("POST", "composite/sobjects/Account", `{"ids":["`+strings.Join(ids, `","`)+`"],"fields":["Name"]}`)
	list, _ := out.([]any)
	if code != 200 || len(list) != 2000 {
		t.Fatalf("2,000 IDs: %d %d", code, len(list))
	}
	for i, x := range list {
		_, isReal := real[i]
		if (x != nil) != isReal {
			t.Fatalf("entry %d: %v", i, x)
		}
		if isReal && x.(map[string]any)["Name"] != fmt.Sprintf("C31 chunk %d", i) {
			t.Fatalf("entry %d out of order: %v", i, x)
		}
	}
	if code, _ = c.call("POST", "composite/sobjects/Account", `{"ids":["`+strings.Join(append(ids, ids[0]), `","`)+`"],"fields":["Name"]}`); code != 400 {
		t.Fatal("2,001 IDs accepted")
	}
	if code, _ = c.call("GET", "composite/sobjects/Account?fields=Name&ids="+strings.Join(ids[:801], ","), ""); code != 400 {
		t.Fatal("801 GET IDs accepted")
	}
}

func TestCollectionsRequestErrors(t *testing.T) {
	c := newColl(t)
	rec := func(n int, typ string) string {
		var out []string
		for i := 0; i < n; i++ {
			out = append(out, `{"attributes":{"type":"`+typ+`"},"Name":"C31 never"}`)
		}
		return strings.Join(out, ",")
	}
	chunks := []string{}
	for i := 0; i < 11; i++ {
		typ := "Account"
		if i%2 == 1 {
			typ = "Contact"
		}
		chunks = append(chunks, `{"attributes":{"type":"`+typ+`"},"Name":"C31 never"}`)
	}
	for name, tc := range map[string]struct{ method, path, body, code string }{
		"201":            {"POST", "composite/sobjects", `{"records":[` + rec(201, "Account") + `]}`, "INVALID_COMPOSITE_REQUEST"},
		"no-type":        {"POST", "composite/sobjects", `{"records":[{"Name":"x"}]}`, "INVALID_COMPOSITE_REQUEST"},
		"id-on-create":   {"POST", "composite/sobjects", `{"records":[{"attributes":{"type":"Account"},"Id":"001000000000000AAA"}]}`, "INVALID_COMPOSITE_REQUEST"},
		"no-id-update":   {"PATCH", "composite/sobjects", `{"records":[{"attributes":{"type":"Account"},"Name":"x"}]}`, "INVALID_COMPOSITE_REQUEST"},
		"upsert-contact": {"PATCH", "composite/sobjects/Account/Compat_Key__c", `{"records":[{"attributes":{"type":"Contact"},"Compat_Key__c":"k"}]}`, "INVALID_COMPOSITE_REQUEST"},
		"11-chunks":      {"POST", "composite/sobjects", `{"records":[` + strings.Join(chunks, ",") + `]}`, "INVALID_COMPOSITE_REQUEST"},
		"aon-string":     {"POST", "composite/sobjects", `{"allOrNone":"true","records":[` + rec(1, "Account") + `]}`, "INVALID_COMPOSITE_REQUEST"},
		"bad-json":       {"POST", "composite/sobjects", `{`, "JSON_PARSER_ERROR"},
		"no-ids":         {"DELETE", "composite/sobjects", ``, "INVALID_COMPOSITE_REQUEST"},
		"delete-body":    {"DELETE", "composite/sobjects?ids=x", `{}`, "INVALID_COMPOSITE_REQUEST"},
		"delete-aon":     {"DELETE", "composite/sobjects?ids=x&allOrNone=yes", ``, "INVALID_COMPOSITE_REQUEST"},
		"unknown-param":  {"DELETE", "composite/sobjects?ids=x&other=1", ``, "INVALID_COMPOSITE_REQUEST"},
	} {
		code, out := c.call(tc.method, tc.path, tc.body)
		if code != 400 || errorCode(out) != tc.code {
			t.Fatalf("%s: %d %v", name, code, out)
		}
	}
	if c.named(c.a, "C31 never") != 0 {
		t.Fatal("request error wrote records")
	}
	if code, _ := c.call("GET", "composite/sobjects", ""); code != 405 {
		t.Fatal("wrong method")
	}
	if code, _ := c.call("POST", "composite/sobjects/Account/Other__c", "{}"); code != 404 {
		t.Fatal("other external ID route")
	}
	if code, _ := c.call("POST", "composite/sobjects/Nope__c", `{"ids":["x"],"fields":["Name"]}`); code != 404 {
		t.Fatal("unknown object")
	}
}
