package platform

import "testing"

func TestCanonicalFields(t *testing.T) {
	p := Principal{Role: "admin"}
	input := map[string]any{"name": "x"}
	got, err := canonicalFields(p, "Account", input)
	if err != nil || got["Name"] != "x" || input["name"] != "x" {
		t.Fatal(got, err, input)
	}
	for _, data := range []map[string]any{{"Name": 1, "name": 2}, {"unknown": "x"}} {
		if _, err := canonicalFields(p, "Account", data); err == nil {
			t.Fatal("accepted", data)
		}
	}
}
