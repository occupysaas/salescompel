package soql

import (
	"testing"
	"time"
)

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func at(s string) time.Time {
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return v
}

// TODAY is the user's local calendar day, not the UTC one.
func TestTodayInUserZone(t *testing.T) {
	s, e, err := DateLiteralRange(Literal{Kind: LitDateLiteral, Text: "TODAY"}, at("2026-11-14T03:00:00Z"), zone(t, "America/Los_Angeles"))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Equal(at("2026-11-13T08:00:00Z")) || !e.Equal(at("2026-11-14T08:00:00Z")) {
		t.Fatalf("TODAY = [%s, %s)", s.UTC(), e.UTC())
	}
	// A nil location is UTC.
	s, e, _ = DateLiteralRange(Literal{Kind: LitDateLiteral, Text: "TODAY"}, at("2026-11-14T03:00:00Z"), nil)
	if !s.Equal(at("2026-11-14T00:00:00Z")) || !e.Equal(at("2026-11-15T00:00:00Z")) {
		t.Fatalf("UTC TODAY = [%s, %s)", s.UTC(), e.UTC())
	}
}

// Day boundaries are local midnights, so a range across the end of daylight
// saving time is one hour longer than its days.
func TestDateLiteralAcrossDST(t *testing.T) {
	s, e, err := DateLiteralRange(Literal{Kind: LitDateLiteral, Text: "LAST_N_DAYS", N: 3}, at("2026-11-02T15:00:00Z"), zone(t, "America/New_York"))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Equal(at("2026-10-30T04:00:00Z")) || !e.Equal(at("2026-11-03T05:00:00Z")) {
		t.Fatalf("LAST_N_DAYS:3 = [%s, %s)", s.UTC(), e.UTC())
	}
	if got := e.Sub(s); got != 97*time.Hour {
		t.Fatalf("range length %s, want 97h", got)
	}
}

func dateBounds(t *testing.T, b *Bound) (time.Time, time.Time) {
	t.Helper()
	and, ok := b.Where.(*BoundAnd)
	if !ok || len(and.Items) != 2 {
		t.Fatalf("where = %#v", b.Where)
	}
	ge, lt := and.Items[0].(*BoundCmp), and.Items[1].(*BoundCmp)
	return ge.Right.Value.(time.Time), lt.Right.Value.(time.Time)
}

// A date field compared to TODAY uses the local calendar date: east of UTC
// the local date is already the next day.
func TestDateFieldUsesLocalDate(t *testing.T) {
	f := newBindFixture()
	f.now = func() time.Time { return at("2026-11-14T20:00:00Z") }
	p := f.admin
	p.Location = zone(t, "Asia/Tokyo")
	b, err := f.bind("select Id from Item__c where Day__c = TODAY", p)
	if err != nil {
		t.Fatal(err)
	}
	s, e := dateBounds(t, b)
	if !s.Equal(at("2026-11-15T00:00:00Z")) || !e.Equal(at("2026-11-16T00:00:00Z")) {
		t.Fatalf("Day__c = TODAY bounds [%s, %s)", s, e)
	}
	b, err = f.bind("select Id from Item__c where At__c = TODAY", p)
	if err != nil {
		t.Fatal(err)
	}
	s, _ = dateBounds(t, b)
	if !s.Equal(at("2026-11-14T15:00:00Z")) {
		t.Fatalf("At__c = TODAY starts %s", s.UTC())
	}
}

// DAY_ONLY stays a UTC date (spec: soql-05); only date literals move.
func TestDayOnlyStaysUTC(t *testing.T) {
	if got := dateFuncSQL("DAY_ONLY", "c"); got != "(c AT TIME ZONE 'UTC')::date" {
		t.Fatalf("DAY_ONLY sql %q", got)
	}
	f := newBindFixture()
	p := f.admin
	p.Location = zone(t, "America/Los_Angeles")
	b, err := f.bind("select Id from Item__c where DAY_ONLY(At__c) = 2026-03-09", p)
	if err != nil {
		t.Fatal(err)
	}
	cmp, ok := b.Where.(*BoundCmp)
	if !ok || !cmp.Right.Value.(time.Time).Equal(at("2026-03-09T00:00:00Z")) || cmp.Right.Cast != "date" {
		t.Fatalf("DAY_ONLY where = %#v", b.Where)
	}
}
