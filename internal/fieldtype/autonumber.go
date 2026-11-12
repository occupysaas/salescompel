package fieldtype

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var autoToken = regexp.MustCompile(`\{[^{}]*\}`)

// ValidAutoNumberFormat requires exactly one {0…} group of 1–10 zeros, only
// {YYYY}/{YY}/{MM}/{DD} as other tokens, no stray braces, and room for the
// number within 30 characters.
func ValidAutoNumberFormat(f string) bool {
	numbers := 0
	for _, tok := range autoToken.FindAllString(f, -1) {
		inner := tok[1 : len(tok)-1]
		switch {
		case inner != "" && strings.Trim(inner, "0") == "" && len(inner) <= 10:
			numbers++
		case inner == "YYYY" || inner == "YY" || inner == "MM" || inner == "DD":
		default:
			return false
		}
	}
	rest := autoToken.ReplaceAllString(f, "")
	if numbers != 1 || strings.ContainsAny(rest, "{}") {
		return false
	}
	out, err := FormatAutoNumber(f, 0, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	return err == nil && out != ""
}

// FormatAutoNumber renders number n using the display format.
func FormatAutoNumber(f string, n int64, now time.Time) (string, error) {
	var err error
	out := autoToken.ReplaceAllStringFunc(f, func(tok string) string {
		switch inner := tok[1 : len(tok)-1]; inner {
		case "YYYY":
			return now.Format("2006")
		case "YY":
			return now.Format("06")
		case "MM":
			return now.Format("01")
		case "DD":
			return now.Format("02")
		default:
			if inner == "" || strings.Trim(inner, "0") != "" {
				err = errors.New("invalid auto-number token")
				return ""
			}
			return fmt.Sprintf("%0*d", len(inner), n)
		}
	})
	if err != nil {
		return "", err
	}
	if len([]rune(out)) > 30 {
		return "", errors.New("auto-number value exceeds 30 characters")
	}
	return out, nil
}
