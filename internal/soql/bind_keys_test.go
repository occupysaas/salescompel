package soql

import "testing"

func TestBindResultKeys(t *testing.T) {
	f := newBindFixture()
	// Distinct paths sharing a leaf shape to distinct JSON keys, so they bind.
	for _, text := range []string{
		"select account.name, name from contact",
		"select Parent__r.Name, Group__r.Name from Item__c",
		"select Parent__r.Name, Keep__r.Name from Item__c",
		"select Owner.Name, CreatedBy.Name from Item__c",
		"select Parent__r.Name, Group__r.Name, Parent__r.Name from Item__c",
	} {
		if _, err := f.bind(text, f.admin); err != nil {
			t.Fatalf("distinct paths sharing a leaf should bind (%s): %v", text, err)
		}
	}
	// Alias, generated expr and child keys still collide with a selected field,
	// in either order. A path leaf keeps its flat result key for this check.
	for _, text := range []string{
		"select SUM(Amount__c) expr0, COUNT(Id) expr0 from Item__c",
		"select Name, COUNT(Id) Name from Item__c group by Name",
		"select COUNT(Id) Name, Parent__r.Name from Item__c group by Parent__r.Name",
		"select Parent__r.Name, COUNT(Id) Name from Item__c group by Parent__r.Name",
		"select (select Id from Typeditems__r), (select Id from Typeditems__r) from Item__c",
	} {
		wantCode(t, mustBind(t, f, text, f.admin), "MALFORMED_QUERY")
	}
	if _, err := f.bind("select Name, Id, Name from Item__c", f.admin); err != nil {
		t.Fatalf("same-field duplicate should bind: %v", err)
	}
	if b, err := f.bind("select account.name, account.name from contact", f.admin); err != nil {
		t.Fatalf("same-path duplicate should bind: %v", err)
	} else if len(b.Columns) != 2 || b.Columns[0].Key != b.Columns[1].Key || pathKeyOf(b.Columns[0].Path) != pathKeyOf(b.Columns[1].Path) {
		t.Fatalf("same-path duplicate columns = %#v", b.Columns)
	}
	// An explicit expr0 alias owns the key; generated numbering skips it.
	b, err := f.bind("select SUM(Amount__c) expr0, COUNT(Id) from Item__c", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.Columns[0].Key != "expr0" || b.Columns[1].Key != "expr1" {
		t.Fatalf("skip keys = %#v", b.Columns)
	}
	// The same alias later in the list still reserves the number.
	b, err = f.bind("select COUNT(Id), SUM(Amount__c) expr0 from Item__c", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.Columns[0].Key != "expr1" || b.Columns[1].Key != "expr0" {
		t.Fatalf("reserve keys = %#v", b.Columns)
	}
}
