package platform

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Findings from the independent Plan 31 code review.

func TestCompositeRefValueSegment(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	out, err := s.Composite(ctx, a, partial(
		sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "R31 nullkey"}),
		sub31("GET", "sobjects/Account/@{acct.id}?fields=Compat_Key__c,Description", "r", nil),
		sub31("PATCH", "sobjects/Account/Compat_Key__c/@{r.Compat_Key__c}", "up", map[string]any{"Name": "R31 never"}),
		sub31("PATCH", "sobjects/Account/@{r.Description}", "upd", map[string]any{"Name": "R31 never"}),
		sub31("GET", "sobjects/Account/@{acct.success}", "bool", nil),
	))
	if err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= 4; i++ {
		e := out.CompositeResponse[i]
		if e.HTTPStatusCode != 400 || entryCode(t, e) != "INVALID_COMPOSITE_REQUEST" {
			t.Fatalf("entry %d: %+v", i, e)
		}
	}
	if countNamed(t, s, a, "R31 never") != 0 {
		t.Fatal("null or empty reference value was used as a literal")
	}
}

func TestCompositeRouteShapes(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	out, err := s.Composite(context.Background(), a, partial(
		sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "R31 shapes"}),
		sub31("GET", "query/?q=SELECT+Id+FROM+Account", "slash", nil),
		sub31("GET", "ui-api/records/x", "uiapi", nil),
		sub31("GET", "sobjects/Account/describe/compactLayouts", "deep", nil),
		sub31("PATCH", "sobjects/account/Compat_Key__c/r31-case-"+a.Tenant, "lower", map[string]any{"Name": "R31 lower"}),
	))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{201, 200, 404, 404, 201} {
		if out.CompositeResponse[i].HTTPStatusCode != want {
			t.Fatalf("entry %d: %+v", i, out.CompositeResponse[i])
		}
	}
}

func TestCompositeCollectionsParamsPrevalidated(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ids := strings.TrimSuffix(strings.Repeat("001000000000000AAA,", 201), ",")
	for _, path := range []string{"composite/sobjects?ids=" + ids, "composite/sobjects", "composite/sobjects/Account?ids=x", "composite/sobjects/Account?fields=Name"} {
		method := "DELETE"
		if strings.Contains(path, "/Account") {
			method = "GET"
		}
		_, err := s.Composite(context.Background(), a, partial(
			sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "R31 prevalidate"}),
			sub31(method, path, "coll", nil),
		))
		var api *APIError
		if !errors.As(err, &api) || api.Status != 400 {
			t.Fatalf("%s %s: %v", method, path, err)
		}
	}
	if countNamed(t, s, a, "R31 prevalidate") != 0 {
		t.Fatal("request error wrote records")
	}
}

func TestDecodeCompositeReferenceIDCode(t *testing.T) {
	for _, ref := range []string{`""`, `5`, `null`} {
		body := `{"compositeRequest":[{"method":"POST","url":"/services/data/v65.0/sobjects/Account","referenceId":` + ref + `,"body":{"Name":"x"}}]}`
		_, err := decodeComposite(strings.NewReader(body))
		var api *APIError
		if !errors.As(err, &api) || api.Code != "JSON_PARSER_ERROR" {
			t.Fatalf("%s: %v", ref, err)
		}
	}
}

func TestCollectionsStrictInput(t *testing.T) {
	c := newColl(t)
	for name, tc := range map[string]struct{ method, path, body string }{
		"repeated-ids":    {"DELETE", "composite/sobjects?ids=a&ids=b", ""},
		"repeated-fields": {"GET", "composite/sobjects/Account?ids=a&fields=Name&fields=Id", ""},
		"trailing-json":   {"POST", "composite/sobjects", `{"records":[{"attributes":{"type":"Account"},"Name":"R31 trailing"}]} {}`},
		"repeated-aon":    {"DELETE", "composite/sobjects?ids=a&allOrNone=true&allOrNone=false", ""},
	} {
		code, out := c.call(tc.method, tc.path, tc.body)
		if code != 400 {
			t.Fatalf("%s: %d %v", name, code, out)
		}
	}
	if c.named(c.a, "R31 trailing") != 0 {
		t.Fatal("trailing JSON body was executed")
	}
	key := "r31-coll-case-" + c.a.Tenant
	got := c.items(c.call("PATCH", "composite/sobjects/account/compat_key__c", `{"records":[{"attributes":{"type":"Account"},"Compat_Key__c":"`+key+`","Name":"R31 coll case"}]}`))
	if got[0]["created"] != true {
		t.Fatalf("lower-case object in Collections upsert: %v", got)
	}
}
