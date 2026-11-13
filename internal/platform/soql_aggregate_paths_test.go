package platform

import (
	"context"
	"encoding/json"
	"testing"

	"salescompel/internal/metadata"
)

// TestAggregateRelationshipPaths checks that supported relationship paths in
// aggregate queries keep their nested result shape: a grouped parent field and
// an aggregate over a parent path both nest under the relationship, and a null
// parent yields a null relationship. spec: soql-05, soql-08
func TestAggregateRelationshipPaths(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()
	if _, err := metadata.CreateField(ctx, s.pool, p.Tenant, metadata.FieldInput{
		Object: "Parent__c", Name: "Amount__c", Label: "Amount", Type: "Currency",
	}); err != nil {
		t.Fatal(err)
	}
	p1, err := s.Create(ctx, p, "Parent__c", map[string]any{"Name": "P1", "Amount__c": json.Number("2.5")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, p, "Parent__c", map[string]any{"Name": "P2", "Amount__c": json.Number("1.25")}); err != nil {
		t.Fatal(err)
	}
	for _, data := range []map[string]any{
		{"Txt__c": "one", "Parent__c": p1},
		{"Txt__c": "two", "Parent__c": p1},
		{"Txt__c": "none"},
	} {
		if _, err = s.Create(ctx, p, "Item__c", data); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("aggregate over parent path", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT SUM(Parent__r.Amount__c) s FROM Item__c")
		if res.TotalSize != 1 || len(res.Records) != 1 {
			t.Fatalf("aggregate path result = %#v", res)
		}
		rec := res.Records[0]
		if len(aggKeys(rec)) != 1 || !aggKeys(rec)["Parent__r"] {
			t.Fatalf("exact aggregate keys = %#v, want attributes and Parent__r", rec)
		}
		parent, ok := rec["Parent__r"].(map[string]any)
		if !ok {
			t.Fatalf("Parent__r = %#v", rec["Parent__r"])
		}
		sum, ok := parent["s"].(json.Number)
		if !ok || string(sum) != "5" {
			t.Fatalf("SUM(Parent__r.Amount__c) = %T %v, want json.Number(5)", parent["s"], parent["s"])
		}
		attrs, _ := parent["attributes"].(map[string]any)
		if attrs["type"] != "Parent__c" {
			t.Fatalf("parent attributes = %#v", parent["attributes"])
		}
	})

	t.Run("aggregate over null parent path", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT SUM(Parent__r.Amount__c) s FROM Item__c WHERE Parent__c = null")
		if res.TotalSize != 1 || len(res.Records) != 1 {
			t.Fatalf("null parent aggregate = %#v", res)
		}
		rec := res.Records[0]
		if len(aggKeys(rec)) != 1 || !aggKeys(rec)["Parent__r"] {
			t.Fatalf("exact null-parent keys = %#v", rec)
		}
		if rec["Parent__r"] != nil {
			t.Fatalf("null parent path = %#v, want nil", rec["Parent__r"])
		}
	})

	t.Run("grouped parent field and count", func(t *testing.T) {
		res := soqlQuery(t, s, p,
			"SELECT Parent__r.Name, COUNT(Id) c FROM Item__c GROUP BY Parent__r.Name ORDER BY Parent__r.Name")
		if res.TotalSize != 2 || len(res.Records) != 2 {
			t.Fatalf("grouped parent rows = %#v", res)
		}
		var nullGroup, namedGroup bool
		for _, rec := range res.Records {
			if rec["Parent__r"] == nil {
				if got := aggInt(t, rec["c"]); got != 1 {
					t.Fatalf("null group count = %v", rec["c"])
				}
				nullGroup = true
				continue
			}
			parent, ok := rec["Parent__r"].(map[string]any)
			if !ok || parent["Name"] != "P1" {
				t.Fatalf("named group = %#v", rec)
			}
			if got := aggInt(t, rec["c"]); got != 2 {
				t.Fatalf("named group count = %v", rec["c"])
			}
			namedGroup = true
		}
		if !nullGroup || !namedGroup {
			t.Fatalf("null=%v named=%v records=%#v", nullGroup, namedGroup, res.Records)
		}
	})
}
