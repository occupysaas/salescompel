package platform

import (
	"errors"
	"strings"
)

var errMalformedID = errors.New("malformed record ID")

func idChecksum(base string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	var suffix [3]byte
	for group := 0; group < 3; group++ {
		bits := 0
		for j := 0; j < 5; j++ {
			c := base[group*5+j]
			if c >= 'A' && c <= 'Z' {
				bits |= 1 << j
			}
		}
		suffix[group] = alphabet[bits]
	}
	return string(suffix[:])
}

// NormalizeRecordID preserves the case-sensitive base and checks the suffix.
func NormalizeRecordID(object, input string) (string, error) {
	prefix := "001"
	if object == "Contact" {
		prefix = "003"
	} else if object != "Account" {
		return "", errMalformedID
	}
	if (len(input) != 15 && len(input) != 18) || !strings.HasPrefix(input, prefix) {
		return "", errMalformedID
	}
	for _, c := range input {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return "", errMalformedID
		}
	}
	suffix := idChecksum(input[:15])
	if len(input) == 18 && !strings.EqualFold(input[15:], suffix) {
		return "", errMalformedID
	}
	return input[:15] + suffix, nil
}

func normalizeReferences(p Principal, object string, data map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(data))
	for k, v := range data {
		f, err := field(p, object, k)
		if err != nil {
			return nil, err
		}
		if s, ok := v.(string); ok && s != "" && f.Type == "reference" {
			normalized, err := NormalizeRecordID(f.ReferenceTo[0], s)
			if err != nil {
				return nil, &APIError{Status: 400, Code: "MALFORMED_ID", Message: "Invalid reference ID", Fields: []string{f.Name}}
			}
			v = normalized
		}
		out[k] = v
	}
	return out, nil
}
