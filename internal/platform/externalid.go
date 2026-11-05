package platform

import (
	"errors"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"strings"
	"unicode/utf8"
)

const externalIDRuleVersion = "trim-nfc-full-fold-v1"

func canonicalExternalID(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New("invalid external ID encoding")
	}
	return norm.NFC.String(cases.Fold().String(norm.NFC.String(strings.Trim(value, " ")))), nil
}
