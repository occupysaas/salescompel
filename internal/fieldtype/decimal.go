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
		// JSON clients (Python, JavaScript) write ordinary floats such as
		// 1e+16 or 5e-05; expand the literal exactly. Strings stay plain.
		var err error
		if s, err = expandExponent(string(t)); err != nil {
			return "", err
		}
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

// expandExponent rewrites a JSON number literal with an exponent as a plain
// decimal string, exactly. Magnitudes far beyond 18 digits are out of range;
// vanishingly small ones become zero.
func expandExponent(lit string) (string, error) {
	i := strings.IndexAny(lit, "eE")
	if i < 0 {
		return lit, nil
	}
	mantissa, expText := lit[:i], strings.TrimPrefix(lit[i+1:], "+")
	negative := strings.HasPrefix(mantissa, "-")
	mant := strings.TrimPrefix(mantissa, "-")
	whole, frac := mant, ""
	if dot := strings.IndexByte(mant, '.'); dot >= 0 {
		whole, frac = mant[:dot], mant[dot+1:]
	}
	expNegative := strings.HasPrefix(expText, "-")
	expDigits := strings.TrimPrefix(expText, "-")
	if whole == "" && frac == "" || !digits(whole) || !digits(frac) || expDigits == "" || !digits(expDigits) {
		return "", ErrSyntax
	}
	all := whole + frac
	if strings.Trim(all, "0") == "" {
		return "0", nil
	}
	expDigits = strings.TrimLeft(expDigits, "0")
	if len(expDigits) > 4 {
		if expNegative {
			return "0", nil
		}
		return "", ErrRange
	}
	exp := 0
	for _, c := range expDigits {
		exp = exp*10 + int(c-'0')
	}
	if expNegative {
		exp = -exp
	}
	point := len(whole) + exp
	switch {
	case point > len(all)+40:
		return "", ErrRange
	case point < -40:
		return "0", nil
	}
	var out string
	switch {
	case point <= 0:
		out = "0." + strings.Repeat("0", -point) + all
	case point >= len(all):
		out = all + strings.Repeat("0", point-len(all))
	default:
		out = all[:point] + "." + all[point:]
	}
	if negative {
		out = "-" + out
	}
	return out, nil
}
