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
	name, ok := p.acceptName()
	if !ok {
		return nil, malformed("expected field or function")
	}
	if p.at(LParen) {
		return p.call(name)
	}
	return p.fieldRef(name)
}

// call parses COUNT/COUNT_DISTINCT/SUM/AVG/MIN/MAX, toLabel and the
// documented date functions; anything else is refused naming the function.
// spec: soql-02
func (p *parser) call(name string) (ValueExpr, error) {
	pos := p.next().Pos // (
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
	name, ok := p.acceptName()
	if !ok {
		return nil, malformed("expected field argument")
	}
	if p.at(LParen) {
		return nil, malformed("expected field argument")
	}
	return p.fieldRef(name)
}

func (p *parser) fieldRef(name string) (*FieldRef, error) {
	parts := strings.Split(name, ".")
	for _, s := range parts {
		if s == "" {
			return nil, malformed("malformed field path")
		}
	}
	return &FieldRef{Path: parts, Pos: p.peek().Pos}, nil
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
		return &And{Items: items, Pos: p.peek().Pos}, nil
	}
	return &Or{Items: items, Pos: p.peek().Pos}, nil
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
	pos := p.peek().Pos
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
		if err := p.expect(RParen, ")"); err != nil {
			return nil, err
		}
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
		if i := strings.IndexByte(name, ':'); i >= 0 {
			v, err := strconv.Atoi(name[i+1:])
			if err != nil {
				return Literal{}, malformed("malformed date literal")
			}
			name = name[:i]
			n = v
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
