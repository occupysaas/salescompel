package soql

import (
	"strconv"
	"strings"
)

// CanonicalQuery re-serializes a parsed query into text that parses back to
// the same AST. Locators store this form so every page is re-parsed and
// re-bound rather than replaying compiled SQL. spec: page-01
func CanonicalQuery(q *Query) string {
	if q == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("SELECT ")
	for i, item := range q.Select {
		if i > 0 {
			b.WriteString(", ")
		}
		writeSelectItem(&b, item)
	}
	b.WriteString(" FROM ")
	b.WriteString(string(q.From))
	if q.Alias != "" {
		b.WriteString(" AS ")
		b.WriteString(q.Alias)
	}
	if q.Where != nil {
		b.WriteString(" WHERE ")
		writeExpr(&b, q.Where)
	}
	if len(q.GroupBy) > 0 {
		b.WriteString(" GROUP BY ")
		for i, v := range q.GroupBy {
			if i > 0 {
				b.WriteString(", ")
			}
			writeValueExpr(&b, v)
		}
	}
	if q.Having != nil {
		b.WriteString(" HAVING ")
		writeExpr(&b, q.Having)
	}
	if len(q.OrderBy) > 0 {
		b.WriteString(" ORDER BY ")
		for i, o := range q.OrderBy {
			if i > 0 {
				b.WriteString(", ")
			}
			writeValueExpr(&b, o.Value)
			if o.Desc {
				b.WriteString(" DESC")
			} else {
				b.WriteString(" ASC")
			}
			switch o.Nulls {
			case NullFirst:
				b.WriteString(" NULLS FIRST")
			case NullLast:
				b.WriteString(" NULLS LAST")
			}
		}
	}
	if q.Limit != nil {
		b.WriteString(" LIMIT ")
		b.WriteString(strconv.Itoa(*q.Limit))
	}
	if q.Offset != nil {
		b.WriteString(" OFFSET ")
		b.WriteString(strconv.Itoa(*q.Offset))
	}
	if q.With != "" {
		b.WriteString(" WITH ")
		b.WriteString(q.With)
	}
	return b.String()
}

func writeSelectItem(b *strings.Builder, item SelectItem) {
	switch t := item.(type) {
	case *FieldRef:
		b.WriteString(strings.Join(t.Path, "."))
	case *SubQuery:
		b.WriteByte('(')
		b.WriteString(CanonicalQuery(t.Query))
		b.WriteByte(')')
	case *Aggregate:
		writeAggregate(b, t)
		if t.Alias != "" {
			b.WriteString(" AS ")
			b.WriteString(t.Alias)
		}
	case *Call:
		writeCall(b, t)
		if t.Alias != "" {
			b.WriteString(" AS ")
			b.WriteString(t.Alias)
		}
	case *FieldsAll:
		b.WriteString("FIELDS(")
		b.WriteString(t.Kind)
		b.WriteByte(')')
	case *TypeOf:
		writeTypeOf(b, t)
	}
}

func writeValueExpr(b *strings.Builder, v ValueExpr) {
	switch t := v.(type) {
	case *FieldRef:
		b.WriteString(strings.Join(t.Path, "."))
	case *Aggregate:
		writeAggregate(b, t)
		if t.Alias != "" {
			b.WriteString(" AS ")
			b.WriteString(t.Alias)
		}
	case *Call:
		writeCall(b, t)
		if t.Alias != "" {
			b.WriteString(" AS ")
			b.WriteString(t.Alias)
		}
	}
}

func writeAggregate(b *strings.Builder, a *Aggregate) {
	b.WriteString(a.Func)
	b.WriteByte('(')
	if a.Arg != nil {
		b.WriteString(strings.Join(a.Arg.Path, "."))
	}
	b.WriteByte(')')
}

func writeCall(b *strings.Builder, c *Call) {
	b.WriteString(c.Func)
	b.WriteByte('(')
	for i, arg := range c.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		writeValueExpr(b, arg)
	}
	b.WriteByte(')')
}

func writeTypeOf(b *strings.Builder, t *TypeOf) {
	b.WriteString("TYPEOF ")
	b.WriteString(string(t.Rel))
	for _, w := range t.Whens {
		b.WriteString(" WHEN ")
		b.WriteString(string(w.Type))
		b.WriteString(" THEN ")
		writeValueList(b, w.Fields)
	}
	if t.Else != nil {
		b.WriteString(" ELSE ")
		writeValueList(b, t.Else)
	}
	b.WriteString(" END")
}

func writeValueList(b *strings.Builder, vs []ValueExpr) {
	for i, v := range vs {
		if i > 0 {
			b.WriteString(", ")
		}
		writeValueExpr(b, v)
	}
}

func writeExpr(b *strings.Builder, e Expr) {
	switch t := e.(type) {
	case *And:
		b.WriteByte('(')
		for i, it := range t.Items {
			if i > 0 {
				b.WriteString(" AND ")
			}
			writeExpr(b, it)
		}
		b.WriteByte(')')
	case *Or:
		b.WriteByte('(')
		for i, it := range t.Items {
			if i > 0 {
				b.WriteString(" OR ")
			}
			writeExpr(b, it)
		}
		b.WriteByte(')')
	case *Not:
		b.WriteString("NOT (")
		writeExpr(b, t.Expr)
		b.WriteByte(')')
	case *Compare:
		writeValueExpr(b, t.Left)
		b.WriteByte(' ')
		b.WriteString(t.Op)
		b.WriteByte(' ')
		writeLiteral(b, t.Right)
	case *In:
		writeValueExpr(b, t.Left)
		if t.Not {
			b.WriteString(" NOT")
		}
		b.WriteString(" IN (")
		if t.Sub != nil {
			b.WriteString(CanonicalQuery(t.Sub))
		} else {
			writeLiterals(b, t.Values)
		}
		b.WriteByte(')')
	case *Includes:
		b.WriteString(strings.Join(t.Left.Path, "."))
		if t.Exclude {
			b.WriteString(" EXCLUDES (")
		} else {
			b.WriteString(" INCLUDES (")
		}
		writeLiterals(b, t.Values)
		b.WriteByte(')')
	}
}

func writeLiterals(b *strings.Builder, lits []Literal) {
	for i, l := range lits {
		if i > 0 {
			b.WriteString(", ")
		}
		writeLiteral(b, l)
	}
}

func writeLiteral(b *strings.Builder, l Literal) {
	switch l.Kind {
	case LitString:
		s := l.Pattern
		if s == "" {
			s = l.Text
		}
		writeQuoted(b, s)
	case LitDateLiteral:
		b.WriteString(l.Text)
		if l.N > 0 {
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(l.N))
		}
	case LitNull:
		b.WriteString("null")
	default:
		b.WriteString(l.Text)
	}
}

// writeQuoted re-serializes a LIKE-pattern encoded string literal. A backslash
// in the pattern encoding is the escape marker for \, % or _; emitting it as a
// single backslash reproduces the original spelling and re-parses to the same
// pattern. spec: soql-03
func writeQuoted(b *strings.Builder, s string) {
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\'':
			b.WriteString(`\'`)
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
}
