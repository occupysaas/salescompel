package fieldtype

import "time"

var (
	minDate = time.Date(1700, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate = time.Date(4000, 12, 31, 23, 59, 59, 0, time.UTC)
)

// ParseDate accepts YYYY-MM-DD within 1700-01-01..4000-12-31.
func ParseDate(v any) (string, error) {
	s, ok := v.(string)
	if !ok || len(s) != 10 {
		return "", ErrSyntax
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", ErrSyntax
	}
	if t.Before(minDate) || t.After(maxDate) {
		return "", ErrRange
	}
	return s, nil
}

var dateTimeLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999-0700", "2006-01-02T15:04:05.999999999"}

// ParseDateTime accepts RFC 3339 with Z, +/-hh:mm or +/-hhmm, or no offset (UTC), and
// returns UTC truncated to whole seconds.
func ParseDateTime(v any) (time.Time, error) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, ErrSyntax
	}
	for _, layout := range dateTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			t = t.UTC().Truncate(time.Second)
			if t.Before(minDate) || t.After(maxDate) {
				return time.Time{}, ErrRange
			}
			return t, nil
		}
	}
	return time.Time{}, ErrSyntax
}

// FormatDateTime renders the REST wire form in UTC.
func FormatDateTime(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000+0000") }
