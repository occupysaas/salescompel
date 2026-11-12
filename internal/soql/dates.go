package soql

import (
	"strings"
	"time"
)

// dayStart is UTC midnight of t's calendar day.
func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func addDays(t time.Time, n int) time.Time { return t.AddDate(0, 0, n) }

// weekStart is the Sunday on or before t (en_US default). spec: soql-04
func weekStart(t time.Time) time.Time {
	d := dayStart(t)
	// time.Weekday: Sunday=0
	return addDays(d, -int(d.Weekday()))
}

func monthStart(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

func addMonths(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	// Normalize to the first of the month before adding, then restore day
	// only when the result is still that month (month-start inputs).
	base := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, n, 0).AddDate(0, 0, d-1)
}

func quarterStart(t time.Time) time.Time {
	y, m, _ := t.Date()
	q := time.Month((int(m)-1)/3*3 + 1)
	return time.Date(y, q, 1, 0, 0, 0, 0, time.UTC)
}

func addQuarters(t time.Time, n int) time.Time { return addMonths(t, 3*n) }

func yearStart(t time.Time) time.Time {
	y, _, _ := t.Date()
	return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
}

func addYears(t time.Time, n int) time.Time { return addMonths(t, 12*n) }

// DateLiteralRange returns the UTC half-open range a date literal covers at
// now. Weeks start on Sunday; fiscal periods equal calendar periods.
// Relative N is 1–1000 for every unit. spec: soql-04
func DateLiteralRange(lit Literal, now time.Time) (start, end time.Time, err error) {
	if lit.Kind != LitDateLiteral {
		return time.Time{}, time.Time{}, malformed("expected date literal")
	}
	name := strings.ToUpper(lit.Text)
	n := lit.N
	today := dayStart(now.UTC())

	// Relative-period N bounds (days documented as 1–1000; applied uniformly).
	// LAST_N_*/NEXT_N_* contain _N_; N_*_AGO start with N_.
	if strings.Contains(name, "_N_") || strings.HasPrefix(name, "N_") {
		if n < 1 || n > 1000 {
			return time.Time{}, time.Time{}, malformed("date literal N out of range")
		}
	}

	switch name {
	// Single days.
	case "YESTERDAY":
		return addDays(today, -1), today, nil
	case "TODAY":
		return today, addDays(today, 1), nil
	case "TOMORROW":
		return addDays(today, 1), addDays(today, 2), nil

	// Days windows: LAST includes today; NEXT is the mirror including today.
	case "LAST_90_DAYS":
		return addDays(today, -90), addDays(today, 1), nil
	case "NEXT_90_DAYS":
		return today, addDays(today, 91), nil
	case "LAST_N_DAYS":
		return addDays(today, -n), addDays(today, 1), nil
	case "NEXT_N_DAYS":
		return today, addDays(today, n+1), nil
	case "N_DAYS_AGO":
		d := addDays(today, -n)
		return d, addDays(d, 1), nil

	// Weeks (Sunday start). Period family excludes the current period for
	// LAST_N_* and takes n periods after current for NEXT_N_*.
	case "LAST_WEEK":
		return addDays(weekStart(today), -7), weekStart(today), nil
	case "THIS_WEEK":
		return weekStart(today), addDays(weekStart(today), 7), nil
	case "NEXT_WEEK":
		return addDays(weekStart(today), 7), addDays(weekStart(today), 14), nil
	case "LAST_N_WEEKS":
		return addDays(weekStart(today), -7*n), weekStart(today), nil
	case "NEXT_N_WEEKS":
		return addDays(weekStart(today), 7), addDays(weekStart(today), 7*(n+1)), nil
	case "N_WEEKS_AGO":
		return addDays(weekStart(today), -7*n), addDays(weekStart(today), -7*(n-1)), nil

	// Months.
	case "LAST_MONTH":
		return addMonths(monthStart(today), -1), monthStart(today), nil
	case "THIS_MONTH":
		return monthStart(today), addMonths(monthStart(today), 1), nil
	case "NEXT_MONTH":
		return addMonths(monthStart(today), 1), addMonths(monthStart(today), 2), nil
	case "LAST_N_MONTHS":
		return addMonths(monthStart(today), -n), monthStart(today), nil
	case "NEXT_N_MONTHS":
		return addMonths(monthStart(today), 1), addMonths(monthStart(today), n+1), nil
	case "N_MONTHS_AGO":
		return addMonths(monthStart(today), -n), addMonths(monthStart(today), -n+1), nil

	// Quarters (calendar == fiscal).
	case "LAST_QUARTER", "LAST_FISCAL_QUARTER":
		return addQuarters(quarterStart(today), -1), quarterStart(today), nil
	case "THIS_QUARTER", "THIS_FISCAL_QUARTER":
		return quarterStart(today), addQuarters(quarterStart(today), 1), nil
	case "NEXT_QUARTER", "NEXT_FISCAL_QUARTER":
		return addQuarters(quarterStart(today), 1), addQuarters(quarterStart(today), 2), nil
	case "LAST_N_QUARTERS", "LAST_N_FISCAL_QUARTERS":
		return addQuarters(quarterStart(today), -n), quarterStart(today), nil
	case "NEXT_N_QUARTERS", "NEXT_N_FISCAL_QUARTERS":
		return addQuarters(quarterStart(today), 1), addQuarters(quarterStart(today), n+1), nil
	case "N_QUARTERS_AGO", "N_FISCAL_QUARTERS_AGO":
		return addQuarters(quarterStart(today), -n), addQuarters(quarterStart(today), -n+1), nil

	// Years (calendar == fiscal).
	case "LAST_YEAR", "LAST_FISCAL_YEAR":
		return addYears(yearStart(today), -1), yearStart(today), nil
	case "THIS_YEAR", "THIS_FISCAL_YEAR":
		return yearStart(today), addYears(yearStart(today), 1), nil
	case "NEXT_YEAR", "NEXT_FISCAL_YEAR":
		return addYears(yearStart(today), 1), addYears(yearStart(today), 2), nil
	case "LAST_N_YEARS", "LAST_N_FISCAL_YEARS":
		return addYears(yearStart(today), -n), yearStart(today), nil
	case "NEXT_N_YEARS", "NEXT_N_FISCAL_YEARS":
		return addYears(yearStart(today), 1), addYears(yearStart(today), n+1), nil
	case "N_YEARS_AGO", "N_FISCAL_YEARS_AGO":
		return addYears(yearStart(today), -n), addYears(yearStart(today), -n+1), nil
	}
	return time.Time{}, time.Time{}, malformed("unknown date literal")
}
