package fieldtype

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// Decimal parses a JSON number or plain decimal string and returns the
// canonical value with exactly scale fractional digits, rounded half-up.
func Decimal(v any, precision, scale int) (string, error) {
	if precision < 1 || precision > 18 || scale < 0 || scale > precision {
		return "", fmt.Errorf("invalid precision %d/%d", precision, scale)
	}
	var s string
	switch t := v.(type) {
	case json.Number:
		s = string(t)
	case string:
		s = t
	default:
		return "", ErrSyntax
	}
	negative := strings.HasPrefix(s, "-")
	body := strings.TrimPrefix(s, "-")
	whole, frac, dotted := strings.Cut(body, ".")
	if whole == "" && frac == "" || !digits(whole) || !digits(frac) || dotted && strings.Contains(frac, ".") {
		return "", ErrSyntax
	}
	round := false
	if len(frac) > scale {
		round = frac[scale] >= '5'
		frac = frac[:scale]
	}
	frac += strings.Repeat("0", scale-len(frac))
	mag, ok := new(big.Int).SetString("0"+whole+frac, 10)
	if !ok {
		return "", ErrSyntax
	}
	if round {
		mag.Add(mag, big.NewInt(1))
	}
	text := mag.String()
	if len(text) < scale+1 {
		text = strings.Repeat("0", scale+1-len(text)) + text
	}
	intPart, fracPart := text[:len(text)-scale], text[len(text)-scale:]
	if intPart != "0" && len(intPart) > precision-scale {
		return "", ErrRange
	}
	if precision == scale && intPart != "0" {
		return "", ErrRange
	}
	out := intPart
	if scale > 0 {
		out += "." + fracPart
	}
	if negative && mag.Sign() != 0 {
		out = "-" + out
	}
	return out, nil
}

func digits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// DecimalJSON converts PostgreSQL numeric text to a JSON number literal
// without trailing fractional zeros.
func DecimalJSON(text string) json.Number {
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "-0" || text == "" {
		text = "0"
	}
	return json.Number(text)
}
