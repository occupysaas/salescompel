package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func compositeJSON(subs string) string { return `{"allOrNone":true,"compositeRequest":[` + subs + `]}` }

const compositeCreate = `{"method":"POST","url":"/services/data/v65.0/sobjects/Account","referenceId":"a","body":{"Name":"Synthetic"}}`

func TestDecodeComposite(t *testing.T) {
	valid := compositeJSON(compositeCreate)
	for _, input := range []string{valid, strings.Replace(valid, `"allOrNone":true`, `"allOrNone":true,"collateSubrequests":false`, 1), strings.Replace(valid, `"Name":"Synthetic"`, `"Unknown":true`, 1)} {
		if _, err := decodeComposite(strings.NewReader(input)); err != nil {
			t.Fatal(err)
		}
	}
	for name, input := range map[string]string{
		"missing": `{}`, "false": strings.Replace(valid, `true`, `false`, 1), "null": strings.Replace(valid, `true`, `null`, 1), "collate": strings.Replace(valid, `true`, `true,"collateSubrequests":true`, 1), "collate-null": strings.Replace(valid, `true`, `true,"collateSubrequests":null`, 1), "unknown": strings.Replace(valid, `true`, `true,"private_marker":1`, 1), "empty": compositeJSON(""), "null-sub": compositeJSON("null"), "null-array": `{"allOrNone":true,"compositeRequest":null}`, "trailing": valid + ` {}`, "bad-json": `{`, "headers": strings.Replace(valid, `"body":`, `"httpHeaders":{},"body":`, 1), "body-null": strings.Replace(valid, `{"Name":"Synthetic"}`, `null`, 1), "body-array": strings.Replace(valid, `{"Name":"Synthetic"}`, `[]`, 1), "body-missing": strings.Replace(valid, `,"body":{"Name":"Synthetic"}`, ``, 1), "duplicate": compositeJSON(compositeCreate + "," + compositeCreate),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := decodeComposite(strings.NewReader(input))
			var api *APIError
			if !errors.As(err, &api) || api.Status != 400 || strings.Contains(api.Message, "private_marker") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
	// DELETE requires no wire body, not even null.
	d := `{"method":"DELETE","url":"/services/data/v65.0/sobjects/Account/001000000000000AAA","referenceId":"d"}`
	if _, err := decodeComposite(strings.NewReader(compositeJSON(d))); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeComposite(strings.NewReader(compositeJSON(strings.TrimSuffix(d, "}") + `,"body":null}`))); err == nil {
		t.Fatal("null DELETE body")
	}
	for _, n := range []int{1, 25, 26} {
		subs := []string{}
		for i := 0; i < n; i++ {
			subs = append(subs, strings.Replace(compositeCreate, `"a"`, fmt.Sprintf(`"a%d"`, i), 1))
		}
		_, err := decodeComposite(strings.NewReader(compositeJSON(strings.Join(subs, ","))))
		if (err == nil) != (n <= 25) {
			t.Fatalf("count %d: %v", n, err)
		}
	}
}
func TestValidateComposite(t *testing.T) {
	var r CompositeRequest
	if err := json.Unmarshal([]byte(compositeJSON(compositeCreate)), &r); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{strings.Repeat("a", 80), "a", "A", "0_"} {
		r.CompositeRequest[0].ReferenceID = id
		if err := validateComposite(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"", strings.Repeat("a", 81), "_a", "é", "a-b"} {
		r.CompositeRequest[0].ReferenceID = id
		if err := validateComposite(r); err == nil {
			t.Fatal("accepted id")
		}
	}
	r.CompositeRequest[0].ReferenceID = "a"
	for _, url := range []string{"https://private_marker.invalid/", "//evil/", "/services/data/v65.0/composite", "/services/data/v64.0/sobjects/Account", "/services/data/v65.0/sobjects/User", "/services/data/v65.0/sobjects/Account/", "/services/data/v65.0/sobjects/Account?q=1", "/services/data/v65.0/sobjects/Account#x", "/services/data/v65.0/sobjects/%41ccount", "/services/data/v65.0/sobjects/../Account", "/services//data/v65.0/sobjects/Account"} {
		r.CompositeRequest[0].URL = url
		if err := validateComposite(r); err == nil {
			t.Fatal("accepted URL")
		}
	}
	r.CompositeRequest[0].URL = "/services/data/v65.0/sobjects/Account"
	for _, method := range []string{"post", "GET", "PUT", ""} {
		r.CompositeRequest[0].Method = method
		if err := validateComposite(r); err == nil {
			t.Fatal("accepted method")
		}
	}
	r.CompositeRequest[0].Method = "POST"
	second := CompositeSubrequest{Method: "POST", URL: "/services/data/v65.0/sobjects/Contact", ReferenceID: "c", Body: map[string]any{"LastName": "Synthetic", "AccountId": "@{a.id}"}}
	r.CompositeRequest = append(r.CompositeRequest, second)
	if err := validateComposite(r); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"@{a.Id}", "@{c.id}", "@{future.id}", "before@{a.id}", "@{a.records[0].Id}", "@{a.id}/more"} {
		r.CompositeRequest[1].Body["AccountId"] = ref
		if err := validateComposite(r); err == nil {
			t.Fatal("accepted reference")
		}
	}
	r.CompositeRequest[1].Body = map[string]any{"LastName": "@{a.id}"}
	if err := validateComposite(r); err == nil {
		t.Fatal("reference in ordinary field")
	}
	r.CompositeRequest[1].Body = map[string]any{"@{a.id}": "x"}
	if err := validateComposite(r); err == nil {
		t.Fatal("reference key")
	}
	r.CompositeRequest[1] = CompositeSubrequest{Method: "PATCH", URL: "/services/data/v65.0/sobjects/Account/@{a.id}", ReferenceID: "p", Body: map[string]any{"Name": "Changed"}}
	if err := validateComposite(r); err != nil {
		t.Fatal(err)
	}
	r.CompositeRequest = append(r.CompositeRequest, CompositeSubrequest{Method: "DELETE", URL: "/services/data/v65.0/sobjects/Account/@{p.id}", ReferenceID: "d"})
	if err := validateComposite(r); err == nil {
		t.Fatal("reference to non-POST")
	}
}
