package soql

import (
	"testing"

	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

// TestBindUngroupedAggregateClauses requires grouped-key validation even when
// there is no GROUP BY: an ungrouped aggregate's ORDER BY and HAVING may only
// reference aggregates. spec: soql-05
func TestBindUngroupedAggregateClauses(t *testing.T) {
	f := newBindFixture()
	for _, text := range []string{
		"select COUNT() from Item__c order by Txt__c",
		"select COUNT(Id) from Item__c order by Txt__c DESC",
		"select COUNT(Id) from Item__c having Txt__c = 'x'",
		"select SUM(Amount__c) from Item__c having Stage__c = 'Open'",
		"select COUNT(Id) from Item__c having DAY_ONLY(At__c) = 2026-11-11",
	} {
		wantCode(t, mustBind(t, f, text, f.admin), "MALFORMED_QUERY")
	}
	// Legal aggregate ORDER BY/HAVING without GROUP BY stays accepted.
	for _, text := range []string{
		"select COUNT(Id) from Item__c order by COUNT(Id)",
		"select COUNT(Id) from Item__c having COUNT(Id) > 5",
		"select SUM(Amount__c) from Item__c having SUM(Amount__c) >= 5",
		"select COUNT() from Item__c having COUNT(Id) > 1",
	} {
		if _, err := f.bind(text, f.admin); err != nil {
			t.Errorf("Bind(%s) = %v, want ok", text, err)
		}
	}
}

// TestBindMinMaxKinds pins MIN/MAX to numeric, date/date-time, text-like and
// picklist kinds. Checkbox, Id, reference and the remaining kinds are refused
// by the binder rather than reaching PostgreSQL. spec: soql-05
func TestBindMinMaxKinds(t *testing.T) {
	f := newBindFixture()
	for _, text := range []string{
		"select MIN(Num__c) from Item__c",
		"select MAX(Amount__c) from Item__c",
		"select MIN(Day__c) from Item__c",
		"select MAX(At__c) from Item__c",
		"select MIN(Txt__c) from Item__c",
		"select MAX(Stage__c) from Item__c",
		"select MIN(Name) from Item__c",
	} {
		if _, err := f.bind(text, f.admin); err != nil {
			t.Errorf("Bind(%s) = %v, want ok", text, err)
		}
	}
	for _, text := range []string{
		"select MIN(Flag__c) from Item__c",
		"select MAX(Flag__c) from Item__c",
		"select MIN(Id) from Item__c",
		"select MAX(Parent__c) from Item__c",
		"select MIN(Long__c) from Item__c",
		"select MAX(Multi__c) from Item__c",
	} {
		wantCode(t, mustBind(t, f, text, f.admin), "MALFORMED_QUERY")
	}
	// HAVING reaches the same kind rule.
	wantCode(t, mustBind(t, f, "select MIN(Flag__c) from Item__c having MIN(Flag__c) = true", f.admin), "MALFORMED_QUERY")
}

// TestBindChildRelationshipFieldPolicy proves the child relationship's
// foreign-key field is authorized before the binder traverses it. The
// restricted principal cannot read Account.Compat_Text__c, so a subquery
// reached through that lookup is refused even though the inner projection is
// readable. spec: soql-02, soql-06
func TestBindChildRelationshipFieldPolicy(t *testing.T) {
	f := newBindFixture()
	parent := metadata.Object{Name: "Parent__c", Label: "Parent", PluralLabel: "Parents", Prefix: "a12",
		Fields: []metadata.Field{bField("Id", fieldtype.Id), bField("Name", fieldtype.Text)}}
	fk := bRef("Compat_Text__c", "", "Accounts", "Parent__c")
	account := metadata.Object{Name: "Account", Label: "Account", PluralLabel: "Accounts", Prefix: "001",
		Fields: []metadata.Field{bField("Id", fieldtype.Id), fk}}
	snap := metadata.Snapshot{Tenant: "t1", Objects: []metadata.Object{parent, account}}
	q, err := Parse("select Id, (select Id from Accounts) from Parent__c")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Bind(q, snap, f.admin, f.now); err != nil {
		t.Fatalf("admin child relation: %v", err)
	}
	_, err = Bind(q, snap, f.restricted, f.now)
	wantCode(t, err, "INVALID_FIELD")
}
