package fieldtype

import (
	"strings"
	"testing"
	"time"
)

func TestAutoNumber(t *testing.T) {
	now := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		format string
		n      int64
		want   string
	}{
		{"PRJ-{0000}", 7, "PRJ-0007"},
		{"PRJ-{0000}", 12345, "PRJ-12345"},
		{"{YYYY}-{00}", 3, "2026-03"},
		{"{YY}{MM}{DD}-{0}", 0, "261111-0"},
	}
	for _, c := range cases {
		if !ValidAutoNumberFormat(c.format) {
			t.Errorf("format %q rejected", c.format)
		}
		got, err := FormatAutoNumber(c.format, c.n, now)
		if err != nil || got != c.want {
			t.Errorf("FormatAutoNumber(%q,%d)=%q,%v want %q", c.format, c.n, got, err, c.want)
		}
	}
	for _, bad := range []string{"A-{00}-{00}", "NONE", "{00000000000}", "{X}-{0}", "{0", strings.Repeat("x", 31) + "{0}"} {
		if ValidAutoNumberFormat(bad) {
			t.Errorf("format %q accepted", bad)
		}
	}
	if _, err := FormatAutoNumber(strings.Repeat("x", 27)+"{0}", 12345, now); err == nil {
		t.Error("31-character result accepted")
	}
}
