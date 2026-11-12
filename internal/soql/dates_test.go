package soql

import (
	"testing"
	"time"
)

func utc(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// TestDateLiteralRanges pins the UTC half-open ranges with Sunday-start weeks
// and fiscal periods equal to calendar periods. spec: soql-04
func TestDateLiteralRanges(t *testing.T) {
	cases := []struct {
		name string
		n    int
		now  time.Time
		s, e time.Time
	}{
		// Single days across a leap-day boundary.
		{"YESTERDAY", 0, utc(2028, 2, 29), utc(2028, 2, 28), utc(2028, 2, 29)},
		{"TODAY", 0, utc(2028, 2, 29), utc(2028, 2, 29), utc(2028, 3, 1)},
		{"TOMORROW", 0, utc(2028, 2, 29), utc(2028, 3, 1), utc(2028, 3, 2)},
		// Weeks start on Sunday: Wednesday 2026-11-11 sits in [11-08, 11-15).
		{"LAST_WEEK", 0, utc(2026, 11, 11), utc(2026, 11, 1), utc(2026, 11, 8)},
		{"THIS_WEEK", 0, utc(2026, 11, 11), utc(2026, 11, 8), utc(2026, 11, 15)},
		{"NEXT_WEEK", 0, utc(2026, 11, 11), utc(2026, 11, 15), utc(2026, 11, 22)},
		// Calendar months and quarters; fiscal equals calendar.
		{"LAST_MONTH", 0, utc(2027, 1, 15), utc(2026, 12, 1), utc(2027, 1, 1)},
		{"THIS_MONTH", 0, utc(2027, 1, 15), utc(2027, 1, 1), utc(2027, 2, 1)},
		{"NEXT_MONTH", 0, utc(2027, 1, 15), utc(2027, 2, 1), utc(2027, 3, 1)},
		{"LAST_QUARTER", 0, utc(2026, 12, 31), utc(2026, 7, 1), utc(2026, 10, 1)},
		{"THIS_QUARTER", 0, utc(2026, 12, 31), utc(2026, 10, 1), utc(2027, 1, 1)},
		{"NEXT_QUARTER", 0, utc(2026, 12, 31), utc(2027, 1, 1), utc(2027, 4, 1)},
		{"LAST_YEAR", 0, utc(2026, 6, 1), utc(2025, 1, 1), utc(2026, 1, 1)},
		{"THIS_YEAR", 0, utc(2026, 6, 1), utc(2026, 1, 1), utc(2027, 1, 1)},
		{"NEXT_YEAR", 0, utc(2026, 6, 1), utc(2027, 1, 1), utc(2028, 1, 1)},
		{"LAST_FISCAL_QUARTER", 0, utc(2026, 12, 31), utc(2026, 7, 1), utc(2026, 10, 1)},
		{"THIS_FISCAL_QUARTER", 0, utc(2026, 12, 31), utc(2026, 10, 1), utc(2027, 1, 1)},
		{"NEXT_FISCAL_QUARTER", 0, utc(2026, 12, 31), utc(2027, 1, 1), utc(2027, 4, 1)},
		{"LAST_FISCAL_YEAR", 0, utc(2026, 6, 1), utc(2025, 1, 1), utc(2026, 1, 1)},
		{"THIS_FISCAL_YEAR", 0, utc(2026, 6, 1), utc(2026, 1, 1), utc(2027, 1, 1)},
		{"NEXT_FISCAL_YEAR", 0, utc(2026, 6, 1), utc(2027, 1, 1), utc(2028, 1, 1)},
		// 90-day windows follow the day family and include today.
		{"LAST_90_DAYS", 0, utc(2026, 11, 11), utc(2026, 8, 13), utc(2026, 11, 12)},
		{"NEXT_90_DAYS", 0, utc(2026, 11, 11), utc(2026, 11, 11), utc(2027, 2, 10)},
		// Relative periods.
		{"LAST_N_DAYS", 30, utc(2026, 11, 11), utc(2026, 10, 12), utc(2026, 11, 12)},
		{"NEXT_N_DAYS", 3, utc(2026, 11, 11), utc(2026, 11, 11), utc(2026, 11, 15)},
		{"N_DAYS_AGO", 3, utc(2026, 11, 11), utc(2026, 11, 8), utc(2026, 11, 9)},
		{"LAST_N_MONTHS", 2, utc(2027, 1, 15), utc(2026, 11, 1), utc(2027, 1, 1)},
		{"NEXT_N_MONTHS", 2, utc(2027, 1, 15), utc(2027, 2, 1), utc(2027, 4, 1)},
		{"N_MONTHS_AGO", 1, utc(2027, 1, 15), utc(2026, 12, 1), utc(2027, 1, 1)},
		{"LAST_N_WEEKS", 2, utc(2026, 11, 11), utc(2026, 10, 25), utc(2026, 11, 8)},
		{"NEXT_N_WEEKS", 2, utc(2026, 11, 11), utc(2026, 11, 15), utc(2026, 11, 29)},
		{"N_WEEKS_AGO", 1, utc(2026, 11, 11), utc(2026, 11, 1), utc(2026, 11, 8)},
		{"LAST_N_QUARTERS", 1, utc(2026, 12, 31), utc(2026, 7, 1), utc(2026, 10, 1)},
		{"N_QUARTERS_AGO", 1, utc(2026, 12, 31), utc(2026, 7, 1), utc(2026, 10, 1)},
		{"LAST_N_YEARS", 2, utc(2026, 6, 1), utc(2024, 1, 1), utc(2026, 1, 1)},
		{"N_YEARS_AGO", 1, utc(2026, 6, 1), utc(2025, 1, 1), utc(2026, 1, 1)},
	}
	for _, c := range cases {
		lit := Literal{Kind: LitDateLiteral, Text: c.name, N: c.n}
		s, e, err := DateLiteralRange(lit, c.now)
		if err != nil {
			t.Errorf("%s:%d @ %s: %v", c.name, c.n, c.now.Format("2006-01-02"), err)
			continue
		}
		if !s.Equal(c.s) || !e.Equal(c.e) {
			t.Errorf("%s:%d @ %s = [%s, %s) want [%s, %s)",
				c.name, c.n, c.now.Format("2006-01-02"),
				s.Format("2006-01-02"), e.Format("2006-01-02"),
				c.s.Format("2006-01-02"), c.e.Format("2006-01-02"))
		}
	}
	// Out-of-range N is MALFORMED_QUERY. spec: soql-06
	for _, lit := range []Literal{
		{Kind: LitDateLiteral, Text: "LAST_N_DAYS", N: 1001},
		{Kind: LitDateLiteral, Text: "LAST_N_DAYS", N: 0},
		{Kind: LitDateLiteral, Text: "N_DAYS_AGO", N: 1001},
	} {
		if _, _, err := DateLiteralRange(lit, utc(2026, 11, 11)); !malformedCode(err) {
			t.Errorf("%v error = %v want MALFORMED_QUERY", lit, err)
		}
	}
	if _, _, err := DateLiteralRange(Literal{Kind: LitDateLiteral, Text: "NOPE"}, utc(2026, 11, 11)); !malformedCode(err) {
		t.Error("unknown date literal accepted")
	}
}
