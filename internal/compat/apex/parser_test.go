package apex

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if code == "" {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var d *Diagnostic
	if !errors.As(err, &d) || d.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}
func source(body string) string {
	return "public class Probe { public static Integer run() { " + body + " } }"
}
func TestCompileSubset(t *testing.T) {
	for _, s := range []string{fixture(t, "compute.cls"), source("String s = 'quote\\'slash\\\\'; Boolean b = true; Account a = null; List<Account> rows = null; { Integer n = -2147483648; } return 0;")} {
		_, e := compile(s, DefaultLimits())
		wantCode(t, e, "")
	}
}
func TestCompileRejectsWholeProgram(t *testing.T) {
	for _, tc := range []struct{ body, code string }{{"System.debug('x');", "unsupported"}, {"List<Account> r = Database.query('x');", "unsupported"}, {"Integer x = ;", "syntax"}, {"return (1;", "syntax"}, {"Integer x = 0; x.Id = 1;", "unsupported"}, {"return run();", "unsupported"}} {
		for _, prefix := range []string{"", "return 0; if (false) { "} {
			body := prefix + tc.body
			if prefix != "" {
				body += " }"
			}
			p, e := compile(source(body), DefaultLimits())
			wantCode(t, e, tc.code)
			if p != nil {
				t.Fatal("partial program accepted")
			}
			var d *Diagnostic
			errors.As(e, &d)
			if d.Line < 1 || d.Column < 1 {
				t.Fatal("missing source position")
			}
		}
	}
	_, e := compile("public class Probe extends Parent { }", DefaultLimits())
	wantCode(t, e, "unsupported")
	for _, s := range []string{source("Integer x = 1; Integer x = 2; return x;"), "public class Probe { public static Integer run(Integer x, String x) {return 0;} }"} {
		_, e := compile(s, DefaultLimits())
		wantCode(t, e, "syntax")
	}
}
func TestCompileBounds(t *testing.T) {
	s := source("return 0;")
	l := DefaultLimits()
	l.SourceBytes = len(s)
	_, e := compile(s, l)
	wantCode(t, e, "")
	l.SourceBytes--
	_, e = compile(s, l)
	wantCode(t, e, "budget")
	for _, body := range []string{"return " + strings.Repeat("(", 100) + "1" + strings.Repeat(")", 100) + ";", "return " + strings.Repeat("- ", 100) + "1;", strings.Repeat("{", 100) + "return 0;" + strings.Repeat("}", 100), "return 1" + strings.Repeat("+1", 100) + ";"} {
		l = DefaultLimits()
		l.ParseDepth = 30
		_, e = compile(source(body), l)
		wantCode(t, e, "budget")
	}
	for _, s := range []string{source("String s = 'unterminated;"), source("return 0;") + "extra", source("return 2147483648;"), source("String s = '\\n'; return 0;"), source("return 0;\xff")} {
		_, e = compile(s, DefaultLimits())
		wantCode(t, e, "syntax")
	}
	l = DefaultLimits()
	l.StringBytes = 3
	_, e = compile(source("String s = 'abc'; return 0;"), l)
	wantCode(t, e, "")
	_, e = compile(source("String s = 'abcd'; return 0;"), l)
	wantCode(t, e, "budget")
}

// Quoted keyword/field text must remain a string, never acquire grammar authority.
func TestCompileRejectsQuotedGrammar(t *testing.T) {
	for _, s := range []string{source("'return' 1;"), source("'Integer' x = 1; return x;"), source("'if' (true) {return 1;} return 0;"), source("Account a = null; String s = a.'Name'; return 0;"), "public class Probe { public static Integer run('Integer' n) { return n; } }"} {
		_, err := compile(s, DefaultLimits())
		if err == nil {
			t.Errorf("quoted grammar accepted: %s", s)
		}
	}
}
