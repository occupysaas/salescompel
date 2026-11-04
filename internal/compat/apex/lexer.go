package apex

import (
	"strings"
	"unicode/utf8"
)

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_' }
func isDigit(b byte) bool  { return b >= '0' && b <= '9' }

const reserved = " public class static Integer run Boolean String Account List if else while for update return true false null Id Name Probe_Group__c SELECT FROM WHERE "
const excluded = " extends implements virtual abstract global private protected void trigger with without sharing inherited try catch finally throw new break continue do switch insert delete upsert undelete merge savepoint interface enum "

func wordIn(words, s string) bool { return strings.Contains(words, " "+s+" ") }
func lex(source string, l Limits) ([]token, error) {
	if len(source) > l.SourceBytes {
		return nil, token{line: 1, column: 1}.err("budget")
	}
	if !utf8.ValidString(source) {
		return nil, token{line: 1, column: 1}.err("syntax")
	}
	var tokens []token
	line, col := 1, 1
	for i := 0; i < len(source); {
		b := source[i]
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			i++
			if b == '\n' {
				line++
				col = 1
			} else {
				col++
			}
			continue
		}
		if b == '/' && i+1 < len(source) && source[i+1] == '/' {
			for i < len(source) && source[i] != '\n' {
				i++
				col++
			}
			continue
		}
		t := token{line: line, column: col}
		start := i
		switch {
		case isLetter(b):
			for i < len(source) && (isLetter(source[i]) || isDigit(source[i])) {
				i++
				col++
			}
			t.text = source[start:i]
			t.kind = "word"
			if wordIn(excluded, t.text) {
				return nil, t.err("unsupported")
			}
		case isDigit(b):
			for i < len(source) && isDigit(source[i]) {
				i++
				col++
			}
			t.text = source[start:i]
			t.kind = "number"
		case b == '\'':
			i++
			col++
			var buf []byte
			closed := false
			for i < len(source) {
				c := source[i]
				i++
				col++
				if c == '\'' {
					closed = true
					break
				}
				if c == '\n' || c == '\r' {
					return nil, t.err("syntax")
				}
				if c == '\\' {
					if i >= len(source) {
						return nil, t.err("syntax")
					}
					c = source[i]
					if c != '\\' && c != '\'' {
						return nil, t.err("syntax")
					}
					i++
					col++
				}
				if len(buf) >= l.StringBytes {
					return nil, t.err("budget")
				}
				buf = append(buf, c)
			}
			if !closed {
				return nil, t.err("syntax")
			}
			t.kind = "string"
			t.text = string(buf)
		default:
			t.kind = "symbol"
			t.text = source[i : i+1]
			i++
			col++
			if i < len(source) {
				pair := source[start : i+1]
				if wordIn(" == != <= >= && || ", pair) {
					t.text = pair
					i++
					col++
				} else if wordIn(" ++ -- += -= *= /= ?. => ", pair) {
					return nil, t.err("unsupported")
				}
			}
			if !strings.Contains("{}();,.=:+-*/!<>[]&|", t.text[:1]) {
				return nil, t.err("unsupported")
			}
		}
		tokens = append(tokens, t)
	}
	return append(tokens, token{kind: "eof", line: line, column: col}), nil
}
