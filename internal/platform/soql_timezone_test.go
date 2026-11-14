package platform

import (
	"context"
	"testing"
	"time"

	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	soqleng "salescompel/internal/soql"
)

// runInTimezone executes the engine in a transaction whose PostgreSQL session
// time zone is not UTC.
func runInTimezone(t *testing.T, f typedFixture, text, zone string, now time.Time) soqleng.Result {
	t.Helper()
	ctx := context.Background()
	tx, err := f.s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", f.p.Tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "SELECT set_config('TimeZone',$1,true)", zone); err != nil {
		t.Fatal(err)
	}
	snap, err := metadata.Load(ctx, tx, f.p.Tenant)
	if err != nil {
		t.Fatal(err)
	}
	exec, err := policy.ForAPI(f.p, "65.0")
	if err != nil {
		t.Fatal(err)
	}
	res, err := soqleng.Run(ctx, tx, exec, snap, text,
		soqleng.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestDayOnlyTimezoneIndependent keeps DAY_ONLY(DateTime) date-literal
// comparisons in UTC regardless of the PostgreSQL session time zone, with the
// same half-open day semantics. spec: soql-04
func TestDayOnlyTimezoneIndependent(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()
	for _, in := range []map[string]any{
		{"Txt__c": "UTC", "At__c": "2026-03-09T23:30:00Z"},
		{"Txt__c": "Next", "At__c": "2026-03-10T00:30:00Z"},
		{"Txt__c": "DateOnly", "Day__c": "2026-03-09"},
	} {
		if _, err := s.Create(ctx, p, "Item__c", in); err != nil {
			t.Fatal(err)
		}
	}
	fixed := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	// UTC+14: a bare date cast to timestamptz lands on the previous UTC day.
	for _, zone := range []string{"UTC", "Pacific/Kiritimati", "America/Los_Angeles"} {
		res := runInTimezone(t, f, "SELECT Txt__c FROM Item__c WHERE DAY_ONLY(At__c) = 2026-03-09", zone, fixed)
		if res.TotalSize != 1 || res.Records[0]["Txt__c"] != "UTC" {
			t.Fatalf("%s date literal = %#v", zone, res.Records)
		}
		res = runInTimezone(t, f, "SELECT Txt__c FROM Item__c WHERE DAY_ONLY(At__c) = TODAY", zone, fixed)
		if res.TotalSize != 1 || res.Records[0]["Txt__c"] != "UTC" {
			t.Fatalf("%s TODAY = %#v", zone, res.Records)
		}
		res = runInTimezone(t, f, "SELECT Txt__c FROM Item__c WHERE DAY_ONLY(At__c) IN (TODAY)", zone, fixed)
		if res.TotalSize != 1 || res.Records[0]["Txt__c"] != "UTC" {
			t.Fatalf("%s IN TODAY = %#v", zone, res.Records)
		}
		// A Date field is already a calendar day: DAY_ONLY must not round-trip
		// it through a session-time-zone timestamptz conversion.
		res = runInTimezone(t, f, "SELECT Txt__c FROM Item__c WHERE DAY_ONLY(Day__c) = 2026-03-09", zone, fixed)
		if res.TotalSize != 1 || res.Records[0]["Txt__c"] != "DateOnly" {
			t.Fatalf("%s DAY_ONLY(Date) = %#v", zone, res.Records)
		}
		res = runInTimezone(t, f, "SELECT DAY_ONLY(Day__c) d FROM Item__c WHERE Txt__c = 'DateOnly'", zone, fixed)
		if res.TotalSize != 1 || res.Records[0]["d"] != "2026-03-09" {
			t.Fatalf("%s DAY_ONLY(Date) select = %#v", zone, res.Records)
		}
	}
}
