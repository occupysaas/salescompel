package soql

import (
	"strconv"
	"strings"
	"testing"

	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

// TestBindLimitsAtCap pins each structural bound at the limit and one over.
// spec: soql-06
func TestBindLimitsAtCap(t *testing.T) {
	f := newBindFixture()
	// Source length: 100,000 passes; 100,001 is refused (over case lives in
	// TestBindLimits).
	at := "select Id " + strings.Repeat(" ", 100_000-len("select Id from Item__c")) + "from Item__c"
	if len(at) != 100_000 {
		t.Fatalf("source length = %d", len(at))
	}
	if _, err := f.bind(at, f.admin); err != nil {
		t.Fatalf("100,000-char query: %v", err)
	}
	// WHERE length: 4,000 passes; 4,001 is refused.
	pad := strings.Repeat("a", 4_000-len("Txt__c = ''"))
	where := "Txt__c = '" + pad + "'"
	if len(where) != 4_000 {
		t.Fatalf("where length = %d", len(where))
	}
	if _, err := f.bind("select Id from Item__c where "+where, f.admin); err != nil {
		t.Fatalf("4,000-char WHERE: %v", err)
	}
	// IN list: 1,000 passes; 1,001 is refused. Numeric elements keep the
	// clause under the WHERE character cap so the list length is what binds.
	in := make([]string, 1_000)
	for i := range in {
		in[i] = "1"
	}
	if _, err := f.bind("select Id from Item__c where Num__c in ("+strings.Join(in, ",")+")", f.admin); err != nil {
		t.Fatalf("1,000-element IN: %v", err)
	}
	// Child subqueries: 20 pass; 21 are refused. Semi-joins do not count
	// toward the child cap.
	subs := make([]string, 20)
	for i := range subs {
		subs[i] = "(select Id from Contacts)"
	}
	if _, err := f.bind("select "+strings.Join(subs, ", ")+" from Account", f.admin); err != nil {
		t.Fatalf("20 child subqueries: %v", err)
	}
	// Semi-joins: 2 pass; 3 are refused (over case in TestBindLimits).
	two := "select Id from Account where Id in (select AccountId from Contact) and Id in (select AccountId from Contact)"
	if _, err := f.bind(two, f.admin); err != nil {
		t.Fatalf("2 semi-joins: %v", err)
	}
	// 20 child subqueries plus 2 semi-joins stay within both caps.
	mixed := "select " + strings.Join(subs, ", ") +
		" from Account where Id in (select AccountId from Contact) and Id in (select AccountId from Contact)"
	if _, err := f.bind(mixed, f.admin); err != nil {
		t.Fatalf("20 children + 2 semis: %v", err)
	}
	// Relative N at 1,000 passes; 1,001 is refused in TestBindLiteralRanges.
	if _, err := f.bind("select Id from Item__c where Day__c = LAST_N_DAYS:1000", f.admin); err != nil {
		t.Fatalf("LAST_N_DAYS:1000: %v", err)
	}
}

func TestBindOrderKeyCap(t *testing.T) {
	f := newBindFixture()
	keys := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = "Txt__c"
		}
		return "select Id from Item__c order by " + strings.Join(parts, ", ")
	}
	// 32 keys pass even when they repeat the same field.
	if _, err := f.bind(keys(maxOrderKeys), f.admin); err != nil {
		t.Fatalf("%d ORDER BY keys: %v", maxOrderKeys, err)
	}
	wantCode(t, mustBind(t, f, keys(maxOrderKeys+1), f.admin), "MALFORMED_QUERY")
}

// newJoinFixture builds J0 with n lookup fields to J1..Jn so a select of
// relationship paths can hit the joined-object cap.
func newJoinFixture(n int) bindFixture {
	objs := make([]metadata.Object, n+1)
	for i := range objs {
		fname := "N" + strconv.Itoa(i) + "__c"
		objs[i] = metadata.Object{
			Name: "J" + strconv.Itoa(i), Label: "J" + strconv.Itoa(i), PluralLabel: "Js" + strconv.Itoa(i),
			Prefix: "a2" + strconv.Itoa(i),
			Fields: []metadata.Field{bField("Id", fieldtype.Id), bField(fname, fieldtype.Text)},
		}
	}
	for i := 1; i <= n; i++ {
		rel := "L" + strconv.Itoa(i) + "__r"
		objs[0].Fields = append(objs[0].Fields,
			bRef("L"+strconv.Itoa(i)+"__c", rel, "Children"+strconv.Itoa(i)+"__r", "J"+strconv.Itoa(i)))
	}
	fixed := newBindFixture()
	return bindFixture{
		snap:  metadata.Snapshot{Tenant: "t1", Objects: objs},
		admin: policy.Principal{Tenant: "t1", Role: "admin"},
		now:   fixed.now,
	}
}

func TestBindJoinedObjectCap(t *testing.T) {
	// 35 joined objects (J0 plus 34 relationship hops) pass; 36 are refused.
	paths := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = "L" + strconv.Itoa(i+1) + "__r.N" + strconv.Itoa(i+1) + "__c"
		}
		return "select " + strings.Join(parts, ", ") + " from J0"
	}
	ok := newJoinFixture(maxJoinedObject - 1)
	if _, err := ok.bind(paths(maxJoinedObject-1), ok.admin); err != nil {
		t.Fatalf("%d joined objects: %v", maxJoinedObject, err)
	}
	over := newJoinFixture(maxJoinedObject)
	wantCode(t, mustBind(t, over, paths(maxJoinedObject), over.admin), "QUERY_TOO_COMPLICATED")
}
