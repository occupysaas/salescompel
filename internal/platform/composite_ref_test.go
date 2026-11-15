package platform

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRefParse(t *testing.T) {
	good := map[string]string{
		"@{a.id}":                     "a:[.id]",
		"@{q.records[0].Id}":          "q:[.records [0] .Id]",
		"@{coll[1].id}":               "coll:[[1] .id]",
		"@{acct.BillingAddress.city}": "acct:[.BillingAddress .city]",
		"@{A_1.x_y}":                  "A_1:[.x_y]",
		"@{a[10]}":                    "a:[[10]]",
	}
	for in, want := range good {
		tm, ok := parseTemplate(in)
		ref, one := tm.single()
		if !ok || !one {
			t.Fatalf("%s: not one reference", in)
		}
		var steps []string
		for _, st := range ref.steps {
			if st.isIdx {
				steps = append(steps, "["+itoa(st.index)+"]")
			} else {
				steps = append(steps, "."+st.name)
			}
		}
		if got := ref.id + ":[" + strings.Join(steps, " ") + "]"; got != want {
			t.Fatalf("%s: got %s want %s", in, got, want)
		}
	}
	bad := []string{"@{a}", "@{a.}", "@{.id}", "@{_a.id}", "@{a[]}", "@{a[01]}", "@{a[-1]}", "@{a[x]}",
		"@{a.id", "@{a..id}", "@{a.id }", "@{a-b.id}", "@{a.id[}", "@{a[100001]}", "x @{ y", "@{}"}
	for _, in := range bad {
		if _, ok := parseTemplate(in); ok {
			t.Fatalf("%q accepted", in)
		}
	}
	tm, ok := parseTemplate("Id = '@{a.id}' AND x = '@{b.records[0].Name}'")
	if !ok || len(tm.refs()) != 2 || tm[0].text != "Id = '" {
		t.Fatalf("embedded: %v %v", ok, tm)
	}
	if tm, ok := parseTemplate("plain } text @ {"); !ok || tm.hasRefs() {
		t.Fatal("plain text must stay literal")
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func bodies(t *testing.T, raw string) map[string]any {
	t.Helper()
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	var out map[string]any
	if err := d.Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRefResolve(t *testing.T) {
	b := bodies(t, `{"acct":{"id":"001A","success":true},
		"q":{"records":[{"Id":"003B","Name":null,"Account":{"Name":"Acme"}}]},
		"coll":[{"id":"001C"},{"id":"001D"}],
		"n":{"count":12.50,"flag":false}}`)
	cases := []struct {
		in   string
		want any
		ok   bool
	}{
		{"@{acct.id}", "001A", true},
		{"@{acct.success}", true, true},
		{"@{q.records[0].Id}", "003B", true},
		{"@{q.records[0].Name}", nil, true},
		{"@{q.records[0].Account.Name}", "Acme", true},
		{"@{coll[1].id}", "001D", true},
		{"@{n.count}", json.Number("12.50"), true},
		{"@{acct.Id}", nil, false},
		{"@{q.records[1].Id}", nil, false},
		{"@{coll.id}", nil, false},
		{"@{acct[0]}", nil, false},
		{"@{missing.id}", nil, false},
		{"@{q.records[0].Id.x}", nil, false},
	}
	for _, c := range cases {
		tm, _ := parseTemplate(c.in)
		ref, _ := tm.single()
		got, ok := resolve(ref, b)
		if ok != c.ok || (ok && !reflect.DeepEqual(got, c.want)) {
			t.Fatalf("%s: got %v %v", c.in, got, ok)
		}
	}
}

func TestRefSubstituteBody(t *testing.T) {
	b := bodies(t, `{"a":{"id":"001A","n":7,"t":true,"z":null,"obj":{"k":1},"list":[1]}}`)
	in := map[string]any{
		"AccountId":   "@{a.id}",
		"Count":       "@{a.n}",
		"Flag":        "@{a.t}",
		"Nothing":     "@{a.z}",
		"Description": "acct @{a.id} has @{a.n} (@{a.z})",
		"Nested":      map[string]any{"deep": []any{"@{a.id}", "lit"}},
		"Plain":       "no refs",
		"Number":      json.Number("3"),
	}
	out, ok := substituteBody(in, b)
	if !ok {
		t.Fatal("substitution failed")
	}
	want := map[string]any{
		"AccountId":   "001A",
		"Count":       json.Number("7"),
		"Flag":        true,
		"Nothing":     nil,
		"Description": "acct 001A has 7 (null)",
		"Nested":      map[string]any{"deep": []any{"001A", "lit"}},
		"Plain":       "no refs",
		"Number":      json.Number("3"),
	}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("got %#v", out)
	}
	if in["AccountId"] != "@{a.id}" {
		t.Fatal("input mutated")
	}
	for _, bad := range []any{"@{a.obj}", "x @{a.list}", "@{a.missing}", map[string]any{"k": []any{"@{b.id}"}}} {
		if _, ok := substituteBody(bad, b); ok {
			t.Fatalf("%v resolved", bad)
		}
	}
}

func TestRefBodyRefs(t *testing.T) {
	refs, ok := bodyRefs(map[string]any{"a": "@{x.id}", "b": []any{"y @{y.records[0].Id}"}, "c": json.Number("1")})
	if !ok || len(refs) != 2 {
		t.Fatalf("%v %v", refs, ok)
	}
	for _, bad := range []any{map[string]any{"@{x.id}": "v"}, map[string]any{"a": "@{x"}, []any{"@{.}"}} {
		if _, ok := bodyRefs(bad); ok {
			t.Fatalf("%v accepted", bad)
		}
	}
}
