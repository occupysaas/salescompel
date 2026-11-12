package fieldtype

import (
	"errors"
	"testing"
)

func TestDateTimeOffsets(t *testing.T) {
	cases := map[string]string{
		"2026-03-08T23:30:00-05:00":     "2026-03-09T04:30:00.000+0000",
		"2026-01-01T12:00:00+0530":      "2026-01-01T06:30:00.000+0000",
		"2026-01-01T12:00:00+05:30":     "2026-01-01T06:30:00.000+0000",
		"2026-01-01T00:00:00.999Z":      "2026-01-01T00:00:00.000+0000",
		"2026-01-01T00:00:00.123+00:00": "2026-01-01T00:00:00.000+0000",
		"2026-06-01T10:20:30":           "2026-06-01T10:20:30.000+0000",
	}
	for in, want := range cases {
		got, err := ParseDateTime(in)
		if err != nil || FormatDateTime(got) != want {
			t.Errorf("DateTime(%q) = %q,%v want %q", in, FormatDateTime(got), err, want)
		}
	}
	for _, in := range []any{"1699-12-31T23:59:59Z", "4001-01-01T00:00:00Z"} {
		if _, err := ParseDateTime(in); !errors.Is(err, ErrRange) {
			t.Errorf("DateTime(%v) err=%v want ErrRange", in, err)
		}
	}
	for _, in := range []any{"2026-13-01T00:00:00Z", "yesterday", 5, nil, "2026-01-01"} {
		if _, err := ParseDateTime(in); !errors.Is(err, ErrSyntax) {
			t.Errorf("DateTime(%v) err=%v want ErrSyntax", in, err)
		}
	}
}

func TestDate(t *testing.T) {
	if got, err := ParseDate("2026-11-11"); err != nil || got != "2026-11-11" {
		t.Fatalf("Date = %q,%v", got, err)
	}
	if got, err := ParseDate("1700-01-01"); err != nil || got != "1700-01-01" {
		t.Fatalf("min date = %q,%v", got, err)
	}
	for _, in := range []any{"2026-02-30", "20260101", "2026-1-01", 7} {
		if _, err := ParseDate(in); !errors.Is(err, ErrSyntax) {
			t.Errorf("Date(%v) err=%v want ErrSyntax", in, err)
		}
	}
	for _, in := range []any{"4001-01-01", "1699-12-31"} {
		if _, err := ParseDate(in); !errors.Is(err, ErrRange) {
			t.Errorf("Date(%v) err=%v want ErrRange", in, err)
		}
	}
}
