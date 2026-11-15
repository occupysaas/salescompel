package platform

import (
	"context"
	"errors"
	"testing"
)

// Drift found by the final spec review: these pin the spec's request-error
// rules where the code had moved them to run time or loosened them.

func wantRequestError(t *testing.T, what string, err error, status int, code string) {
	t.Helper()
	var api *APIError
	if !errors.As(err, &api) || api.Status != status || api.Code != code {
		t.Fatalf("%s: got %v, want request error %d %s", what, err, status, code)
	}
}

// spec: "In url, only as the whole of a record-ID segment, the whole of an
// external-ID value segment, or inside a query-parameter value. Anywhere
// else in a URL is a request error."
func TestCompositeRefLocatorRejected(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	_, err := s.Composite(context.Background(), a, partial(
		sub31("GET", "query?q=SELECT+Id+FROM+Account+LIMIT+1", "q", nil),
		sub31("GET", "query/@{q.nextRecordsUrl}", "more", nil),
	))
	wantRequestError(t, "locator reference", err, 400, "INVALID_COMPOSITE_REQUEST")
}

// spec: a Collections subrequest's request errors are Composite request
// errors, and an unknown or unreadable retrieve field is 400 INVALID_FIELD.
func TestCompositeRetrieveFieldPrevalidated(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	for _, retrieve := range []CompositeSubrequest{
		sub31("GET", "composite/sobjects/Account?ids=001000000000000AAA&fields=Name,Nope__c", "get", nil),
		sub31("POST", "composite/sobjects/Account", "post", map[string]any{"ids": []any{"001000000000000AAA"}, "fields": []any{"Nope__c"}}),
	} {
		_, err := s.Composite(ctx, a, partial(
			sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "R31 drift field"}),
			retrieve,
		))
		wantRequestError(t, retrieve.ReferenceID, err, 400, "INVALID_FIELD")
	}
	if countNamed(t, s, a, "R31 drift field") != 0 {
		t.Fatal("a request error still wrote records")
	}

	// A field that holds a reference is only known at run time: item error.
	out, err := s.Composite(ctx, a, partial(
		sub31("POST", "sobjects/Account", "acct", map[string]any{"Name": "R31 drift ref"}),
		sub31("GET", "composite/sobjects/Account?ids=@{acct.id}&fields=@{acct.id}", "ref", nil),
		sub31("GET", "composite/sobjects/Nope__c?ids=001000000000000AAA&fields=Name", "unknown", nil),
	))
	if err != nil {
		t.Fatal(err)
	}
	if e := out.CompositeResponse[1]; e.HTTPStatusCode != 400 || entryCode(t, e) != "INVALID_FIELD" {
		t.Fatalf("reference field: %+v", e)
	}
	// An unknown object stays an item 404, like every sobjects route.
	if e := out.CompositeResponse[2]; e.HTTPStatusCode != 404 || entryCode(t, e) != "NOT_FOUND" {
		t.Fatalf("unknown object: %+v", e)
	}
	if countNamed(t, s, a, "R31 drift ref") != 1 {
		t.Fatal("independent subrequest did not commit")
	}
}
