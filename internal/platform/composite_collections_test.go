package platform

import (
	"context"
	"testing"
)

func collCreate(ref string, inner bool, names ...string) CompositeSubrequest {
	var records []any
	for _, n := range names {
		r := map[string]any{"attributes": map[string]any{"type": "Account"}, "Name": n}
		if n == "" {
			r = map[string]any{"attributes": map[string]any{"type": "Account"}, "Bogus": 1}
		}
		records = append(records, r)
	}
	return sub31("POST", "composite/sobjects", ref, map[string]any{"allOrNone": inner, "records": records})
}

func TestCompositeCollectionsOuterAllOrNone(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	// Outer true, inner false: the outer flag wins and everything rolls back.
	req := partial(
		sub31("POST", "sobjects/Account", "first", map[string]any{"Name": "CC31 outer"}),
		collCreate("coll", false, "CC31 outer", "", "CC31 outer"),
		sub31("POST", "sobjects/Account", "last", map[string]any{"Name": "CC31 outer"}),
	)
	req.AllOrNone = true
	out, err := s.Composite(context.Background(), a, req)
	if err != nil {
		t.Fatal(err)
	}
	r := out.CompositeResponse
	if r[0].HTTPStatusCode != 400 || entryCode(t, r[0]) != "PROCESSING_HALTED" || r[2].HTTPStatusCode != 400 || entryCode(t, r[2]) != "PROCESSING_HALTED" {
		t.Fatalf("others: %+v", r)
	}
	items, ok := r[1].Body.([]any)
	if r[1].HTTPStatusCode != 400 || !ok || len(items) != 3 {
		t.Fatalf("collections entry: %+v", r[1])
	}
	if itemCode(items[0].(map[string]any)) != "ALL_OR_NONE_OPERATION_ROLLED_BACK" || itemCode(items[1].(map[string]any)) != "INVALID_FIELD" {
		t.Fatalf("items: %v", items)
	}
	if countNamed(t, s, a, "CC31 outer") != 0 {
		t.Fatal("outer all-or-none left records")
	}
}

func TestCompositeCollectionsOuterPartial(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	out, err := s.Composite(ctx, a, partial(
		collCreate("partialColl", false, "CC31 kept", "", "CC31 kept"),
		collCreate("atomicColl", true, "CC31 rolled", ""),
		sub31("POST", "sobjects/Contact", "contact", map[string]any{"LastName": "CC31 ref", "AccountId": "@{partialColl[0].id}"}),
		sub31("GET", "composite/sobjects/Account?ids=@{partialColl[2].id}&fields=Name", "fetch", nil),
		sub31("DELETE", "composite/sobjects?ids=@{partialColl[2].id}", "del", nil),
	))
	if err != nil {
		t.Fatal(err)
	}
	r := out.CompositeResponse
	for i, want := range []int{200, 200, 201, 200, 200} {
		if r[i].HTTPStatusCode != want {
			t.Fatalf("entry %d: %+v", i, r[i])
		}
	}
	inner := r[1].Body.([]any)
	if itemCode(inner[0].(map[string]any)) != "ALL_OR_NONE_OPERATION_ROLLED_BACK" {
		t.Fatalf("inner all-or-none: %v", inner)
	}
	if countNamed(t, s, a, "CC31 rolled") != 0 || countNamed(t, s, a, "CC31 kept") != 1 {
		t.Fatal("inner flags not respected")
	}
	first := r[0].Body.([]any)[0].(map[string]any)["id"]
	row, err := s.Read(ctx, a, "Contact", r[2].Body.(map[string]any)["id"].(string))
	if err != nil || row["AccountId"] != first {
		t.Fatalf("reference into Collections result: %v %v", err, row)
	}
	fetched := r[3].Body.([]any)
	if len(fetched) != 1 || fetched[0].(map[string]any)["Name"] != "CC31 kept" {
		t.Fatalf("retrieve subrequest: %v", fetched)
	}
	if r[4].Body.([]any)[0].(map[string]any)["success"] != true {
		t.Fatalf("delete subrequest: %v", r[4].Body)
	}
}

func TestCompositeCollectionsPrevalidation(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	bad := sub31("POST", "composite/sobjects", "coll", map[string]any{"records": []any{map[string]any{"Name": "no type"}}})
	if _, err := s.Composite(context.Background(), a, partial(sub31("POST", "sobjects/Account", "a", map[string]any{"Name": "CC31 never"}), bad)); err == nil {
		t.Fatal("malformed Collections body accepted")
	}
	if countNamed(t, s, a, "CC31 never") != 0 {
		t.Fatal("request error wrote records")
	}
}

func TestHTTPCompositePartial(t *testing.T) {
	c := newColl(t)
	body := `{"compositeRequest":[
		{"method":"POST","url":"/services/data/v65.0/sobjects/Account","referenceId":"acct","body":{"Name":"H31 acct"}},
		{"method":"POST","url":"/services/data/v65.0/sobjects/Account","referenceId":"bad","body":{"Bogus":1}},
		{"method":"GET","url":"/services/data/v65.0/query?q=SELECT+Name+FROM+Account+WHERE+Id='@{acct.id}'","referenceId":"q"},
		{"method":"POST","url":"/services/data/v65.0/composite/sobjects","referenceId":"coll","body":{"records":[{"attributes":{"type":"Contact"},"LastName":"@{q.records[0].Name}","AccountId":"@{acct.id}"}]}}]}`
	code, out := c.call("POST", "composite", body)
	envelope, ok := out.(map[string]any)
	if code != 200 || !ok || len(envelope) != 1 {
		t.Fatalf("%d %v", code, out)
	}
	entries := envelope["compositeResponse"].([]any)
	want := []float64{201, 400, 200, 200}
	for i, e := range entries {
		m := e.(map[string]any)
		if len(m) != 4 || m["httpStatusCode"] != want[i] || len(m["httpHeaders"].(map[string]any)) != 0 {
			t.Fatalf("entry %d: %v", i, m)
		}
	}
	bad := entries[1].(map[string]any)["body"].([]any)[0].(map[string]any)
	if bad["errorCode"] != "INVALID_FIELD" {
		t.Fatalf("item error body: %v", bad)
	}
	item := entries[3].(map[string]any)["body"].([]any)[0].(map[string]any)
	if item["success"] != true {
		t.Fatalf("collections entry: %v", item)
	}
	if code, out = c.call("POST", "composite", `{"compositeRequest":[{"method":"POST","url":"/services/data/v65.0/sobjects/Account","referenceId":"a[1]","body":{"Name":"x"}}]}`); code != 400 || errorCode(out) != "JSON_PARSER_ERROR" {
		t.Fatalf("bad referenceId: %d %v", code, out)
	}
}
