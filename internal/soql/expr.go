package soql

import (
	"strconv"
	"strings"
)

func (p *parser) valueList() ([]ValueExpr, error) {
	var out []ValueExpr
	for {
		v, err := p.valueExpr()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		if !p.at(Comma) {
			return out, nil
		}
		p.next()
	}
}

func (p *parser) valueExpr() (ValueExpr, error) {
	tok, ok := p.acceptName()
	if !ok {
		return nil, malformed("expected field or function")
	}
	if p.at(LParen) {
		return p.call(tok.Text, tok.Pos)
	}
	return p.fieldRef(tok.Text, tok.Pos)
}

// call parses COUNT/COUNT_DISTINCT/SUM/AVG/MIN/MAX, toLabel and the
// documented date functions; anything else is refused naming the function.
// spec: soql-02
func (p *parser) call(name string, pos Pos) (ValueExpr, error) {
	p.next() // (
	lower := strings.ToLower(name)
	switch {
	case aggFunc[lower]:
		fn := strings.ToUpper(name)
		if p.at(RParen) {
			p.next()
			if lower != "count" {
				return nil, malformed("expected field argument for " + fn)
			}
			return &Aggregate{Func: fn, Pos: pos}, nil
		}
		arg, err := p.argField()
		if err != nil {
			return nil, err
		}
		if err := p.expect(RParen, ")"); err != nil {
			return nil, err
		}
		return &Aggregate{Func: fn, Arg: arg, Pos: pos}, nil
	case lower == "tolabel" || dateFunc[lower]:
		fn := name
		if lower != "tolabel" {
			fn = strings.ToUpper(name)
		} else {
			fn = "toLabel"
		}
		arg, err := p.argField()
		if err != nil {
			return nil, err
		}
		if err := p.expect(RParen, ")"); err != nil {
			return nil, err
		}
		return &Call{Func: fn, Args: []ValueExpr{arg}, Pos: pos}, nil
	}
	return nil, malformed("function " + name + " is not supported")
}

func (p *parser) argField() (*FieldRef, error) {
	tok, ok := p.acceptName()
	if !ok {
		return nil, malformed("expected field argument")
	}
	if p.at(LParen) {
		return nil, malformed("expected field argument")
	}
	return p.fieldRef(tok.Text, tok.Pos)
}

// fieldRef builds a path node. pos is the first token of the reference.
func (p *parser) fieldRef(name string, pos Pos) (*FieldRef, error) {
	parts := strings.Split(name, ".")
	for _, s := range parts {
		if s == "" {
			return nil, malformed("malformed field path")
		}
	}
	return &FieldRef{Path: parts, Pos: pos}, nil
}

func (p *parser) orderItem() (OrderItem, error) {
	v, err := p.valueExpr()
	if err != nil {
		return OrderItem{}, err
	}
	item := OrderItem{Value: v}
	if p.atKeyword("desc") {
		item.Desc = true
		p.next()
	} else if p.atKeyword("asc") {
		p.next()
	}
	if p.atKeyword("nulls") {
		p.next()
		switch {
		case p.atKeyword("first"):
			item.Nulls = NullFirst
			p.next()
		case p.atKeyword("last"):
			item.Nulls = NullLast
			p.next()
		default:
			return OrderItem{}, malformed("expected NULLS FIRST or NULLS LAST")
		}
	}
	return item, nil
}

// expr parses one parenthesis level, refusing mixed AND/OR without parentheses.
// spec: soql-05
func (p *parser) expr() (Expr, error) {
	pos := p.peek().Pos
	first, err := p.term()
	if err != nil {
		return nil, err
	}
	items := []Expr{first}
	conn := ""
	for p.atKeyword("and") || p.atKeyword("or") {
		op := "and"
		if p.atKeyword("or") {
			op = "or"
		}
		if conn != "" && conn != op {
			return nil, malformed("mixed AND and OR require parentheses")
		}
		conn = op
		p.next()
		t, err := p.term()
		if err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if len(items) == 1 {
		return first, nil
	}
	if conn == "and" {
		return &And{Items: items, Pos: pos}, nil
	}
	return &Or{Items: items, Pos: pos}, nil
}

func (p *parser) term() (Expr, error) {
	if p.atKeyword("not") {
		pos := p.next().Pos
		e, err := p.term()
		if err != nil {
			return nil, err
		}
		return &Not{Expr: e, Pos: pos}, nil
	}
	return p.primary()
}

func (p *parser) primary() (Expr, error) {
	pos := p.peek().Pos
	if p.at(LParen) {
		p.next()
		e, err := p.expr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(RParen, ")"); err != nil {
			return nil, err
		}
		return e, nil
	}
	// spec: soql-03
	left, err := p.valueExpr()
	if err != nil {
		return nil, err
	}
	switch {
	case p.at(Op):
		op := p.next().Text
		right, err := p.literal()
		if err != nil {
			return nil, err
		}
		return &Compare{Left: left, Op: op, Right: right, Pos: pos}, nil
	case p.atKeyword("like"):
		p.next()
		right, err := p.literal()
		if err != nil {
			return nil, err
		}
		return &Compare{Left: left, Op: "LIKE", Right: right, Pos: pos}, nil
	case p.atKeyword("not") || p.atKeyword("in"):
		neg := false
		if p.atKeyword("not") {
			neg = true
			p.next()
		}
		if err := p.expectKeyword("in"); err != nil {
			return nil, err
		}
		return p.in(left, neg, pos)
	case p.atKeyword("includes") || p.atKeyword("excludes"):
		exclude := p.atKeyword("excludes")
		p.next()
		ref, ok := left.(*FieldRef)
		if !ok {
			return nil, malformed("INCLUDES and EXCLUDES require a field")
		}
		if err := p.expect(LParen, "("); err != nil {
			return nil, err
		}
		values, err := p.literalList()
		if err != nil {
			return nil, err
		}
		if err := p.expect(RParen, ")"); err != nil {
			return nil, err
		}
		return &Includes{Left: ref, Exclude: exclude, Values: values, Pos: pos}, nil
	}
	return nil, malformed("expected comparison operator")
}

func (p *parser) in(left ValueExpr, neg bool, pos Pos) (Expr, error) {
	if err := p.expect(LParen, "("); err != nil {
		return nil, err
	}
	if p.atKeyword("select") {
		q, err := p.query()
		if err != nil {
			return nil, err
		}
		if !p.at(RParen) {
			return nil, malformed("unterminated subquery: expected )")
		}
		p.next()
		return &In{Left: left, Not: neg, Sub: q, Pos: pos}, nil
	}
	values, err := p.literalList()
	if err != nil {
		return nil, err
	}
	if err := p.expect(RParen, ")"); err != nil {
		return nil, err
	}
	return &In{Left: left, Not: neg, Values: values, Pos: pos}, nil
}

func (p *parser) literalList() ([]Literal, error) {
	var out []Literal
	for {
		l, err := p.literal()
		if err != nil {
			return nil, err
		}
		out = append(out, l)
		if !p.at(Comma) {
			return out, nil
		}
		p.next()
	}
}

func (p *parser) literal() (Literal, error) {
	t := p.peek()
	switch t.Kind {
	case String:
		p.next()
		return Literal{Kind: LitString, Text: t.Text}, nil
	case Number:
		p.next()
		return Literal{Kind: LitNumber, Text: t.Text}, nil
	case Date:
		p.next()
		return Literal{Kind: LitDate, Text: t.Text}, nil
	case DateTime:
		p.next()
		return Literal{Kind: LitDateTime, Text: t.Text}, nil
	case DateLiteral:
		p.next()
		name := t.Text
		n := 0
		// The lexer has already required and validated the :n suffix, so the
		// digits are guaranteed to fit an int.
		if i := strings.IndexByte(name, ':'); i >= 0 {
			name, n = name[:i], digitsInt(name[i+1:])
		}
		return Literal{Kind: LitDateLiteral, Text: name, N: n}, nil
	case Keyword:
		if p.atKeyword("null") {
			p.next()
			return Literal{Kind: LitNull}, nil
		}
	case Ident:
		if strings.EqualFold(t.Text, "true") || strings.EqualFold(t.Text, "false") {
			p.next()
			return Literal{Kind: LitBool, Text: strings.ToLower(t.Text)}, nil
		}
	}
	return Literal{}, malformed("expected literal value")
}

func (p *parser) alias() (string, error) {
	if p.atKeyword("as") {
		p.next()
		if !p.at(Ident) {
			return "", malformed("expected alias name")
		}
		return p.next().Text, nil
	}
	if p.at(Ident) {
		return p.next().Text, nil
	}
	return "", nil
}

func (p *parser) intLit() (int, error) {
	if !p.at(Number) {
		return 0, malformed("expected integer")
	}
	t := p.next()
	if !digits(t.Text) {
		return 0, malformed("expected integer")
	}
	n, err := strconv.Atoi(t.Text)
	if err != nil {
		return 0, malformed("expected integer")
	}
	return n, nil
}
