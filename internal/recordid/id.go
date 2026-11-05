package recordid

import (
	"crypto/rand"
	"errors"
	"strings"
)

var ErrMalformedID = errors.New("malformed record ID")

func Checksum(base string) string {
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
func Normalize(prefix, input string) (string, error) {
	if (len(input) != 15 && len(input) != 18) || !strings.HasPrefix(input, prefix) {
		return "", ErrMalformedID
	}
	for _, c := range input {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return "", ErrMalformedID
		}
	}
	suffix := Checksum(input[:15])
	if len(input) == 18 && !strings.EqualFold(input[15:], suffix) {
		return "", ErrMalformedID
	}
	return input[:15] + suffix, nil
}

func New(prefix string) (string, error) {
	var b [9]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var out strings.Builder
	out.WriteString(prefix)
	// 72 random bits, represented by twelve base64-like alphanumeric characters.
	var acc uint64
	for _, x := range b[:8] {
		acc = acc<<8 | uint64(x)
	}
	for i := 0; i < 11; i++ {
		out.WriteByte(alphabet[acc%62])
		acc /= 62
	}
	out.WriteByte(alphabet[int(b[8])%62])
	base := out.String()
	return base + Checksum(base), nil
}
