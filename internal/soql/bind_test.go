package soql

import (
	"errors"
	"strings"
	"testing"
	"time"

	"salescompel/internal/contract"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

// bindFixture is an in-memory typed catalog: the Plan 28 field set on Item__c,
// standard Account/Contact, Typed__c ("Typed items" → Typeditems__r) and a
// self-referential Loop__c for path-depth limits. spec: soql-01
type bindFixture struct {
	snap       metadata.Snapshot
	admin      policy.Principal
	restricted policy.Principal
	now        Clock
}

func ptr[T any](v T) *T { return &v }

func bField(name string, k fieldtype.Kind) metadata.Field {
	f := metadata.Field{Name: name, Label: name, Nillable: true, Createable: true, Updateable: true,
		Options: metadata.Options{Kind: k}}
	switch k {
	case fieldtype.Number, fieldtype.Currency, fieldtype.Percent:
		f.Precision, f.Scale = 18, 2
	case fieldtype.Text:
		f.Length = 80
	}
	return metadata.Complete(f)
}

func bRef(name, rel, child, target string) metadata.Field {
	f := metadata.Field{Name: name, Label: name, Nillable: true, Createable: true, Updateable: true,
		ReferenceTo: []string{target}, RelationshipName: ptr(rel), Length: 18,
		Options: metadata.Options{Kind: fieldtype.Lookup, ChildRelationshipName: child}}
	return metadata.Complete(f)
}

func newBindFixture() bindFixture {
	std := metadata.StandardObjects()
	reqDay := bField("ReqDay__c", fieldtype.Date)
	reqDay.Nillable = false
	item := metadata.Object{Name: "Item__c", Label: "Item", PluralLabel: "Items", Prefix: "a11",
		Fields: []metadata.Field{
			bField("Id", fieldtype.Id),
			bField("Name", fieldtype.Text),
			bField("Txt__c", fieldtype.Text),
			bField("Num__c", fieldtype.Number),
			bField("Amount__c", fieldtype.Currency),
			bField("Day__c", fieldtype.Date),
			reqDay,
			bField("At__c", fieldtype.DateTime),
			bField("Flag__c", fieldtype.Checkbox),
			bField("Stage__c", fieldtype.Picklist),
			bField("Multi__c", fieldtype.MultiselectPicklist),
			bField("Long__c", fieldtype.LongTextArea),
			bRef("Parent__c", "Parent__r", "Items__r", "Parent__c"),
			bRef("Group__c", "Group__r", "Items__r", "Group__c"),
		}}
	parent := metadata.Object{Name: "Parent__c", Label: "Parent", PluralLabel: "Parents", Prefix: "a12",
		Fields: []metadata.Field{bField("Id", fieldtype.Id), bField("Name", fieldtype.Text)}}
	group := metadata.Object{Name: "Group__c", Label: "Group", PluralLabel: "Groups", Prefix: "a15",
		Fields: []metadata.Field{bField("Id", fieldtype.Id), bField("Name", fieldtype.Text)}}
	typed := metadata.Object{Name: "Typed__c", Label: "Typed", PluralLabel: "Typed items", Prefix: "a13",
		Fields: []metadata.Field{
			bField("Id", fieldtype.Id),
			bField("Name", fieldtype.Text),
			bRef("Item__c", "Item__r", "Typeditems__r", "Item__c"),
		}}
	loop := metadata.Object{Name: "Loop__c", Label: "Loop", PluralLabel: "Loops", Prefix: "a14",
		Fields: []metadata.Field{
			bField("Id", fieldtype.Id),
			bField("Name", fieldtype.Text),
			bRef("Self__c", "Self__r", "Loops__r", "Loop__c"),
		}}
	snap := metadata.Snapshot{Tenant: "t1", Objects: append(std, item, parent, group, typed, loop)}
	fixed := time.Date(2026, 11, 11, 15, 0, 0, 0, time.UTC)
	return bindFixture{
		snap:       snap,
		admin:      policy.Principal{Tenant: "t1", Role: "admin"},
		restricted: policy.Principal{Tenant: "t1", Role: "restricted"},
		now:        func() time.Time { return fixed },
	}
}

func (f bindFixture) bind(text string, p policy.Principal) (*Bound, error) {
	q, err := Parse(text)
	if err != nil {
		return nil, err
	}
	return Bind(q, f.snap, p, f.now)
}

func wantCode(t *testing.T, err error, code string) *contract.APIError {
	t.Helper()
	var ae *contract.APIError
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
	return ae
}

func TestBindNames(t *testing.T) {
	f := newBindFixture()
	// Case-insensitive object, field and relationship resolve to canonical names.
	b, err := f.bind("select txt__c from item__c where num__c = 5", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.Object.Name != "Item__c" || len(b.Columns) != 1 || b.Columns[0].Key != "Txt__c" || b.Columns[0].Field.Name != "Txt__c" {
		t.Fatalf("object/field canonicalisation wrong: %#v", b.Columns)
	}
	b, err = f.bind("select account.name, lastname from contact", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.Object.Name != "Contact" || len(b.Columns) != 2 {
		t.Fatalf("contact select wrong: %#v", b.Columns)
	}
	if b.Columns[0].Key != "Name" || len(b.Columns[0].Path) != 1 ||
		b.Columns[0].Path[0].Object.Name != "Account" || b.Columns[0].Path[0].Via.Name != "AccountId" {
		t.Fatalf("relationship not canonical: %#v", b.Columns[0])
	}
	if b.Columns[1].Key != "LastName" {
		t.Fatalf("leaf key wrong: %#v", b.Columns[1])
	}
	// Child relationship names are case-insensitive and canonical.
	b, err = f.bind("select name, (select name from typeditems__r) from item__c", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Children) != 1 || b.Children[0].Key != "Typeditems__r" ||
		b.Children[0].Relationship.ChildSObject != "Typed__c" ||
		b.Children[0].Query == nil || b.Children[0].Query.Object.Name != "Typed__c" {
		t.Fatalf("child not canonical: %#v", b.Children)
	}
	if _, err = f.bind("select name, (select name from TYPEDITEMS__R) from item__c", f.admin); err != nil {
		t.Fatal(err)
	}
	// Unknown object, field and relationship.
	wantCode(t, mustBind(t, f, "select id from Nope__c", f.admin), "INVALID_TYPE")
	wantCode(t, mustBind(t, f, "select Nope__c from Item__c", f.admin), "INVALID_FIELD")
	wantCode(t, mustBind(t, f, "select Nope__r.Name from Item__c", f.admin), "INVALID_FIELD")
	wantCode(t, mustBind(t, f, "select id from Item__c where Nope__c = 1", f.admin), "INVALID_FIELD")
	// Five relationship levels are fine; six are too complicated.
	ok := "select Self__r.Self__r.Self__r.Self__r.Self__r.Name from Loop__c"
	if _, err = f.bind(ok, f.admin); err != nil {
		t.Fatalf("5-level path: %v", err)
	}
	deep := "select Self__r.Self__r.Self__r.Self__r.Self__r.Self__r.Name from Loop__c"
	wantCode(t, mustBind(t, f, deep, f.admin), "QUERY_TOO_COMPLICATED")
	// A nested child subquery is refused.
	wantCode(t, mustBind(t, f, "select name, (select name, (select id from Contacts) from Typeditems__r) from Item__c", f.admin), "MALFORMED_QUERY")
}

func mustBind(t *testing.T, f bindFixture, text string, p policy.Principal) error {
	t.Helper()
	_, err := f.bind(text, p)
	if err == nil {
		t.Fatalf("Bind(%s) unexpectedly succeeded", text)
	}
	return err
}

func TestBindLiteralTypes(t *testing.T) {
	f := newBindFixture()
	for _, ok := range []string{
		"select Id from Item__c where Num__c = 5",
		"select Id from Item__c where Num__c = 5.5e1",
		"select Id from Item__c where Day__c = 2026-11-11",
		"select Id from Item__c where At__c = 2026-01-01T00:00:00Z",
		"select Id from Item__c where Flag__c = true",
		"select Id from Item__c where Multi__c INCLUDES ('A;B')",
		"select Id from Item__c where Multi__c EXCLUDES ('A')",
		"select Id from Item__c where Day__c = TODAY",
		"select Id from Item__c where Txt__c = 'x'",
		"select Id from Item__c where Stage__c = 'Open'",
	} {
		if _, err := f.bind(ok, f.admin); err != nil {
			t.Errorf("Bind(%s) = %v, want ok", ok, err)
		}
	}
	// Wrong-type literal: INVALID_FIELD, documented wording, value never echoed.
	err := mustBind(t, f, "select Id from Item__c where Num__c = '5'", f.admin)
	ae := wantCode(t, err, "INVALID_FIELD")
	want := "value of filter criterion for field 'Num__c' must be of type double and should not be enclosed in quotes"
	if ae.Message != want {
		t.Fatalf("message = %q want %q", ae.Message, want)
	}
	if strings.Contains(ae.Message, "'5'") || strings.Contains(ae.Message, "5") {
		t.Fatalf("message echoes the query value: %q", ae.Message)
	}
	for _, c := range []struct{ text, code string }{
		{"select Id from Item__c where At__c = 2026-01-01T00:00:00", "MALFORMED_QUERY"},
		{"select Id from Item__c where Day__c = 2026-01-01T00:00:00", "MALFORMED_QUERY"},
		{"select Id from Item__c where Id = 'not-an-id'", "INVALID_QUERY_FILTER_OPERATOR"},
		{"select Id from Item__c where Txt__c INCLUDES ('A')", "INVALID_QUERY_FILTER_OPERATOR"},
		{"select Id from Item__c where Num__c LIKE 'a%'", "INVALID_QUERY_FILTER_OPERATOR"},
		{"select Id from Item__c where Day__c = 'x'", "INVALID_FIELD"},
		{"select Id from Item__c where Flag__c = 'true'", "INVALID_FIELD"},
		{"select Id from Item__c where Txt__c = 5", "INVALID_FIELD"},
		{"select Id from Item__c where Num__c = true", "INVALID_FIELD"},
	} {
		wantCode(t, mustBind(t, f, c.text, f.admin), c.code)
	}
}

func TestBindSecurity(t *testing.T) {
	f := newBindFixture()
	denied := []string{
		"select Compat_Text__c from Account",
		"select Id from Account where Compat_Text__c = 'x'",
		"select Id from Account order by Compat_Text__c",
		"select COUNT(Id) from Account group by Compat_Text__c",
		"select COUNT(Compat_Text__c) from Account",
		"select Account.Compat_Text__c from Contact",
		"select Name, (select Account.Compat_Text__c from Contacts) from Account",
	}
	for _, text := range denied {
		wantCode(t, mustBind(t, f, text, f.restricted), "INVALID_FIELD")
	}
	for _, text := range denied[:5] {
		if _, err := f.bind(text, f.admin); err != nil {
			t.Errorf("admin Bind(%s) = %v, want ok", text, err)
		}
	}
	// FIELDS(ALL) silently drops the denied field.
	b, err := f.bind("select FIELDS(ALL) from Account limit 200", f.restricted)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range b.Columns {
		if c.Key == "Compat_Text__c" {
			t.Fatal("FIELDS(ALL) kept the denied field")
		}
	}
	b, err = f.bind("select FIELDS(ALL) from Account limit 200", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range b.Columns {
		if c.Key == "Compat_Text__c" {
			found = true
		}
	}
	if !found {
		t.Fatal("FIELDS(ALL) dropped Compat_Text__c for admin")
	}
}

func TestBindLimits(t *testing.T) {
	f := newBindFixture()
	long := "select Id " + strings.Repeat(" ", 99979) + "from Item__c"
	wantCode(t, mustBind(t, f, long, f.admin), "QUERY_TOO_COMPLICATED")
	where := "Txt__c = '" + strings.Repeat("a", 3990) + "'"
	wantCode(t, mustBind(t, f, "select Id from Item__c where "+where, f.admin), "QUERY_TOO_COMPLICATED")
	subs := make([]string, 21)
	for i := range subs {
		subs[i] = "(select Id from Contacts)"
	}
	wantCode(t, mustBind(t, f, "select "+strings.Join(subs, ", ")+" from Account", f.admin), "QUERY_TOO_COMPLICATED")
	semi := "select Id from Account where Id in (select AccountId from Contact) and Id in (select AccountId from Contact) and Id in (select AccountId from Contact)"
	wantCode(t, mustBind(t, f, semi, f.admin), "QUERY_TOO_COMPLICATED")
	in := make([]string, 1001)
	for i := range in {
		in[i] = "'a'"
	}
	wantCode(t, mustBind(t, f, "select Id from Item__c where Txt__c in ("+strings.Join(in, ",")+")", f.admin), "QUERY_TOO_COMPLICATED")
	wantCode(t, mustBind(t, f, "select FIELDS(ALL) from Item__c", f.admin), "MALFORMED_QUERY")
	wantCode(t, mustBind(t, f, "select FIELDS(ALL) from Item__c limit 201", f.admin), "MALFORMED_QUERY")
	wantCode(t, mustBind(t, f, "select Id from Item__c offset 2001", f.admin), "MALFORMED_QUERY")
	wantCode(t, mustBind(t, f, "select Id from Item__c with SYSTEM_MODE", f.admin), "MALFORMED_QUERY")
	ae := wantCode(t, mustBind(t, f, "select TYPEOF Account WHEN Account THEN Name END from Contact", f.admin), "MALFORMED_QUERY")
	if !strings.Contains(strings.ToUpper(ae.Message), "TYPEOF") {
		t.Fatalf("TYPEOF message does not name the construct: %q", ae.Message)
	}
	if _, err := f.bind("select FIELDS(ALL) from Item__c limit 200", f.admin); err != nil {
		t.Fatalf("FIELDS(ALL) with LIMIT 200: %v", err)
	}
	if _, err := f.bind("select Id from Item__c offset 2000", f.admin); err != nil {
		t.Fatalf("OFFSET 2000: %v", err)
	}
	if _, err := f.bind("select Id from Item__c with SECURITY_ENFORCED", f.admin); err != nil {
		t.Fatalf("SECURITY_ENFORCED: %v", err)
	}
}

func TestBindAggregateRules(t *testing.T) {
	f := newBindFixture()
	wantCode(t, mustBind(t, f, "select SUM(Txt__c) from Item__c", f.admin), "MALFORMED_QUERY")
	wantCode(t, mustBind(t, f, "select SUM(Amount__c), Txt__c from Item__c", f.admin), "MALFORMED_QUERY")
	wantCode(t, mustBind(t, f, "select COUNT(Id) from Item__c group by At__c", f.admin), "MALFORMED_QUERY")
	b, err := f.bind("select CALENDAR_YEAR(At__c) from Item__c group by CALENDAR_YEAR(At__c)", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Columns) != 1 || b.Columns[0].Key != "expr0" || b.Columns[0].Func != "CALENDAR_YEAR" {
		t.Fatalf("date function column: %#v", b.Columns)
	}
	b, err = f.bind("select Stage__c, COUNT(Id) total, SUM(Amount__c), AVG(Amount__c), MIN(Day__c), MAX(At__c) from Item__c group by Stage__c", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"Stage__c", "total", "expr0", "expr1", "expr2", "expr3"}
	if len(b.Columns) != len(wantKeys) {
		t.Fatalf("columns = %#v", b.Columns)
	}
	for i, w := range wantKeys {
		if b.Columns[i].Key != w {
			t.Errorf("key[%d] = %q want %q", i, b.Columns[i].Key, w)
		}
	}
	if !b.Aggregate {
		t.Fatal("Aggregate flag not set")
	}
	b, err = f.bind("select CALENDAR_YEAR(At__c) y, COUNT(Id) from Item__c group by CALENDAR_YEAR(At__c)", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Columns) != 2 || b.Columns[0].Key != "y" || b.Columns[1].Key != "expr0" {
		t.Fatalf("keys = %#v", b.Columns)
	}
	b, err = f.bind("select COUNT() from Item__c", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if !b.CountOnly || !b.Aggregate {
		t.Fatalf("COUNT() flags: %#v", b)
	}
	// HAVING compares aggregates by literal kind, not the argument's field type.
	if _, err = f.bind("select COUNT(Id) from Item__c having COUNT(Id) > 5", f.admin); err != nil {
		t.Fatalf("HAVING COUNT(Id) > 5: %v", err)
	}
}

func TestBindResultKeys(t *testing.T) {
	f := newBindFixture()
	// Path-leaf collisions and alias/child duplicates are rejected; exact same-field duplicates collapse.
	for _, text := range []string{
		"select account.name, name from contact",
		"select Parent__r.Name, Group__r.Name from Item__c",
		"select SUM(Amount__c) expr0, COUNT(Id) expr0 from Item__c",
		"select Name, COUNT(Id) Name from Item__c group by Name",
		"select (select Id from Typeditems__r), (select Id from Typeditems__r) from Item__c",
	} {
		wantCode(t, mustBind(t, f, text, f.admin), "MALFORMED_QUERY")
	}
	if _, err := f.bind("select Name, Id, Name from Item__c", f.admin); err != nil {
		t.Fatalf("same-field duplicate should bind: %v", err)
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

func TestBindHavingTyping(t *testing.T) {
	f := newBindFixture()
	for _, text := range []string{
		"select SUM(Amount__c) from Item__c having SUM(Amount__c) = 'x'",
		"select COUNT(Id) from Item__c having COUNT(Id) = true",
		"select COUNT(Id) from Item__c having COUNT(Id) = 'x'",
		"select CALENDAR_YEAR(At__c) from Item__c group by CALENDAR_YEAR(At__c) having CALENDAR_YEAR(At__c) = true",
		// Aggregates are refused in WHERE; they stay legal in HAVING.
		"select Id from Item__c where SUM(Amount__c) > 5",
		"select Id from Item__c where COUNT(Id) > 5",
	} {
		wantCode(t, mustBind(t, f, text, f.admin), "MALFORMED_QUERY")
	}
	// MIN/MAX type against the argument field and DAY_ONLY as a date: wrong literals are INVALID_FIELD.
	wantCode(t, mustBind(t, f, "select MIN(Day__c) from Item__c having MIN(Day__c) = 'x'", f.admin), "INVALID_FIELD")
	wantCode(t, mustBind(t, f, "select Id from Item__c where DAY_ONLY(At__c) = 5", f.admin), "INVALID_FIELD")
	for _, text := range []string{
		"select SUM(Amount__c) from Item__c having SUM(Amount__c) = 5",
		"select MIN(Day__c) from Item__c having MIN(Day__c) = 2026-11-11",
		"select MAX(At__c) from Item__c having MAX(At__c) = 2026-01-01T00:00:00Z",
		"select CALENDAR_YEAR(At__c) from Item__c group by CALENDAR_YEAR(At__c) having CALENDAR_YEAR(At__c) = 2026",
		"select Id from Item__c where DAY_ONLY(At__c) = 2026-03-09",
		"select DAY_ONLY(At__c) from Item__c group by DAY_ONLY(At__c) having DAY_ONLY(At__c) = 2026-01-01",
	} {
		if _, err := f.bind(text, f.admin); err != nil {
			t.Errorf("Bind(%s) = %v, want ok", text, err)
		}
	}
	// HAVING without aggregation or GROUP BY is refused.
	wantCode(t, mustBind(t, f, "select Name from Item__c having Name = 'x'", f.admin), "MALFORMED_QUERY")
	wantCode(t, mustBind(t, f, "select Name from Item__c having COUNT(Id) > 5", f.admin), "MALFORMED_QUERY")
	// A date function result is numeric, so a date literal is refused; only
	// MIN/MAX over a temporal field compare as that temporal value.
	wantCode(t, mustBind(t, f, "select CALENDAR_YEAR(At__c) from Item__c group by CALENDAR_YEAR(At__c) having CALENDAR_YEAR(At__c) = TODAY", f.admin), "MALFORMED_QUERY")
	wantCode(t, mustBind(t, f, "select COUNT(Day__c) from Item__c having COUNT(Day__c) = TODAY", f.admin), "MALFORMED_QUERY")
	b, err := f.bind("select MIN(Day__c) from Item__c having MIN(Day__c) = TODAY", f.admin)
	if err != nil {
		t.Fatalf("MIN(Day__c) = TODAY: %v", err)
	}
	if _, ok := b.Having.(*BoundAnd); !ok {
		t.Fatalf("MIN(Day__c) = TODAY rewrote to %#v, want range", b.Having)
	}
}

func TestBindFunctionArgs(t *testing.T) {
	f := newBindFixture()
	// The SELECT rules hold in WHERE and GROUP BY.
	for _, text := range []string{
		"select Id from Item__c where toLabel(Txt__c) = 'x'",
		"select Id from Item__c where CALENDAR_YEAR(Txt__c) = 2026",
		"select CALENDAR_YEAR(Txt__c) from Item__c group by CALENDAR_YEAR(Txt__c)",
		"select Id from Item__c where toLabel(Num__c) = 'x'",
		"select Id from Item__c order by CALENDAR_YEAR(Txt__c)",
		// The shared callArg path holds on the HAVING side too.
		"select COUNT(Id) from Item__c having toLabel(Txt__c) = 'x'",
	} {
		wantCode(t, mustBind(t, f, text, f.admin), "MALFORMED_QUERY")
	}
	for _, text := range []string{
		"select Id from Item__c where toLabel(Stage__c) = 'Closed label'",
		"select Id from Item__c where CALENDAR_YEAR(At__c) = 2026",
		"select Id from Item__c where CALENDAR_YEAR(Day__c) = 2026",
	} {
		if _, err := f.bind(text, f.admin); err != nil {
			t.Errorf("Bind(%s) = %v, want ok", text, err)
		}
	}
}

func TestBindDateLiteralIN(t *testing.T) {
	f := newBindFixture()
	// At__c IN (TODAY) is the day's half-open range, not the midnight instant.
	b, err := f.bind("select Id from Item__c where At__c in (TODAY)", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	and, ok := b.Where.(*BoundAnd)
	if !ok || len(and.Items) != 2 {
		t.Fatalf("IN (TODAY) = %#v, want two-sided range", b.Where)
	}
	assertRange(t, and.Items, "At__c")
	// IN (TODAY, YESTERDAY) is an OR of ranges.
	b, err = f.bind("select Id from Item__c where At__c in (TODAY, YESTERDAY)", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	or, ok := b.Where.(*BoundOr)
	if !ok || len(or.Items) != 2 {
		t.Fatalf("IN (TODAY, YESTERDAY) = %#v, want OR of ranges", b.Where)
	}
	for _, it := range or.Items {
		sub, ok := it.(*BoundAnd)
		if !ok || len(sub.Items) != 2 {
			t.Fatalf("range item = %#v", it)
		}
		assertRange(t, sub.Items, "At__c")
	}
	// Mixed IN: the date literal expands to a range, the instant stays equality.
	b, err = f.bind("select Id from Item__c where At__c in (TODAY, 2026-01-01T00:00:00Z)", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	or, ok = b.Where.(*BoundOr)
	if !ok || len(or.Items) != 2 {
		t.Fatalf("mixed IN = %#v", b.Where)
	}
	rangeAnd, ok := or.Items[0].(*BoundAnd)
	if !ok {
		t.Fatalf("mixed IN range = %#v", or.Items[0])
	}
	assertRange(t, rangeAnd.Items, "At__c")
	eq, ok := or.Items[1].(*BoundCmp)
	if !ok || eq.Op != "=" || eq.Right.Cast != "timestamptz" {
		t.Fatalf("mixed IN literal = %#v", or.Items[1])
	}
	// NOT IN on a nillable plain field is OR(IS NULL, NOT …).
	b, err = f.bind("select Id from Item__c where At__c not in (TODAY)", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	or, ok = b.Where.(*BoundOr)
	if !ok || len(or.Items) != 2 {
		t.Fatalf("NOT IN (TODAY) = %#v, want null OR not-range", b.Where)
	}
	isNull, ok := or.Items[0].(*BoundCmp)
	if !ok || isNull.Op != "=" || isNull.Right.Value != nil {
		t.Fatalf("NOT IN null arm = %#v", or.Items[0])
	}
	assertNotRange(t, or.Items[1], "At__c")
	// NOT IN on a non-nillable plain field is just NOT the range.
	b, err = f.bind("select Id from Item__c where ReqDay__c not in (TODAY)", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	assertNotRange(t, b.Where, "ReqDay__c")
	// Date fields use the date cast.
	b, err = f.bind("select Id from Item__c where Day__c in (TODAY)", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	and, ok = b.Where.(*BoundAnd)
	if !ok {
		t.Fatalf("Day__c IN = %#v", b.Where)
	}
	ge := and.Items[0].(*BoundCmp)
	if ge.Right.Cast != "date" {
		t.Fatalf("Day__c cast = %q want date", ge.Right.Cast)
	}
	// Date literals outside IN still refuse on non-temporal fields.
	wantCode(t, mustBind(t, f, "select Id from Item__c where Txt__c in (TODAY)", f.admin), "INVALID_FIELD")
}

// assertRange checks a >= start and < end pair for the named field.
func assertRange(t *testing.T, items []BoundExpr, field string) {
	t.Helper()
	ge, ok := items[0].(*BoundCmp)
	if !ok || ge.Op != ">=" || ge.Left.Field.Name != field {
		t.Fatalf("range start = %#v", items[0])
	}
	lt, ok := items[1].(*BoundCmp)
	if !ok || lt.Op != "<" {
		t.Fatalf("range end = %#v", items[1])
	}
	start, ok1 := ge.Right.Value.(time.Time)
	end, ok2 := lt.Right.Value.(time.Time)
	if !ok1 || !ok2 || !end.After(start) {
		t.Fatalf("bad range [%#v, %#v)", ge.Right.Value, lt.Right.Value)
	}
}

// assertNotRange checks NOT (range) for the named field.
func assertNotRange(t *testing.T, e BoundExpr, field string) {
	t.Helper()
	if not, ok := e.(*BoundNot); ok {
		if and, ok := not.Expr.(*BoundAnd); ok && len(and.Items) == 2 {
			assertRange(t, and.Items, field)
			return
		}
	}
	t.Fatalf("NOT range = %#v", e)
}

func TestBindOrderByResolution(t *testing.T) {
	f := newBindFixture()
	// Unqualified ORDER BY names resolve against the object's fields first.
	b, err := f.bind("select account.name from contact order by name", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Order) != 1 || b.Order[0].Value.Field.Name != "Name" || len(b.Order[0].Value.Path) != 0 {
		t.Fatalf("order by name resolved to %#v, want Contact.Name", b.Order)
	}
	// Aliases resolve when the name is not a field.
	b, err = f.bind("select COUNT(Id) total from Item__c order by total", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.Order[0].Value.Agg != "COUNT" {
		t.Fatalf("order by total = %#v", b.Order[0].Value)
	}
	b, err = f.bind("select COUNT(Id) from Item__c order by expr0", f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.Order[0].Value.Agg != "COUNT" {
		t.Fatalf("order by expr0 = %#v", b.Order[0].Value)
	}
}

func TestBindChildClauses(t *testing.T) {
	f := newBindFixture()
	for _, text := range []string{
		"select name, (select name from typeditems__r offset 1) from item__c",
		"select name, (select count(Id) from typeditems__r) from item__c",
		"select name, (select name from typeditems__r group by name) from item__c",
		"select name, (select name from typeditems__r having name = 'x') from item__c",
	} {
		wantCode(t, mustBind(t, f, text, f.admin), "MALFORMED_QUERY")
	}
}
