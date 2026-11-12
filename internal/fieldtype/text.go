package fieldtype

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidText reports valid UTF-8 without NUL and at most max characters.
func ValidText(s string, max int) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && utf8.RuneCountInString(s) <= max
}

// ValidEmail applies the candidate address grammar (spec Sec. 2).
func ValidEmail(s string) bool {
	if !ValidText(s, 80) || strings.Count(s, "@") != 1 {
		return false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	local, domain, found := strings.Cut(s, "@")
	if !found || local == "" || !strings.Contains(domain, ".") {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			return false
		}
	}
	return true
}
