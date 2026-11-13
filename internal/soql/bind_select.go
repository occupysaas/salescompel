package soql

import (
	"strings"

	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

// --- select items: fields, aggregates and functions ---

func (b *binder) bindFieldRef(obj metadata.Object, ref *FieldRef, ctx *bindCtx) (BoundColumn, BoundValue, error) {
	path, f, err := b.resolvePath(obj, ref.Path)
	if err != nil {
		return BoundColumn{}, BoundValue{}, err
	}
	key := f.Name
	c := BoundColumn{Key: key, Path: path, Field: f}
	v := BoundValue{Path: path, Field: f, Key: key}
	return c, v, nil
}

func (b *binder) bindAggregate(obj metadata.Object, a *Aggregate, ctx *bindCtx) (BoundColumn, BoundValue, error) {
	// spec: soql-02
	fn := strings.ToUpper(a.Func)
	var path []PathStep
	var f metadata.Field
	if a.Arg != nil {
		p, fl, err := b.resolvePath(obj, a.Arg.Path)
		if err != nil {
			return BoundColumn{}, BoundValue{}, err
		}
		path, f = p, fl
		if err := checkAggKind(fn, f); err != nil {
			return BoundColumn{}, BoundValue{}, err
		}
	} else if fn != "COUNT" {
		return BoundColumn{}, BoundValue{}, malformed("expected field argument for " + fn)
	}
	key := a.Alias
	if key == "" {
		key = ctx.nextExprKey()
	}
	c := BoundColumn{Key: key, Path: path, Field: f, Agg: fn}
	v := BoundValue{Path: path, Field: f, Agg: fn, Key: key}
	return c, v, nil
}

// callArg resolves a toLabel or date-function call's single field argument
// and applies the function's field-kind rules. SELECT, WHERE, GROUP BY,
// HAVING and ORDER BY share it so the same rules hold in every clause.
// spec: soql-02
func (b *binder) callArg(obj metadata.Object, call *Call) ([]PathStep, metadata.Field, string, bool, error) {
	if len(call.Args) != 1 {
		return nil, metadata.Field{}, "", false, malformed("function " + call.Func + " expects one field")
	}
	ref, ok := call.Args[0].(*FieldRef)
	if !ok {
		return nil, metadata.Field{}, "", false, malformed("function " + call.Func + " expects a field")
	}
	path, f, err := b.resolvePath(obj, ref.Path)
	if err != nil {
		return nil, metadata.Field{}, "", false, err
	}
	label := strings.EqualFold(call.Func, "toLabel")
	if label {
		if k := f.Kind(); k != fieldtype.Picklist && k != fieldtype.MultiselectPicklist {
			return nil, metadata.Field{}, "", false, malformed("toLabel expects a picklist field")
		}
		return path, f, "toLabel", true, nil
	}
	if !dateFunc[strings.ToLower(call.Func)] {
		return nil, metadata.Field{}, "", false, malformed("function " + call.Func + " is not supported")
	}
	fn := strings.ToUpper(call.Func)
	if k := f.Kind(); k != fieldtype.Date && k != fieldtype.DateTime {
		return nil, metadata.Field{}, "", false, malformed("function " + fn + " expects a date field")
	}
	return path, f, fn, false, nil
}

func (b *binder) bindCall(obj metadata.Object, call *Call, ctx *bindCtx) (BoundColumn, BoundValue, error) {
	// spec: soql-02
	path, f, fn, label, err := b.callArg(obj, call)
	if err != nil {
		return BoundColumn{}, BoundValue{}, err
	}
	key := call.Alias
	if key == "" {
		key = ctx.nextExprKey()
	}
	c := BoundColumn{Key: key, Path: path, Field: f, Func: fn, Label: label}
	v := BoundValue{Path: path, Field: f, Func: fn, Label: label, Key: key}
	return c, v, nil
}

func checkAggKind(fn string, f metadata.Field) error {
	// spec: soql-02
	k := f.Kind()
	ok := false
	switch fn {
	case "SUM", "AVG":
		ok = k.Numeric()
	case "MIN", "MAX":
		ok = k.Numeric() || k == fieldtype.Date || k == fieldtype.DateTime || k.Filterable()
	case "COUNT", "COUNT_DISTINCT":
		ok = f.Filterable && k != fieldtype.LongTextArea
	}
	if !ok {
		return malformed("aggregate function does not accept this field type")
	}
	return nil
}
