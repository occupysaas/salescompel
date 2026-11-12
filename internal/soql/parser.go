package soql

import "strings"

type parser struct {
	toks []Token
	i    int
}

// Parse turns SOQL text into an AST. Keywords are case-insensitive; mixed
// AND/OR at one parenthesis level is refused.
// spec: soql-05
func Parse(text string) (*Query, error) {
	toks, err := Lex(text)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	q, err := p.query()
	if err != nil {
		return nil, err
	}
	if !p.at(EOF) {
		return nil, malformed("unexpected trailing input")
	}
	return q, nil
}

func (p *parser) query() (*Query, error) {
	// spec: soql-02
	q := &Query{Pos: p.peek().Pos}
	if err := p.expectKeyword("select"); err != nil {
		return nil, err
	}
	for {
		item, err := p.selectItem()
		if err != nil {
			return nil, err
		}
		q.Select = append(q.Select, item)
		if !p.at(Comma) {
			break
		}
		p.next()
	}
	if err := p.expectKeyword("from"); err != nil {
		return nil, err
	}
	name, ok := p.acceptName()
	if !ok {
		return nil, malformed("expected object or relationship name")
	}
	q.From = Name(name.Text)
	alias, err := p.alias()
	if err != nil {
		return nil, err
	}
	q.Alias = alias
	if p.atKeyword("where") {
		p.next()
		e, err := p.expr()
		if err != nil {
			return nil, err
		}
		q.Where = e
	}
	// spec: soql-05
	if p.atKeyword("group") {
		p.next()
		if err := p.expectKeyword("by"); err != nil {
			return nil, err
		}
		for {
			v, err := p.valueExpr()
			if err != nil {
				return nil, err
			}
			q.GroupBy = append(q.GroupBy, v)
			if !p.at(Comma) {
				break
			}
			p.next()
		}
	}
	if p.atKeyword("having") {
		p.next()
		e, err := p.expr()
		if err != nil {
			return nil, err
		}
		q.Having = e
	}
	if p.atKeyword("order") {
		p.next()
		if err := p.expectKeyword("by"); err != nil {
			return nil, err
		}
		for {
			item, err := p.orderItem()
			if err != nil {
				return nil, err
			}
			q.OrderBy = append(q.OrderBy, item)
			if !p.at(Comma) {
				break
			}
			p.next()
		}
	}
	if p.atKeyword("limit") {
		p.next()
		n, err := p.intLit()
		if err != nil {
			return nil, err
		}
		q.Limit = &n
	}
	if p.atKeyword("offset") {
		p.next()
		n, err := p.intLit()
		if err != nil {
			return nil, err
		}
		q.Offset = &n
	}
	if p.atKeyword("with") {
		p.next()
		mode, ok := p.acceptName()
		if !ok {
			return nil, malformed("expected WITH mode")
		}
		m := strings.ToUpper(mode.Text)
		switch m {
		case "SECURITY_ENFORCED", "USER_MODE", "SYSTEM_MODE":
			q.With = m
		default:
			return nil, malformed("unsupported WITH clause")
		}
	}
	return q, nil
}

func (p *parser) selectItem() (SelectItem, error) {
	if p.at(LParen) {
		pos := p.next().Pos
		q, err := p.query()
		if err != nil {
			return nil, err
		}
		if !p.at(RParen) {
			return nil, malformed("unterminated subquery: expected )")
		}
		p.next()
		return &SubQuery{Query: q, Pos: pos}, nil
	}
	if p.atKeyword("typeof") {
		return p.typeOf()
	}
	tok, ok := p.acceptName()
	if !ok {
		return nil, malformed("expected select item")
	}
	if !p.at(LParen) {
		return p.fieldRef(tok.Text, tok.Pos)
	}
	if strings.EqualFold(tok.Text, "fields") {
		return p.fieldsAll(tok.Pos)
	}
	v, err := p.call(tok.Text, tok.Pos)
	if err != nil {
		return nil, err
	}
	alias, err := p.alias()
	if err != nil {
		return nil, err
	}
	switch t := v.(type) {
	case *Aggregate:
		t.Alias = alias
	case *Call:
		t.Alias = alias
	}
	item, ok := v.(SelectItem)
	if !ok {
		return nil, malformed("expected select item")
	}
	return item, nil
}

func (p *parser) fieldsAll(pos Pos) (SelectItem, error) {
	p.next() // (
	kind, ok := p.acceptName()
	if !ok {
		return nil, malformed("expected FIELDS kind")
	}
	k := strings.ToUpper(kind.Text)
	if k != "ALL" && k != "CUSTOM" && k != "STANDARD" {
		return nil, malformed("FIELDS kind must be ALL, CUSTOM or STANDARD")
	}
	if err := p.expect(RParen, ")"); err != nil {
		return nil, err
	}
	return &FieldsAll{Kind: k, Pos: pos}, nil
}

func (p *parser) typeOf() (SelectItem, error) {
	pos := p.next().Pos // TYPEOF
	rel, ok := p.acceptName()
	if !ok {
		return nil, malformed("expected TYPEOF relationship")
	}
	t := &TypeOf{Rel: Name(rel.Text), Pos: pos}
	for p.atKeyword("when") {
		p.next()
		typ, ok := p.acceptName()
		if !ok {
			return nil, malformed("expected TYPEOF type")
		}
		if err := p.expectKeyword("then"); err != nil {
			return nil, err
		}
		fields, err := p.valueList()
		if err != nil {
			return nil, err
		}
		t.Whens = append(t.Whens, TypeOfWhen{Type: Name(typ.Text), Fields: fields})
	}
	if len(t.Whens) == 0 {
		return nil, malformed("TYPEOF requires at least one WHEN branch")
	}
	if p.atKeyword("else") {
		p.next()
		fields, err := p.valueList()
		if err != nil {
			return nil, err
		}
		t.Else = fields
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return t, nil
}

func (p *parser) peek() Token { return p.toks[p.i] }

func (p *parser) at(k TokenKind) bool { return p.peek().Kind == k }

func (p *parser) atKeyword(w string) bool {
	t := p.peek()
	return t.Kind == Keyword && strings.EqualFold(t.Text, w)
}

func (p *parser) next() Token {
	t := p.toks[p.i]
	p.i++
	return t
}

func (p *parser) expect(k TokenKind, what string) error {
	if !p.at(k) {
		return malformed("expected " + what)
	}
	p.next()
	return nil
}

func (p *parser) expectKeyword(w string) error {
	if !p.atKeyword(w) {
		return malformed("expected " + strings.ToUpper(w))
	}
	p.next()
	return nil
}

// acceptName takes a word where a name is expected and returns its token.
// Clause keywords are refused so a duplicated clause stays an error, while
// Order remains a name.
func (p *parser) acceptName() (Token, bool) {
	t := p.peek()
	switch t.Kind {
	case Ident:
		p.next()
		return t, true
	case Keyword:
		if clauseKeyword[strings.ToLower(t.Text)] {
			return Token{}, false
		}
		p.next()
		return t, true
	}
	return Token{}, false
}
