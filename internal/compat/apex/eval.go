package apex

import (
	"context"
	"errors"
	"math"
)

type runtimeValue struct {
	value Value
	rec   *record
	list  []*record
	typ   string
}

func scalar(v Value) runtimeValue {
	typ := "null"
	switch v.kind {
	case integerKind:
		typ = "Integer"
	case booleanKind:
		typ = "Boolean"
	case stringKind:
		typ = "String"
	}
	return runtimeValue{value: v, typ: typ}
}
func assignable(typ string, v runtimeValue) bool {
	if v.typ == "null" {
		return typ == "String" || typ == "Account" || typ == "List<Account>"
	}
	return typ == v.typ
}

type binding struct {
	typ   string
	value runtimeValue
}
type evaluator struct {
	ctx    context.Context
	tx     *transaction
	meter  *meter
	scopes []map[string]*binding
}

func evaluate(ctx context.Context, p *program, args []Value, tx *transaction, meter *meter) (Value, error) {
	e := &evaluator{ctx: ctx, tx: tx, meter: meter}
	params := map[string]*binding{}
	for i, a := range args {
		params[p.params[i].name] = &binding{typ: p.params[i].typ, value: scalar(a)}
	}
	v, returned, err := e.block(p.body, params)
	if err != nil {
		return Value{}, err
	}
	if !returned {
		return Value{}, p.body.tok.err("runtime")
	}
	if v.typ != "Integer" {
		return Value{}, p.body.tok.err("type")
	}
	return v.value, nil
}
func locate(err error, t token) error {
	var d *Diagnostic
	if errors.As(err, &d) && d.Line == 0 {
		return t.err(d.Code)
	}
	return err
}
func (e *evaluator) lookup(n string) (*binding, error) {
	for i := len(e.scopes) - 1; i >= 0; i-- {
		if b, ok := e.scopes[i][n]; ok {
			return b, nil
		}
	}
	return nil, diagnostic("type")
}
func (e *evaluator) block(s *statement, initial map[string]*binding) (runtimeValue, bool, error) {
	if initial == nil {
		initial = map[string]*binding{}
	}
	e.scopes = append(e.scopes, initial)
	defer func() { e.scopes = e.scopes[:len(e.scopes)-1] }()
	if err := e.meter.checkpoint(e.ctx); err != nil {
		return runtimeValue{}, false, locate(err, s.tok)
	}
	for _, st := range s.statements {
		v, ret, err := e.stmt(st)
		if err != nil || ret {
			return v, ret, err
		}
	}
	return runtimeValue{}, false, nil
}
func (e *evaluator) stmt(s *statement) (v runtimeValue, returned bool, err error) {
	defer func() { err = locate(err, s.tok) }()
	if err = e.meter.checkpoint(e.ctx); err != nil {
		return
	}
	switch s.op {
	case "block":
		return e.block(s, nil)
	case "declare":
		v, err = e.expr(s.expr)
		if err != nil {
			return
		}
		if !assignable(s.typ, v) {
			err = diagnostic("type")
			return
		}
		e.scopes[len(e.scopes)-1][s.name] = &binding{typ: s.typ, value: v}
	case "assign":
		if s.target.op == "variable" {
			var b *binding
			b, err = e.lookup(s.target.name)
			if err != nil {
				return
			}
			v, err = e.expr(s.expr)
			if err != nil {
				return
			}
			if !assignable(b.typ, v) {
				err = diagnostic("type")
				return
			}
			b.value = v
		} else {
			var target runtimeValue
			target, err = e.expr(s.target.left)
			if err != nil {
				return
			}
			if target.typ == "null" {
				err = diagnostic("runtime")
				return
			}
			if target.typ != "Account" || target.rec == nil {
				err = diagnostic("type")
				return
			}
			v, err = e.expr(s.expr)
			if err != nil {
				return
			}
			if v.typ != "String" {
				err = diagnostic("type")
				return
			}
			target.rec.account.Name = v.value.s
		}
	case "return":
		v, err = e.expr(s.expr)
		if err == nil && v.typ != "Integer" {
			err = diagnostic("type")
		}
		returned = err == nil
		return
	case "update":
		v, err = e.expr(s.expr)
		if err != nil {
			return
		}
		if v.typ == "null" {
			err = diagnostic("runtime")
			return
		}
		if v.typ != "List<Account>" {
			err = diagnostic("type")
			return
		}
		if len(v.list) > e.meter.limits.CollectionItems {
			err = diagnostic("budget")
			return
		}
		rows := make([]record, len(v.list))
		for i, r := range v.list {
			rows[i] = *r
		}
		err = e.tx.stage(e.ctx, rows)
	case "if":
		v, err = e.expr(s.expr)
		if err != nil {
			return
		}
		var b bool
		b, err = condition(v)
		if err != nil {
			return
		}
		if b {
			return e.block(s.body, nil)
		}
		if s.alternative != nil {
			return e.block(s.alternative, nil)
		}
	case "while":
		for {
			if err = e.meter.checkpoint(e.ctx); err != nil {
				return
			}
			v, err = e.expr(s.expr)
			if err != nil {
				return
			}
			var b bool
			b, err = condition(v)
			if err != nil || !b {
				return
			}
			v, returned, err = e.block(s.body, nil)
			if returned || err != nil {
				return
			}
		}
	case "for":
		v, err = e.expr(s.expr)
		if err != nil {
			return
		}
		if v.typ == "null" {
			err = diagnostic("runtime")
			return
		}
		if v.typ != "List<Account>" {
			err = diagnostic("type")
			return
		}
		for _, r := range v.list {
			if err = e.meter.checkpoint(e.ctx); err != nil {
				return
			}
			out, ret, er := e.block(s.body, map[string]*binding{s.name: {typ: "Account", value: runtimeValue{typ: "Account", rec: r}}})
			if ret || er != nil {
				return out, ret, er
			}
		}
	default:
		err = diagnostic("unsupported")
	}
	return runtimeValue{}, false, err
}
func condition(v runtimeValue) (bool, error) {
	if v.typ == "null" {
		return false, diagnostic("runtime")
	}
	if v.typ != "Boolean" {
		return false, diagnostic("type")
	}
	return v.value.b, nil
}
func (e *evaluator) expr(n *expression) (v runtimeValue, err error) {
	defer func() { err = locate(err, n.tok) }()
	if err = e.meter.checkpoint(e.ctx); err != nil {
		return
	}
	switch n.op {
	case "literal":
		return scalar(n.literal), nil
	case "variable":
		var b *binding
		b, err = e.lookup(n.name)
		if err != nil {
			return
		}
		return b.value, nil
	case "field":
		var a runtimeValue
		a, err = e.expr(n.left)
		if err != nil {
			return
		}
		if a.typ == "null" {
			return runtimeValue{}, diagnostic("runtime")
		}
		if a.typ != "Account" || a.rec == nil {
			return runtimeValue{}, diagnostic("type")
		}
		switch n.name {
		case "Id":
			return scalar(String(a.rec.account.ID)), nil
		case "Name":
			return scalar(String(a.rec.account.Name)), nil
		case "Probe_Group__c":
			return scalar(String(a.rec.account.Cohort)), nil
		}
		return runtimeValue{}, diagnostic("unsupported")
	case "query":
		var b *binding
		b, err = e.lookup(n.name)
		if err != nil {
			return
		}
		if b.value.typ != "String" {
			return runtimeValue{}, diagnostic("type")
		}
		var rows []record
		rows, err = e.tx.query(e.ctx, b.value.value.s)
		if err != nil {
			return
		}
		list := make([]*record, len(rows))
		for i := range rows {
			list[i] = &rows[i]
		}
		return runtimeValue{typ: "List<Account>", list: list}, nil
	case "unary!", "unary-":
		var a runtimeValue
		a, err = e.expr(n.left)
		if err != nil {
			return
		}
		if a.typ == "null" {
			return runtimeValue{}, diagnostic("runtime")
		}
		if n.op == "unary!" {
			if a.typ != "Boolean" {
				return runtimeValue{}, diagnostic("type")
			}
			return scalar(Boolean(!a.value.b)), nil
		}
		if a.typ != "Integer" {
			return runtimeValue{}, diagnostic("type")
		}
		return checked(-int64(a.value.i))
	default:
		var a, b runtimeValue
		a, err = e.expr(n.left)
		if err != nil {
			return
		}
		if n.op == "&&" || n.op == "||" {
			var ab bool
			ab, err = condition(a)
			if err != nil {
				return
			}
			if n.op == "&&" && !ab || n.op == "||" && ab {
				return scalar(Boolean(ab)), nil
			}
			b, err = e.expr(n.right)
			if err != nil {
				return
			}
			var bb bool
			bb, err = condition(b)
			return scalar(Boolean(bb)), err
		}
		b, err = e.expr(n.right)
		if err != nil {
			return
		}
		return e.binary(n.op, a, b)
	}
}
func checked(n int64) (runtimeValue, error) {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return runtimeValue{}, diagnostic("runtime")
	}
	return scalar(Integer(int32(n))), nil
}
func (e *evaluator) binary(op string, a, b runtimeValue) (runtimeValue, error) {
	if op == "==" || op == "!=" {
		eq := false
		if a.typ == "null" || b.typ == "null" {
			eq = a.typ == b.typ
		} else {
			if a.typ != b.typ {
				return runtimeValue{}, diagnostic("type")
			}
			switch a.typ {
			case "Integer", "Boolean", "String":
				eq = a.value == b.value
			default:
				return runtimeValue{}, diagnostic("type")
			}
		}
		if op == "!=" {
			eq = !eq
		}
		return scalar(Boolean(eq)), nil
	}
	if a.typ == "null" || b.typ == "null" {
		return runtimeValue{}, diagnostic("runtime")
	}
	if op == "+" && a.typ == "String" && b.typ == "String" {
		if len(b.value.s) > e.meter.limits.StringBytes-len(a.value.s) {
			return runtimeValue{}, diagnostic("budget")
		}
		return scalar(String(a.value.s + b.value.s)), nil
	}
	if a.typ != "Integer" || b.typ != "Integer" {
		return runtimeValue{}, diagnostic("type")
	}
	x, y := int64(a.value.i), int64(b.value.i)
	switch op {
	case "+":
		return checked(x + y)
	case "-":
		return checked(x - y)
	case "*":
		return checked(x * y)
	case "/":
		if y == 0 {
			return runtimeValue{}, diagnostic("runtime")
		}
		return checked(x / y)
	case "<":
		return scalar(Boolean(x < y)), nil
	case "<=":
		return scalar(Boolean(x <= y)), nil
	case ">":
		return scalar(Boolean(x > y)), nil
	case ">=":
		return scalar(Boolean(x >= y)), nil
	}
	return runtimeValue{}, diagnostic("unsupported")
}
