package soql

import (
	"strconv"
	"strings"
	"time"

	"salescompel/internal/contract"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

// Bind resolves a parsed query against the tenant catalog, typing every
// literal and enforcing security, aggregate rules and structural limits.
// spec: soql-01, soql-06
func Bind(q *Query, snap metadata.Snapshot, p policy.Principal, now Clock) (*Bound, error) {
	if q == nil {
		return nil, malformed("empty query")
	}
	if now == nil {
		now = time.Now
	}
	p.Catalog = &snap
	b := &binder{snap: snap, p: p, now: now, joined: map[string]bool{}}
	return b.query(q, false)
}

type binder struct {
	snap   metadata.Snapshot
	p      policy.Principal
	now    Clock
	joined map[string]bool
}

type bindCtx struct {
	aliases map[string]BoundValue
	exprN   int
	inOr    bool // a semi-join under OR is refused
	child   bool // child subquery: refuse aggregates, GROUP BY, OFFSET, nesting
}

func (b *binder) query(q *Query, child bool) (*Bound, error) {
	obj, err := b.snap.Object(string(q.From))
	if err != nil {
		return nil, err
	}
	return b.queryObj(q, obj, child)
}

func (b *binder) queryObj(q *Query, obj metadata.Object, child bool) (*Bound, error) {
	// spec: soql-06
	if err := checkStructure(q); err != nil {
		return nil, err
	}
	if q.With == "SYSTEM_MODE" {
		return nil, malformed("WITH SYSTEM_MODE is not supported")
	}
	b.joined[obj.Name] = true
	ctx := &bindCtx{aliases: map[string]BoundValue{}, child: child}

	cols, children, err := b.bindSelect(q, obj, ctx)
	if err != nil {
		return nil, err
	}
	where, err := b.bindExpr(q.Where, obj, ctx)
	if err != nil {
		return nil, err
	}
	group, err := b.bindGroupBy(q, obj, ctx)
	if err != nil {
		return nil, err
	}
	having, err := b.bindExpr(q.Having, obj, ctx)
	if err != nil {
		return nil, err
	}
	order, err := b.bindOrder(q, obj, ctx)
	if err != nil {
		return nil, err
	}
	if err := b.bindLimitOffset(q, child); err != nil {
		return nil, err
	}
	// spec: soql-02 — FIELDS(ALL|CUSTOM) require LIMIT ≤ 200.
	for _, item := range q.Select {
		fa, ok := item.(*FieldsAll)
		if !ok || fa.Kind != "ALL" && fa.Kind != "CUSTOM" {
			continue
		}
		if q.Limit == nil || *q.Limit > maxFieldsLimit {
			return nil, malformed("FIELDS(" + fa.Kind + ") requires LIMIT at most 200")
		}
	}
	out := &Bound{
		Object: obj, Columns: cols, Children: children,
		Where: where, GroupBy: group, Having: having, Order: order,
		Limit: q.Limit, Offset: q.Offset,
	}
	if err := b.finishAggregates(q, out, child); err != nil {
		return nil, err
	}
	return out, nil
}

func (b *binder) bindLimitOffset(q *Query, child bool) error {
	// spec: soql-06
	if q.Offset != nil {
		if child {
			return malformed("OFFSET is not supported in subqueries")
		}
		if *q.Offset > maxOffset {
			return malformed("OFFSET is out of range")
		}
	}
	return nil
}

// --- select ---

func (b *binder) bindSelect(q *Query, obj metadata.Object, ctx *bindCtx) ([]BoundColumn, []BoundChild, error) {
	var cols []BoundColumn
	var children []BoundChild
	seen := map[string]bool{}
	for _, item := range q.Select {
		switch t := item.(type) {
		case *TypeOf:
			return nil, nil, malformed("TYPEOF is not supported")
		case *FieldsAll:
			exp, err := b.expandFields(obj, t)
			if err != nil {
				return nil, nil, err
			}
			for _, c := range exp {
				key := pathKey(c.Path, c.Field.Name)
				if seen[key] {
					return nil, nil, malformed("duplicate field in SELECT")
				}
				seen[key] = true
				cols = append(cols, c)
			}
		case *SubQuery:
			if ctx.child {
				return nil, nil, malformed("nested subqueries are not supported")
			}
			ch, err := b.bindChild(obj, t.Query)
			if err != nil {
				return nil, nil, err
			}
			children = append(children, ch)
		case *FieldRef:
			c, v, err := b.bindFieldRef(obj, t, ctx)
			if err != nil {
				return nil, nil, err
			}
			key := pathKey(c.Path, c.Field.Name)
			if seen[key] {
				return nil, nil, malformed("duplicate field in SELECT")
			}
			seen[key] = true
			cols = append(cols, c)
			if c.Key != "" {
				ctx.aliases[c.Key] = v
			}
		case *Aggregate:
			if ctx.child {
				return nil, nil, malformed("aggregates are not supported in subqueries")
			}
			c, v, err := b.bindAggregate(obj, t, ctx)
			if err != nil {
				return nil, nil, err
			}
			cols = append(cols, c)
			ctx.aliases[c.Key] = v
		case *Call:
			c, v, err := b.bindCall(obj, t, ctx)
			if err != nil {
				return nil, nil, err
			}
			cols = append(cols, c)
			ctx.aliases[c.Key] = v
		}
	}
	return cols, children, nil
}

func pathKey(path []PathStep, field string) string {
	parts := make([]string, 0, len(path)+1)
	for _, s := range path {
		parts = append(parts, s.Object.Name+"/"+s.Via.Name)
	}
	parts = append(parts, field)
	return strings.Join(parts, "|")
}

func (b *binder) bindFieldRef(obj metadata.Object, ref *FieldRef, ctx *bindCtx) (BoundColumn, BoundValue, error) {
	path, f, err := b.resolvePath(obj, ref.Path)
	if err != nil {
		return BoundColumn{}, BoundValue{}, err
	}
	key := f.Name
	c := BoundColumn{Key: key, Path: path, Field: f}
	v := BoundValue{Path: path, Field: f, Key: key}
	ctx.aliases[key] = v
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
		key = "expr" + strconv.Itoa(ctx.exprN)
		ctx.exprN++
	}
	c := BoundColumn{Key: key, Path: path, Field: f, Agg: fn}
	v := BoundValue{Path: path, Field: f, Agg: fn, Key: key}
	return c, v, nil
}

func (b *binder) bindCall(obj metadata.Object, call *Call, ctx *bindCtx) (BoundColumn, BoundValue, error) {
	// spec: soql-02
	if len(call.Args) != 1 {
		return BoundColumn{}, BoundValue{}, malformed("function " + call.Func + " expects one field")
	}
	ref, ok := call.Args[0].(*FieldRef)
	if !ok {
		return BoundColumn{}, BoundValue{}, malformed("function " + call.Func + " expects a field")
	}
	path, f, err := b.resolvePath(obj, ref.Path)
	if err != nil {
		return BoundColumn{}, BoundValue{}, err
	}
	label := strings.EqualFold(call.Func, "toLabel")
	fn := call.Func
	if label {
		fn = "toLabel"
		if k := f.Kind(); k != fieldtype.Picklist && k != fieldtype.MultiselectPicklist {
			return BoundColumn{}, BoundValue{}, malformed("toLabel expects a picklist field")
		}
	} else if !dateFunc[strings.ToLower(call.Func)] {
		return BoundColumn{}, BoundValue{}, malformed("function " + call.Func + " is not supported")
	} else {
		fn = strings.ToUpper(call.Func)
		if k := f.Kind(); k != fieldtype.Date && k != fieldtype.DateTime {
			return BoundColumn{}, BoundValue{}, malformed("function " + fn + " expects a date field")
		}
	}
	key := call.Alias
	if key == "" {
		key = "expr" + strconv.Itoa(ctx.exprN)
		ctx.exprN++
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

// --- paths ---

func (b *binder) resolvePath(obj metadata.Object, parts []string) ([]PathStep, metadata.Field, error) {
	if len(parts) == 0 {
		return nil, metadata.Field{}, malformed("malformed field path")
	}
	// spec: soql-06 — at most five relationship hops.
	if len(parts)-1 > maxPathLevels {
		return nil, metadata.Field{}, tooComplicated("relationship path is too deep")
	}
	cur := obj
	var path []PathStep
	for _, rel := range parts[:len(parts)-1] {
		target, via, err := b.resolveRel(cur, rel)
		if err != nil {
			return nil, metadata.Field{}, err
		}
		path = append(path, PathStep{Object: target, Via: via})
		cur = target
		b.joined[target.Name] = true
	}
	f, err := b.resolveField(cur, parts[len(parts)-1])
	if err != nil {
		return nil, metadata.Field{}, err
	}
	if len(b.joined) > maxJoinedObject {
		return nil, metadata.Field{}, tooComplicated("too many joined objects")
	}
	return path, f, nil
}

func (b *binder) resolveField(obj metadata.Object, name string) (metadata.Field, error) {
	// spec: soql-07
	f, err := b.snap.Field(obj.Name, name)
	if err != nil {
		return metadata.Field{}, err
	}
	if !policy.Allowed(b.p, obj.Name, f.Name) {
		return metadata.Field{}, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", f.Name)
	}
	return f, nil
}

func (b *binder) resolveRel(obj metadata.Object, name string) (metadata.Object, metadata.Field, error) {
	for _, f := range obj.Fields {
		if f.RelationshipName == nil || !strings.EqualFold(*f.RelationshipName, name) {
			continue
		}
		if !policy.Allowed(b.p, obj.Name, f.Name) {
			return metadata.Object{}, metadata.Field{}, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", name)
		}
		if len(f.ReferenceTo) == 0 {
			return metadata.Object{}, metadata.Field{}, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", name)
		}
		target, err := b.snap.Object(f.ReferenceTo[0])
		if err != nil {
			return metadata.Object{}, metadata.Field{}, err
		}
		return target, f, nil
	}
	return metadata.Object{}, metadata.Field{}, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", name)
}

// --- FIELDS() ---

func (b *binder) expandFields(obj metadata.Object, fa *FieldsAll) ([]BoundColumn, error) {
	// spec: soql-02, soql-07
	var cols []BoundColumn
	for _, f := range obj.Fields {
		custom := strings.HasSuffix(f.Name, "__c")
		switch fa.Kind {
		case "STANDARD":
			if custom {
				continue
			}
		case "CUSTOM":
			if !custom {
				continue
			}
		}
		if !policy.Allowed(b.p, obj.Name, f.Name) {
			continue
		}
		cols = append(cols, BoundColumn{Key: f.Name, Field: f})
	}
	return cols, nil
}

// --- child subqueries ---

func (b *binder) bindChild(parent metadata.Object, sub *Query) (BoundChild, error) {
	// spec: soql-02, child-01
	if sub.Having != nil || len(sub.GroupBy) > 0 {
		return BoundChild{}, malformed("GROUP BY and HAVING are not supported in subqueries")
	}
	name := string(sub.From)
	var rel metadata.ChildRelationship
	found := false
	for _, cr := range b.snap.ChildRelationships(parent.Name) {
		if strings.EqualFold(cr.RelationshipName, name) {
			rel, found = cr, true
			break
		}
	}
	if !found {
		return BoundChild{}, contract.FieldProblem("INVALID_FIELD", "Child relationship is unavailable", name)
	}
	childObj, err := b.snap.Object(rel.ChildSObject)
	if err != nil {
		return BoundChild{}, err
	}
	inner, err := b.queryObj(sub, childObj, true)
	if err != nil {
		return BoundChild{}, err
	}
	return BoundChild{Key: rel.RelationshipName, Relationship: rel, Query: inner}, nil
}
