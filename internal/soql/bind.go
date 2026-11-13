package soql

import (
	"strconv"
	"strings"
	"time"

	"salescompel/internal/contract"
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
	aliases  map[string]BoundValue // select aliases, lower-cased keys
	keys     map[string]bool       // result keys already taken, lower-cased
	keyPaths map[string]string     // key → path signature for FieldRef dedup
	reserve  map[string]bool       // explicit aliases that own their exprN
	exprN    int
	inOr     bool // a semi-join under OR is refused
	child    bool // child subquery: refuse aggregates, GROUP BY, OFFSET, nesting
	having   bool // binding HAVING: aggregates are legal here, not in WHERE
}

// nextExprKey numbers unaliased aggregate and function items expr0, expr1, …
// among themselves, skipping any number an explicit alias already owns.
// spec: soql-02
func (c *bindCtx) nextExprKey() string {
	for {
		key := "expr" + strconv.Itoa(c.exprN)
		c.exprN++
		if !c.reserve[strings.ToLower(key)] && !c.keys[strings.ToLower(key)] {
			return key
		}
	}
}

// takeKey records a SELECT result key, refusing duplicates.
// spec: soql-02
func (c *bindCtx) takeKey(key string) error {
	lower := strings.ToLower(key)
	if c.keys[lower] {
		return malformed("duplicate field selected")
	}
	c.keys[lower] = true
	return nil
}

// recordKey records a FieldRef result key. Exact same-field duplicates (same
// key, same path) collapse. Distinct paths sharing a leaf keep their path
// identity and shape to distinct JSON keys (root Name versus Account.Name), so
// they bind. A key already taken by an alias, generated expr, child or
// expanded FIELDS() entry still rejects.
func (c *bindCtx) recordKey(key, pkey string) error {
	lower := strings.ToLower(key)
	if existing, ok := c.keyPaths[lower]; ok {
		if existing == pkey {
			return nil // same field, same path: collapse
		}
		return nil // distinct path: separate shaped key
	}
	if c.keys[lower] {
		return malformed("duplicate field selected")
	}
	c.keys[lower] = true
	c.keyPaths[lower] = pkey
	return nil
}

// reserveAliases marks explicit select aliases so generated exprN keys skip
// numbers the aliases own. spec: soql-02
func reserveAliases(items []SelectItem) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		switch t := item.(type) {
		case *Aggregate:
			if t.Alias != "" {
				out[strings.ToLower(t.Alias)] = true
			}
		case *Call:
			if t.Alias != "" {
				out[strings.ToLower(t.Alias)] = true
			}
		}
	}
	return out
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
	ctx := &bindCtx{
		aliases:  map[string]BoundValue{},
		keys:     map[string]bool{},
		keyPaths: map[string]string{},
		reserve:  reserveAliases(q.Select),
		child:    child,
	}

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
	hctx := *ctx
	hctx.having = true
	having, err := b.bindExpr(q.Having, obj, &hctx)
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
		Source: CanonicalQuery(q), Revision: b.snap.Revision,
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
				if err := ctx.takeKey(c.Key); err != nil {
					return nil, nil, err
				}
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
			if err := ctx.takeKey(ch.Key); err != nil {
				return nil, nil, err
			}
			children = append(children, ch)
		case *FieldRef:
			c, _, err := b.bindFieldRef(obj, t, ctx)
			if err != nil {
				return nil, nil, err
			}
			if err := ctx.recordKey(c.Key, pathKeyOf(c.Path)); err != nil {
				return nil, nil, err
			}
			cols = append(cols, c)
		case *Aggregate:
			if ctx.child {
				return nil, nil, malformed("aggregates are not supported in subqueries")
			}
			c, v, err := b.bindAggregate(obj, t, ctx)
			if err != nil {
				return nil, nil, err
			}
			if err := ctx.takeKey(c.Key); err != nil {
				return nil, nil, err
			}
			cols = append(cols, c)
			ctx.aliases[strings.ToLower(c.Key)] = v
		case *Call:
			c, v, err := b.bindCall(obj, t, ctx)
			if err != nil {
				return nil, nil, err
			}
			if err := ctx.takeKey(c.Key); err != nil {
				return nil, nil, err
			}
			cols = append(cols, c)
			ctx.aliases[strings.ToLower(c.Key)] = v
		}
	}
	return cols, children, nil
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
	// Traversing the relationship reads the child's foreign-key field, so that
	// field's policy applies even when the inner projection omits it.
	// spec: soql-02, soql-06
	if !policy.Allowed(b.p, childObj.Name, rel.Field) {
		return BoundChild{}, contract.FieldProblem("INVALID_FIELD", "Child relationship is unavailable", name)
	}
	inner, err := b.queryObj(sub, childObj, true)
	if err != nil {
		return BoundChild{}, err
	}
	return BoundChild{Key: rel.RelationshipName, Relationship: rel, Query: inner}, nil
}

// --- GROUP BY, ORDER BY, aggregates ---

func (b *binder) bindGroupBy(q *Query, obj metadata.Object, ctx *bindCtx) ([]BoundValue, error) {
	// spec: soql-05
	if ctx.child && len(q.GroupBy) > 0 {
		return nil, malformed("GROUP BY is not supported in subqueries")
	}
	var out []BoundValue
	for _, g := range q.GroupBy {
		v, err := b.resolveValue(g, obj, ctx)
		if err != nil {
			return nil, err
		}
		if v.Agg != "" {
			return nil, malformed("aggregate functions are not groupable")
		}
		// GROUP BY is fields, parent paths and date functions only. Labels are
		// per-row expressions and are refused here. spec: soql-05
		if v.Label {
			return nil, malformed("toLabel is not groupable")
		}
		if v.Func == "" && !v.Field.Groupable {
			return nil, malformed("field cannot be grouped")
		}
		out = append(out, v)
	}
	return out, nil
}

func (b *binder) bindOrder(q *Query, obj metadata.Object, ctx *bindCtx) ([]BoundOrder, error) {
	// spec: soql-05
	if len(q.OrderBy) > maxOrderKeys {
		return nil, malformed("too many ORDER BY keys")
	}
	var out []BoundOrder
	for _, o := range q.OrderBy {
		v, err := b.bindOrderValue(o.Value, obj, ctx)
		if err != nil {
			return nil, err
		}
		// ORDER BY accepts fields, paths, date functions, aggregates and their
		// aliases. toLabel is not an order key; refusing it here avoids an
		// expression the compiler would have to discard on a locator page.
		// spec: soql-05
		if v.Label {
			return nil, malformed("toLabel is not supported in ORDER BY")
		}
		if v.Agg == "" && v.Func == "" && v.Field.Name != "" && !v.Field.Sortable {
			return nil, contract.FieldProblem("INVALID_FIELD", "Field cannot be sorted", v.Field.Name)
		}
		out = append(out, BoundOrder{Value: v, Desc: o.Desc, Nulls: o.Nulls})
	}
	return out, nil
}

func (b *binder) finishAggregates(q *Query, out *Bound, child bool) error {
	// spec: soql-02
	hasAgg := false
	countOnly := false
	plain := false
	for _, c := range out.Columns {
		switch {
		case c.Agg != "":
			hasAgg = true
			if c.Agg == "COUNT" && c.Field.Name == "" {
				countOnly = true
			}
		default:
			// Functions are per-row values: like plain fields they cannot be
			// mixed with aggregates without GROUP BY. spec: soql-02
			plain = true
		}
	}
	if child && hasAgg {
		return malformed("aggregates are not supported in subqueries")
	}
	if hasAgg || len(out.GroupBy) > 0 {
		out.Aggregate = true
	}
	// GROUP BY makes an aggregate query even without an aggregate function;
	// either form is a single grouped statement and cannot carry child rows.
	if out.Aggregate && len(out.Children) > 0 {
		return malformed("aggregate queries cannot be combined with subqueries")
	}
	// COUNT() is an envelope-only count; grouping would need result rows.
	if countOnly && len(out.GroupBy) > 0 {
		return malformed("COUNT() cannot be used with GROUP BY")
	}
	out.CountOnly = countOnly && hasAgg && len(out.Columns) == 1 && len(out.GroupBy) == 0
	if hasAgg && len(out.GroupBy) == 0 && plain {
		return malformed("aggregates cannot be mixed with plain fields without GROUP BY")
	}
	// spec: soql-05 — HAVING is an aggregate-query clause.
	if out.Having != nil && !out.Aggregate {
		return malformed("HAVING requires aggregation or GROUP BY")
	}
	// Aggregate and grouped queries share one grouped-key rule. With no GROUP
	// BY the key set is empty, so any non-aggregate SELECT, ORDER BY or HAVING
	// operand fails here instead of reaching PostgreSQL as a server error.
	// spec: soql-05
	if out.Aggregate {
		if err := checkGrouped(out); err != nil {
			return err
		}
	}
	return nil
}
