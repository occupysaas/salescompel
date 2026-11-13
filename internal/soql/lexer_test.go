package soql

import (
	"errors"
	"salescompel/internal/contract"
	"testing"
)

func stringTokens(t *testing.T, text string) []Token {
	t.Helper()
	ts, err := Lex(text)
	if err != nil {
		t.Fatalf("Lex(%q) error %v", text, err)
	}
	var out []Token
	for _, tok := range ts {
		if tok.Kind == String {
			out = append(out, tok)
		}
	}
	return out
}

func TestLexEscapes(t *testing.T) {
	cases := []struct{ text, want string }{
		{`'O\'Brien'`, "O'Brien"},
		{`'é'`, "é"},
		{`'\u00e9'`, "é"},
		{`'50\%'`, `50\%`},
		{`'ab\_'`, `ab\_`},
		{`'a\"b'`, `a"b`},
		{`'a\\b'`, `a\b`},
		{`'a\nb'`, "a\nb"},
		{`'a\rb'`, "a\rb"},
		{`'a\tb'`, "a\tb"},
		{`'a\bb'`, "a\bb"},
		{`'a\fb'`, "a\fb"},
		{`'50%'`, `50%`},
	}
	for _, c := range cases {
		got := stringTokens(t, c.text)
		if len(got) != 1 || got[0].Text != c.want {
			t.Errorf("Lex(%q) = %#v want text %q", c.text, got, c.want)
		}
	}
	for _, text := range []string{`'bad\q'`, `'open`, `'a\u00'`, `'a\u0000b'`, `'trailing\`} {
		_, err := Lex(text)
		if !malformedCode(err) {
			t.Errorf("Lex(%q) error = %v want MALFORMED_QUERY", text, err)
		}
	}
}

func TestLexRejectsBadInput(t *testing.T) {
	for _, text := range []string{"SELECT Id FROM Account WHERE Name = '\xff'", "SELECT Id FROM Account WHERE Name = 'a\x00'", "SELECT \x00", string([]byte{0xff})} {
		_, err := Lex(text)
		if !malformedCode(err) {
			t.Errorf("Lex(%q) error = %v want MALFORMED_QUERY", text, err)
		}
	}
}

func TestLexKinds(t *testing.T) {
	ts, err := Lex("SELECT Id FROM Item__c WHERE Day__c = LAST_N_DAYS:30 AND At__c = 2026-01-01T00:00:00Z AND Day__c = 2026-11-11 LIMIT 5")
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		kind TokenKind
		text string
	}{
		{Keyword, "SELECT"}, {Ident, "Id"}, {Keyword, "FROM"}, {Ident, "Item__c"},
		{Keyword, "WHERE"}, {Ident, "Day__c"}, {Op, "="}, {DateLiteral, "LAST_N_DAYS:30"},
		{Keyword, "AND"}, {Ident, "At__c"}, {Op, "="}, {DateTime, "2026-01-01T00:00:00Z"},
		{Keyword, "AND"}, {Ident, "Day__c"}, {Op, "="}, {Date, "2026-11-11"},
		{Keyword, "LIMIT"}, {Number, "5"}, {EOF, ""},
	}
	if len(ts) != len(want) {
		t.Fatalf("got %d tokens want %d: %#v", len(ts), len(want), ts)
	}
	for i, w := range want {
		if ts[i].Kind != w.kind || ts[i].Text != w.text {
			t.Errorf("token %d = (%v,%q) want (%v,%q)", i, ts[i].Kind, ts[i].Text, w.kind, w.text)
		}
	}
}

func malformedCode(err error) bool {
	var ae *contract.APIError
	return errors.As(err, &ae) && ae.Code == "MALFORMED_QUERY"
}

func TestLexDateTimeZone(t *testing.T) {
	for _, text := range []string{
		"2026-01-01T00:00:00Z",
		"2026-01-01T00:00:00+05:30",
		"2026-01-01T00:00:00-08:00",
		"2026-01-01T00:00:00.123Z",
	} {
		ts, err := Lex(text)
		if err != nil {
			t.Errorf("Lex(%q) error %v", text, err)
			continue
		}
		if ts[0].Kind != DateTime || ts[0].Text != text {
			t.Errorf("Lex(%q) token = (%v,%q)", text, ts[0].Kind, ts[0].Text)
		}
	}
	for _, text := range []string{
		"2026-01-01T00:00:00",
		"2026-01-01T00:00:00.5",
		"2026-01-01T00:00:00+05",
		"2026-01-01T00:00:00+5:00",
		"2026-01-01T00:00:00+05-30",
	} {
		if _, err := Lex(text); !malformedCode(err) {
			t.Errorf("Lex(%q) error = %v want MALFORMED_QUERY", text, err)
		}
	}
}

func TestLexRelativeDateLiteralN(t *testing.T) {
	ts, err := Lex("LAST_N_DAYS:30")
	if err != nil {
		t.Fatal(err)
	}
	if ts[0].Kind != DateLiteral || ts[0].Text != "LAST_N_DAYS:30" {
		t.Fatalf("token = (%v,%q)", ts[0].Kind, ts[0].Text)
	}
	for _, text := range []string{"LAST_N_DAYS", "LAST_N_DAYS:", "LAST_N_DAYS:x"} {
		if _, err := Lex(text); !malformedCode(err) {
			t.Errorf("Lex(%q) error = %v want MALFORMED_QUERY", text, err)
		}
	}
	_, err = Lex("LAST_N_DAYS:99999999999999999999")
	var ae *contract.APIError
	if !errors.As(err, &ae) || ae.Message != "relative date literal :n is out of range" {
		t.Errorf("overflow error = %v want out-of-range message", err)
	}
}
