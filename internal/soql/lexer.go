package soql

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Lex tokenizes SOQL. It rejects invalid UTF-8, NUL, unclosed strings,
// unknown escapes and date-times without Z or an offset.
// spec: soql-03
func Lex(text string) ([]Token, error) {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return nil, malformed("query must be valid UTF-8 without NUL")
	}
	l := lexer{src: text}
	var out []Token
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		out = append(out, tok)
		if tok.Kind == EOF {
			return out, nil
		}
	}
}

type lexer struct {
	src string
	i   int
}

func (l *lexer) next() (Token, error) {
	for l.i < len(l.src) && isSpace(l.src[l.i]) {
		l.i++
	}
	start := l.i
	if l.i >= len(l.src) {
		return Token{Kind: EOF, Pos: Pos{Offset: start}}, nil
	}
	c := l.src[l.i]
	switch {
	case c == '\'':
		return l.lexString()
	case c == '(':
		l.i++
		return Token{Kind: LParen, Text: "(", Pos: Pos{Offset: start}}, nil
	case c == ')':
		l.i++
		return Token{Kind: RParen, Text: ")", Pos: Pos{Offset: start}}, nil
	case c == ',':
		l.i++
		return Token{Kind: Comma, Text: ",", Pos: Pos{Offset: start}}, nil
	case c == '=' || c == '<' || c == '>' || c == '!':
		return l.lexOp()
	case c >= '0' && c <= '9':
		return l.lexNumberOrDate()
	case c == '-' && l.i+1 < len(l.src) && l.src[l.i+1] >= '0' && l.src[l.i+1] <= '9':
		return l.lexNumberOrDate()
	case isIdentStart(c):
		return l.lexWord()
	}
	return Token{}, malformed("unexpected character in query")
}

func (l *lexer) lexString() (Token, error) {
	start := l.i
	l.i++ // opening quote
	var b strings.Builder
	for l.i < len(l.src) {
		c := l.src[l.i]
		l.i++
		if c == '\'' {
			return Token{Kind: String, Text: b.String(), Pos: Pos{Offset: start}}, nil
		}
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if l.i >= len(l.src) {
			return Token{}, malformed("unclosed string literal")
		}
		e := l.src[l.i]
		l.i++
		switch e {
		case '\'':
			b.WriteByte('\'')
		case '"':
			b.WriteByte('"')
		case '\\':
			// A literal backslash is doubled in the pattern encoding so LIKE can
			// distinguish it from the escape marker before a wildcard.
			b.WriteString(`\\`)
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case '%', '_':
			// kept escaped so LIKE can tell wildcards from literals
			b.WriteByte('\\')
			b.WriteByte(e)
		case 'u':
			if l.i+4 > len(l.src) {
				return Token{}, malformed("invalid unicode escape in string")
			}
			v, err := strconv.ParseUint(l.src[l.i:l.i+4], 16, 16)
			if err != nil || v == 0 || v >= 0xd800 && v <= 0xdfff {
				return Token{}, malformed("invalid unicode escape in string")
			}
			l.i += 4
			if r := rune(v); r == '\\' {
				// A unicode-escaped backslash is a literal backslash: keep the
				// pattern encoding unambiguous.
				b.WriteString(`\\`)
			} else {
				b.WriteRune(r)
			}
		default:
			return Token{}, malformed("unknown escape in string")
		}
	}
	return Token{}, malformed("unclosed string literal")
}

// decodePattern returns the literal string value of a string-literal token.
// Token text is LIKE-pattern encoded: a literal backslash is doubled, while
// \% and \_ stay escaped so a LIKE operand can tell them apart from the
// wildcards. spec: soql-03
func decodePattern(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\', '%', '_':
				b.WriteByte(s[i+1])
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func (l *lexer) lexOp() (Token, error) {
	start := l.i
	c := l.src[l.i]
	l.i++
	text := string(c)
	switch c {
	case '=', '<', '>':
		if l.i < len(l.src) && l.src[l.i] == '=' {
			text += "="
			l.i++
		} else if c == '<' && l.i < len(l.src) && l.src[l.i] == '>' {
			text = "<>"
			l.i++
		}
	case '!':
		if l.i >= len(l.src) || l.src[l.i] != '=' {
			return Token{}, malformed("unexpected character in query")
		}
		text = "!="
		l.i++
	}
	return Token{Kind: Op, Text: text, Pos: Pos{Offset: start}}, nil
}

// lexNumberOrDate scans YYYY-MM-DD[THH:MM:SS…] before numbers so the
// date punctuation is not read as arithmetic.
func (l *lexer) lexNumberOrDate() (Token, error) {
	start := l.i
	if l.dateAhead() {
		return l.lexDate()
	}
	if l.src[l.i] == '-' {
		l.i++
	}
	for l.i < len(l.src) && isDigit(l.src[l.i]) {
		l.i++
	}
	if l.i < len(l.src) && l.src[l.i] == '.' && l.i+1 < len(l.src) && isDigit(l.src[l.i+1]) {
		l.i++
		for l.i < len(l.src) && isDigit(l.src[l.i]) {
			l.i++
		}
	}
	if l.i < len(l.src) && (l.src[l.i] == 'e' || l.src[l.i] == 'E') {
		j := l.i + 1
		if j < len(l.src) && (l.src[j] == '+' || l.src[j] == '-') {
			j++
		}
		if j < len(l.src) && isDigit(l.src[j]) {
			l.i = j
			for l.i < len(l.src) && isDigit(l.src[l.i]) {
				l.i++
			}
		}
	}
	return Token{Kind: Number, Text: l.src[start:l.i], Pos: Pos{Offset: start}}, nil
}

func (l *lexer) dateAhead() bool {
	s := l.src[l.i:]
	if len(s) < 10 || !digits(s[0:4]) || s[4] != '-' || !digits(s[5:7]) || s[7] != '-' || !digits(s[8:10]) {
		return false
	}
	return true
}

func (l *lexer) lexDate() (Token, error) {
	start := l.i
	l.i += 10
	if l.i >= len(l.src) || l.src[l.i] != 'T' {
		return Token{Kind: Date, Text: l.src[start:l.i], Pos: Pos{Offset: start}}, nil
	}
	kind := DateTime
	l.i++ // T
	// HH:MM:SS
	if l.i+8 > len(l.src) || !digits(l.src[l.i:l.i+2]) || l.src[l.i+2] != ':' ||
		!digits(l.src[l.i+3:l.i+5]) || l.src[l.i+5] != ':' || !digits(l.src[l.i+6:l.i+8]) {
		return Token{}, malformed("malformed date-time literal")
	}
	l.i += 8
	if l.i < len(l.src) && l.src[l.i] == '.' {
		l.i++
		n := 0
		for l.i < len(l.src) && isDigit(l.src[l.i]) {
			l.i++
			n++
		}
		if n == 0 {
			return Token{}, malformed("malformed date-time literal")
		}
	}
	if l.i >= len(l.src) {
		return Token{}, malformed("date-time literal requires Z or an offset")
	}
	// spec: soql-03
	if l.src[l.i] == 'Z' {
		l.i++
	} else if l.src[l.i] == '+' || l.src[l.i] == '-' {
		l.i++
		if l.i+5 > len(l.src) || !digits(l.src[l.i:l.i+2]) || l.src[l.i+2] != ':' || !digits(l.src[l.i+3:l.i+5]) {
			return Token{}, malformed("date-time literal requires Z or an offset")
		}
		l.i += 5
	} else {
		return Token{}, malformed("date-time literal requires Z or an offset")
	}
	return Token{Kind: kind, Text: l.src[start:l.i], Pos: Pos{Offset: start}}, nil
}

func (l *lexer) lexWord() (Token, error) {
	start := l.i
	for l.i < len(l.src) && isIdentPart(l.src[l.i]) {
		l.i++
	}
	for l.i < len(l.src) && l.src[l.i] == '.' && l.i+1 < len(l.src) && isIdentStart(l.src[l.i+1]) {
		l.i++
		for l.i < len(l.src) && isIdentPart(l.src[l.i]) {
			l.i++
		}
	}
	word := l.src[start:l.i]
	if isDateLiteral(word) {
		if isDateLiteralN(word) {
			if l.i >= len(l.src) || l.src[l.i] != ':' {
				return Token{}, malformed("relative date literal requires :n")
			}
			j := l.i + 1
			nstart := j
			for j < len(l.src) && isDigit(l.src[j]) {
				j++
			}
			if j == nstart {
				return Token{}, malformed("relative date literal requires :n")
			}
			if _, err := strconv.Atoi(l.src[nstart:j]); err != nil {
				return Token{}, malformed("relative date literal :n is out of range")
			}
			l.i = j
			return Token{Kind: DateLiteral, Text: strings.ToUpper(l.src[start:l.i]), Pos: Pos{Offset: start}}, nil
		}
		return Token{Kind: DateLiteral, Text: strings.ToUpper(word), Pos: Pos{Offset: start}}, nil
	}
	if keywordSet[strings.ToLower(word)] {
		return Token{Kind: Keyword, Text: word, Pos: Pos{Offset: start}}, nil
	}
	return Token{Kind: Ident, Text: word, Pos: Pos{Offset: start}}, nil
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func digits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// digitsInt converts a digit-only string. Callers must have validated it:
// the lexer rejects an empty or out-of-range :n suffix.
func digitsInt(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}
func isIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
func isIdentPart(c byte) bool { return isIdentStart(c) || isDigit(c) }
