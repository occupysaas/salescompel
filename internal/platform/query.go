package platform

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Query struct {
	ExplicitLimit bool
	Object        string
	Fields        []string
	WhereField    string
	WhereValue    *string
	HasWhere      bool
	Order         string
	Desc          bool
	Limit         int
}
type token struct {
	text    string
	literal bool
}

func lex(s string) ([]token, error) {
	var out []token
	for i := 0; i < len(s); {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if c == '\'' {
			i++
			var b strings.Builder
			closed := false
			for i < len(s) {
				c = s[i]
				i++
				if c == '\'' {
					closed = true
					break
				}
				if c == '\\' {
					if i == len(s) || (s[i] != '\\' && s[i] != '\'') {
						return nil, invalid("MALFORMED_QUERY", "Invalid escape")
					}
					c = s[i]
					i++
				}
				b.WriteByte(c)
			}
			if !closed {
				return nil, invalid("MALFORMED_QUERY", "Unclosed literal")
			}
			out = append(out, token{b.String(), true})
			continue
		}
		if c == ',' || c == '=' {
			out = append(out, token{text: string(c)})
			i++
			continue
		}
		start := i
		for i < len(s) && (s[i] == '_' || s[i] == '.' || s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z') {
			i++
		}
		if start == i {
			return nil, invalid("MALFORMED_QUERY", "Unsupported query syntax")
		}
		out = append(out, token{text: s[start:i]})
	}
	return out, nil
}
func identifier(t token) bool {
	if t.literal || t.text == "" {
		return false
	}
	for i, r := range t.text {
		if !(unicode.IsLetter(r) || r == '_' || i > 0 && (unicode.IsDigit(r) || r == '.')) {
			return false
		}
	}
	return true
}
func ParseQuery(s string) (Query, error) {
	q := Query{Limit: 2000}
	// PostgreSQL text cannot hold these; reject them as malformed input, not storage failures.
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return q, invalid("MALFORMED_QUERY", "Query must be valid UTF-8 without NUL")
	}
	ts, err := lex(s)
	if err != nil {
		return q, err
	}
	i := 0
	take := func(w string) bool {
		if i < len(ts) && !ts[i].literal && strings.EqualFold(ts[i].text, w) {
			i++
			return true
		}
		return false
	}
	ident := func() string {
		if i < len(ts) && identifier(ts[i]) {
			v := ts[i].text
			i++
			return v
		}
		return ""
	}
	bad := func() (Query, error) { return q, invalid("MALFORMED_QUERY", "Unsupported or incomplete query") }
	if !take("SELECT") {
		return bad()
	}
	for {
		f := ident()
		if f == "" {
			return bad()
		}
		q.Fields = append(q.Fields, f)
		if !take(",") {
			break
		}
	}
	if !take("FROM") {
		return bad()
	}
	q.Object = ident()
	if q.Object == "" {
		return bad()
	}
	if take("WHERE") {
		q.HasWhere = true
		q.WhereField = ident()
		if q.WhereField == "" || !take("=") || i == len(ts) {
			return bad()
		}
		if ts[i].literal {
			v := ts[i].text
			q.WhereValue = &v
			i++
		} else if !take("null") {
			return bad()
		}
	}
	if take("ORDER") {
		if !take("BY") {
			return bad()
		}
		q.Order = ident()
		if q.Order == "" {
			return bad()
		}
		q.Desc = take("DESC")
		if !q.Desc {
			take("ASC")
		}
	}
	if take("LIMIT") {
		q.ExplicitLimit = true
		if i == len(ts) || ts[i].literal {
			return bad()
		}
		n, e := strconv.Atoi(ts[i].text)
		if e != nil || n < 1 || n > 2000 {
			return bad()
		}
		q.Limit = n
		i++
	}
	if i != len(ts) {
		return bad()
	}
	return q, nil
}
