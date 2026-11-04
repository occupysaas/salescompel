package apex

import (
	"strconv"
)

type parser struct {
	tokens     []token
	pos, depth int
	limits     Limits
	err        error
	scopes     []map[string]bool
}

func compile(source string, l Limits) (*program, error) {
	if _, e := newMeter(l); e != nil {
		return nil, e
	}
	ts, e := lex(source, l)
	if e != nil {
		return nil, e
	}
	p := &parser{tokens: ts, limits: l}
	p.expect("public")
	p.expect("class")
	name := p.identifier()
	p.expect("{")
	p.expect("public")
	p.expect("static")
	p.expect("Integer")
	p.expect("run")
	p.expect("(")
	pr := &program{class: name}
	scope := map[string]bool{}
	if !p.at(")") {
		for p.err == nil {
			typ := p.typeName(true)
			n := p.identifier()
			if scope[n] {
				p.fail("syntax")
			}
			scope[n] = true
			pr.params = append(pr.params, parameter{typ, n})
			if !p.accept(",") {
				break
			}
		}
	}
	p.expect(")")
	pr.body = p.block(scope)
	p.expect("}")
	if p.peek().kind != "eof" {
		p.fail("syntax")
	}
	if p.err != nil {
		return nil, p.err
	}
	return pr, nil
}
func (p *parser) peek() token      { return p.tokens[p.pos] }
func (p *parser) at(s string) bool { return p.peek().text == s && p.peek().kind != "string" }
func (p *parser) take() token {
	t := p.peek()
	if t.kind != "eof" {
		p.pos++
	}
	return t
}
func (p *parser) fail(code string) {
	if p.err == nil {
		p.err = p.peek().err(code)
	}
}
func (p *parser) accept(s string) bool {
	if p.err == nil && p.at(s) {
		p.take()
		return true
	}
	return false
}
func (p *parser) expect(s string) {
	if !p.accept(s) {
		p.fail("syntax")
	}
}
func (p *parser) identifier() string {
	t := p.peek()
	if t.kind != "word" || wordIn(reserved, t.text) {
		p.fail("syntax")
		return ""
	}
	p.take()
	return t.text
}
func (p *parser) enter() bool {
	if p.depth >= p.limits.ParseDepth {
		p.fail("budget")
		return false
	}
	p.depth++
	return true
}
func (p *parser) typeName(scalar bool) string {
	t := p.take()
	if t.kind != "word" {
		p.err = t.err("syntax")
		return ""
	}
	switch t.text {
	case "Integer", "Boolean", "String":
		return t.text
	case "Account":
		if !scalar {
			return "Account"
		}
	case "List":
		if !scalar {
			p.expect("<")
			p.expect("Account")
			p.expect(">")
			return "List<Account>"
		}
	}
	p.err = t.err("unsupported")
	return ""
}
func (p *parser) block(initial map[string]bool) *statement {
	if !p.enter() {
		return nil
	}
	defer func() { p.depth-- }()
	t := p.peek()
	p.expect("{")
	if initial == nil {
		initial = map[string]bool{}
	}
	p.scopes = append(p.scopes, initial)
	defer func() { p.scopes = p.scopes[:len(p.scopes)-1] }()
	s := &statement{tok: t, op: "block"}
	for p.err == nil && !p.at("}") && p.peek().kind != "eof" {
		s.statements = append(s.statements, p.stmt())
	}
	p.expect("}")
	return s
}
func (p *parser) declare(n string) {
	s := p.scopes[len(p.scopes)-1]
	if s[n] {
		p.fail("syntax")
	}
	s[n] = true
}
func (p *parser) stmt() *statement {
	t := p.peek()
	s := &statement{tok: t}
	if t.kind == "string" || t.kind == "number" {
		p.fail("syntax")
		return s
	}
	switch t.text {
	case "{":
		return p.block(nil)
	case "Integer", "Boolean", "String", "Account", "List":
		s.op = "declare"
		s.typ = p.typeName(false)
		s.name = p.identifier()
		p.declare(s.name)
		p.expect("=")
		s.expr = p.expr(1)
		p.expect(";")
	case "if", "while":
		s.op = p.take().text
		p.expect("(")
		s.expr = p.expr(1)
		p.expect(")")
		s.body = p.block(nil)
		if s.op == "if" && p.accept("else") {
			s.alternative = p.block(nil)
		}
	case "for":
		p.take()
		s.op = "for"
		p.expect("(")
		p.expect("Account")
		s.name = p.identifier()
		p.expect(":")
		s.expr = p.expr(1)
		p.expect(")")
		s.body = p.block(map[string]bool{s.name: true})
	case "return", "update":
		s.op = p.take().text
		s.expr = p.expr(1)
		p.expect(";")
	default:
		s.op = "assign"
		s.target = p.primary()
		if p.err == nil && (s.target == nil || (s.target.op != "variable" && s.target.op != "field")) {
			p.fail("syntax")
		}
		if s.target != nil && s.target.op == "field" && s.target.name != "Name" {
			p.fail("unsupported")
		}
		p.expect("=")
		s.expr = p.expr(1)
		p.expect(";")
	}
	return s
}
func precedence(op string) int {
	switch op {
	case "||":
		return 1
	case "&&":
		return 2
	case "==", "!=":
		return 3
	case "<", "<=", ">", ">=":
		return 4
	case "+", "-":
		return 5
	case "*", "/":
		return 6
	}
	return 0
}
func (p *parser) node(t token, op string, a, b *expression) *expression {
	h := 1
	for _, n := range []*expression{a, b} {
		if n != nil && n.height >= h {
			h = n.height + 1
		}
	}
	if h > p.limits.ParseDepth && p.err == nil {
		p.err = t.err("budget")
	}
	return &expression{tok: t, op: op, left: a, right: b, height: h}
}
func (p *parser) expr(min int) *expression {
	if p.err != nil || !p.enter() {
		return nil
	}
	defer func() { p.depth-- }()
	left := p.unary()
	for p.err == nil {
		t := p.peek()
		prec := precedence(t.text)
		if t.kind == "string" || prec < min {
			break
		}
		p.take()
		right := p.expr(prec + 1)
		left = p.node(t, t.text, left, right)
	}
	return left
}
func (p *parser) unary() *expression {
	if p.err != nil {
		return nil
	}
	if p.at("!") || p.at("-") {
		if !p.enter() {
			return nil
		}
		defer func() { p.depth-- }()
		t := p.take()
		if t.text == "-" && p.peek().kind == "number" && p.peek().text == "2147483648" {
			p.take()
			n := p.node(t, "literal", nil, nil)
			n.literal = Integer(-2147483648)
			return n
		}
		return p.node(t, "unary"+t.text, p.unary(), nil)
	}
	return p.primary()
}
func (p *parser) primary() *expression {
	if p.err != nil {
		return nil
	}
	t := p.peek()
	var n *expression
	switch {
	case t.kind == "number":
		p.take()
		v, e := strconv.ParseInt(t.text, 10, 32)
		if e != nil {
			p.err = t.err("syntax")
		}
		n = p.node(t, "literal", nil, nil)
		n.literal = Integer(int32(v))
	case t.kind == "string":
		p.take()
		n = p.node(t, "literal", nil, nil)
		n.literal = String(t.text)
	case p.at("true") || p.at("false") || p.at("null"):
		p.take()
		n = p.node(t, "literal", nil, nil)
		if t.text != "null" {
			n.literal = Boolean(t.text == "true")
		}
	case p.accept("("):
		n = p.expr(1)
		p.expect(")")
	case p.accept("["):
		n = p.node(t, "query", nil, nil)
		for _, s := range []string{"SELECT", "Id", ",", "Name", ",", "Probe_Group__c", "FROM", "Account", "WHERE", "Probe_Group__c", "=", ":"} {
			p.expect(s)
		}
		n.name = p.identifier()
		p.expect("]")
	case t.kind == "word":
		if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].text == "(" {
			p.fail("unsupported")
			return nil
		}
		n = p.node(t, "variable", nil, nil)
		n.name = p.identifier()
	default:
		p.fail("syntax")
		return nil
	}
	if p.accept(".") {
		f := p.take()
		if f.kind != "word" || (f.text != "Id" && f.text != "Name" && f.text != "Probe_Group__c") {
			if p.err == nil {
				p.err = f.err("unsupported")
			}
		}
		n = p.node(f, "field", n, nil)
		n.name = f.text
	}
	if p.at("(") || p.at(".") {
		p.fail("unsupported")
	}
	return n
}
