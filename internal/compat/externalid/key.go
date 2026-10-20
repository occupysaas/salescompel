// Package externalid evaluates offline external-ID equality hypotheses.
// It is not a product validation or runtime compatibility contract.
package externalid

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// HypothesisVersion identifies the transformation evaluated by this experiment.
const HypothesisVersion = "nfc-full-fold-v1"

// CanonicalKey applies NFC, locale-independent full case folding, then NFC.
// It preserves whitespace and literal percent escapes. Invalid UTF-8 is rejected.
func CanonicalKey(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New("external ID key is not valid UTF-8")
	}
	return norm.NFC.String(cases.Fold().String(norm.NFC.String(value))), nil
}
