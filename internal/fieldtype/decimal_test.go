package fieldtype

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDecimal(t *testing.T) {
	cases := []struct {
		p, s int
		in   any
		want string
		err  error
	}{
		{18, 2, json.Number("0.30000000000000004"), "0.30", nil},
		{18, 2, "12.345", "12.35", nil},
		{18, 2, "-12.345", "-12.35", nil},
		{5, 2, "999.995", "", ErrRange},
		{5, 2, "999.994", "999.99", nil},
		{18, 0, "1e3", "", ErrSyntax},
		{18, 2, "NaN", "", ErrSyntax},
		{18, 2, "Infinity", "", ErrSyntax},
		{18, 2, "0x10", "", ErrSyntax},
		{18, 2, " ", "", ErrSyntax},
		{18, 2, "", "", ErrSyntax},
		{18, 2, true, "", ErrSyntax},
		{18, 2, 1.5, "", ErrSyntax},
		{18, 18, "0.1", "0.100000000000000000", nil},
		{18, 18, "1", "", ErrRange},
		{18, 2, "-0.001", "0.00", nil},
		{18, 2, json.Number("12345678901234567.89"), "", ErrRange},
		{18, 2, json.Number("1234567890123456.789"), "1234567890123456.79", nil},
		{18, 0, "007", "7", nil},
		{3, 0, "999.5", "", ErrRange},
		{18, 2, "+1", "", ErrSyntax},
		{18, 2, "1.2.3", "", ErrSyntax},
		{18, 2, "-", "", ErrSyntax},
		{18, 0, json.Number("1e+16"), "10000000000000000", nil},
		{18, 6, json.Number("5e-05"), "0.000050", nil},
		{18, 2, json.Number("-1.25E1"), "-12.50", nil},
		{18, 2, json.Number("1e-7"), "0.00", nil},
		{18, 0, json.Number("1e18"), "", ErrRange},
		{18, 0, json.Number("1e999999999"), "", ErrRange},
		{18, 2, json.Number("1e-999999999"), "0.00", nil},
	}
	for _, c := range cases {
		got, err := Decimal(c.in, c.p, c.s)
		if !errors.Is(err, c.err) || got != c.want {
			t.Errorf("Decimal(%#v,%d,%d) = %q,%v; want %q,%v", c.in, c.p, c.s, got, err, c.want, c.err)
		}
	}
}

func TestDecimalJSON(t *testing.T) {
	for in, want := range map[string]string{"12.50": "12.5", "3.00": "3", "-0.0": "0", "120": "120", "0.10": "0.1", "-1.230": "-1.23"} {
		if got := DecimalJSON(in); string(got) != want {
			t.Errorf("DecimalJSON(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNoFloatPath(t *testing.T) {
	files, _ := os.ReadDir(".")
	for _, f := range files {
		if strings.HasSuffix(f.Name(), "_test.go") || !strings.HasSuffix(f.Name(), ".go") {
			continue
		}
		b, _ := os.ReadFile(f.Name())
		if strings.Contains(string(b), "float64") || strings.Contains(string(b), "ParseFloat") || strings.Contains(string(b), "big.Float") {
			t.Errorf("%s uses floating point", f.Name())
		}
	}
}
