package soql

import (
	"encoding/json"
	"strings"

	"salescompel/internal/contract"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
	"salescompel/internal/recordid"
)

// bindExpr types a WHERE or HAVING tree.
// spec: soql-03
func (b *binder) bindExpr(e Expr, obj metadata.Object, ctx *bindCtx) (BoundExpr, error) {
	if e == nil {
		return nil, nil
	}
	switch t := e.(type) {
	case *And:
		items := make([]BoundExpr, 0, len(t.Items))
		for _, it := range t.Items {
			x, err := b.bindExpr(it, obj, ctx)
			if err != nil {
				return nil, err
			}
			items = append(items, x)
		}
		return &BoundAnd{Items: items}, nil
	case *Or:
		sub := *ctx
		sub.inOr = true
		items := make([]BoundExpr, 0, len(t.Items))
		for _, it := range t.Items {
			x, err := b.bindExpr(it, obj, &sub)
			if err != nil {
				return nil, err
			}
			items = append(items, x)
		}
		return &BoundOr{Items: items}, nil
	case *Not:
		x, err := b.bindExpr(t.Expr, obj, ctx)
		if err != nil {
			return nil, err
		}
		return &BoundNot{Expr: x}, nil
	case *Compare:
		return b.bindCompare(t, obj, ctx)
	case *In:
		return b.bindIn(t, obj, ctx)
	case *Includes:
		return b.bindIncludes(t, obj, ctx)
	}
	return nil, malformed("expected filter expression")
}

func (b *binder) bindCompare(c *Compare, obj metadata.Object, ctx *bindCtx) (BoundExpr, error) {
	// spec: soql-03, soql-04
	left, err := b.resolveValue(c.Left, obj, ctx)
	if err != nil {
		return nil, err
	}
	op := strings.ToUpper(c.Op)
	if left.Field.Name != "" && !left.Field.Filterable && left.Field.Kind() != fieldtype.MultiselectPicklist {
		return nil, contract.FieldProblem("INVALID_FIELD", "Field cannot be filtered", left.Field.Name)
	}
	if left.Label {
		return b.bindLabelCmp(left, op, c.Right)
	}
	if c.Right.Kind == LitDateLiteral && isTemporal(left.Field) {
		return b.bindDateLiteralCmp(left, op, c.Right)
	}
	if c.Right.Kind == LitNull {
		if op != "=" && op != "!=" {
			return nil, contract.Invalid("INVALID_QUERY_FILTER_OPERATOR", "operator is not valid for null")
		}
		return &BoundCmp{Left: left, Op: op, Right: Param{Value: nil}}, nil
	}
	if op == "LIKE" && (left.Agg != "" || left.Func != "" || !textLike(left.Field.Kind())) {
		return nil, contract.Invalid("INVALID_QUERY_FILTER_OPERATOR", "LIKE is not valid for this field type")
	}
	// Aggregates and functions type the operand by literal kind (HAVING).
	if left.Agg != "" || (left.Func != "" && !left.Label) {
		right, err := b.typeByKind(c.Right)
		if err != nil {
			return nil, err
		}
		return &BoundCmp{Left: left, Op: op, Right: right}, nil
	}
	right, err := b.typeLiteral(fieldOwner(obj, left), left.Field, c.Right)
	if err != nil {
		return nil, err
	}
	return &BoundCmp{Left: left, Op: op, Right: right}, nil
}

func (b *binder) bindLabelCmp(left BoundValue, op string, lit Literal) (BoundExpr, error) {
	if lit.Kind != LitString {
		return nil, wrongType(left.Field)
	}
	if op != "=" && op != "!=" {
		return nil, contract.Invalid("INVALID_QUERY_FILTER_OPERATOR", "operator is not valid for labels")
	}
	return &BoundCmp{Left: left, Op: op, Right: Param{Value: lit.Text, Cast: "text"}}, nil
}

// bindDateLiteralCmp rewrites a comparison against a date literal into
// half-open range predicates. spec: soql-04
func (b *binder) bindDateLiteralCmp(left BoundValue, op string, lit Literal) (BoundExpr, error) {
	start, end, err := DateLiteralRange(lit, b.now().UTC())
	if err != nil {
		return nil, err
	}
	cast := "timestamptz"
	if left.Field.Kind() == fieldtype.Date {
		cast = "date"
	}
	ge := BoundCmp{Left: left, Op: ">=", Right: Param{Value: start, Cast: cast}}
	lt := BoundCmp{Left: left, Op: "<", Right: Param{Value: end, Cast: cast}}
	switch op {
	case "=":
		return &BoundAnd{Items: []BoundExpr{&ge, &lt}}, nil
	case "!=", "<>":
		return &BoundNot{Expr: &BoundAnd{Items: []BoundExpr{&ge, &lt}}}, nil
	case "<":
		return &BoundCmp{Left: left, Op: "<", Right: Param{Value: start, Cast: cast}}, nil
	case "<=":
		return &lt, nil
	case ">":
		return &BoundCmp{Left: left, Op: ">=", Right: Param{Value: end, Cast: cast}}, nil
	case ">=":
		return &ge, nil
	}
	return nil, contract.Invalid("INVALID_QUERY_FILTER_OPERATOR", "operator is not valid for date literals")
}

func isTemporal(f metadata.Field) bool {
	k := f.Kind()
	return k == fieldtype.Date || k == fieldtype.DateTime
}

func textLike(k fieldtype.Kind) bool {
	switch k {
	case fieldtype.Text, fieldtype.TextArea, fieldtype.Email, fieldtype.Phone,
		fieldtype.Url, fieldtype.AutoNumber, fieldtype.Picklist, fieldtype.PersonName,
		fieldtype.Id:
		return true
	}
	return false
}

func fieldOwner(root metadata.Object, v BoundValue) metadata.Object {
	if len(v.Path) == 0 {
		return root
	}
	return v.Path[len(v.Path)-1].Object
}

func (b *binder) bindIn(in *In, obj metadata.Object, ctx *bindCtx) (BoundExpr, error) {
	// spec: soql-03, soql-06
	left, err := b.resolveValue(in.Left, obj, ctx)
	if err != nil {
		return nil, err
	}
	if left.Field.Name != "" && !left.Field.Filterable && left.Field.Kind() != fieldtype.MultiselectPicklist {
		return nil, contract.FieldProblem("INVALID_FIELD", "Field cannot be filtered", left.Field.Name)
	}
	if in.Sub != nil {
		return b.bindSemi(left, in, ctx)
	}
	if len(in.Values) > maxInList {
		return nil, tooComplicated("IN list is too long")
	}
	owner := fieldOwner(obj, left)
	vals := make([]Param, 0, len(in.Values))
	for _, lit := range in.Values {
		p, err := b.typeLiteral(owner, left.Field, lit)
		if err != nil {
			return nil, err
		}
		vals = append(vals, p)
	}
	return &BoundIn{Left: left, Not: in.Not, Values: vals}, nil
}

func (b *binder) bindSemi(left BoundValue, in *In, ctx *bindCtx) (BoundExpr, error) {
	// spec: soql-03
	if ctx.inOr {
		return nil, malformed("semi-join under OR is not supported")
	}
	if ctx.child {
		return nil, malformed("semi-join is not supported in subqueries")
	}
	k := left.Field.Kind()
	if k != fieldtype.Id && !k.Reference() {
		return nil, malformed("semi-join requires an Id or reference field")
	}
	sub := in.Sub
	if len(sub.GroupBy) > 0 || sub.Having != nil || len(sub.OrderBy) > 0 || sub.Limit != nil || sub.Offset != nil || sub.With != "" {
		return nil, malformed("semi-join subquery supports only WHERE")
	}
	if len(sub.Select) != 1 {
		return nil, malformed("semi-join subquery must select one Id or reference field")
	}
	ref, ok := sub.Select[0].(*FieldRef)
	if !ok {
		return nil, malformed("semi-join subquery must select one Id or reference field")
	}
	if len(ref.Path) != 1 {
		return nil, malformed("semi-join subquery must select one Id or reference field")
	}
	inner, err := b.query(sub, true)
	if err != nil {
		return nil, err
	}
	if len(inner.Columns) != 1 {
		return nil, malformed("semi-join subquery must select one Id or reference field")
	}
	sk := inner.Columns[0].Field.Kind()
	if sk != fieldtype.Id && !sk.Reference() {
		return nil, malformed("semi-join subquery must select one Id or reference field")
	}
	// Nesting is refused: the inner query is child=true, which already refuses
	// its own semi-joins via bindSemi's ctx.child check above.
	return &BoundIn{Left: left, Not: in.Not, Semi: inner}, nil
}

func (b *binder) bindIncludes(inc *Includes, obj metadata.Object, ctx *bindCtx) (BoundExpr, error) {
	// spec: soql-03
	left, err := b.resolveValue(inc.Left, obj, ctx)
	if err != nil {
		return nil, err
	}
	if left.Field.Kind() != fieldtype.MultiselectPicklist {
		return nil, contract.Invalid("INVALID_QUERY_FILTER_OPERATOR", "INCLUDES and EXCLUDES require a multi-select picklist")
	}
	if len(inc.Values) > maxInList {
		return nil, tooComplicated("INCLUDES list is too long")
	}
	vals := make([]Param, 0, len(inc.Values))
	for _, lit := range inc.Values {
		if lit.Kind != LitString {
			return nil, wrongType(left.Field)
		}
		vals = append(vals, Param{Value: lit.Text, Cast: "text"})
	}
	return &BoundIncludes{Left: left, Exclude: inc.Exclude, Values: vals}, nil
}

// --- literal typing ---

func wrongType(f metadata.Field) error {
	// spec: soql-03 — never echo the query value.
	name := f.Name
	typ := f.Type
	if typ == "" {
		typ = f.Kind().DescribeType()
	}
	return contract.FieldProblem("INVALID_FIELD",
		"value of filter criterion for field '"+name+"' must be of type "+typ+" and should not be enclosed in quotes",
		name)
}

// typeByKind types a literal for an aggregate or function result, which has
// no field describe type (HAVING). spec: soql-05
func (b *binder) typeByKind(lit Literal) (Param, error) {
	switch lit.Kind {
	case LitNull:
		return Param{Value: nil}, nil
	case LitBool:
		return Param{Value: lit.Text == "true", Cast: "bool"}, nil
	case LitNumber:
		s, err := fieldtype.Decimal(json.Number(lit.Text), 18, 8)
		if err != nil {
			return Param{}, malformed("invalid numeric literal")
		}
		return Param{Value: s, Cast: "numeric"}, nil
	case LitString:
		return Param{Value: lit.Text, Cast: "text"}, nil
	case LitDate:
		t, err := fieldtype.ParseDateTime(lit.Text + "T00:00:00Z")
		if err != nil {
			return Param{}, malformed("invalid date literal")
		}
		return Param{Value: t, Cast: "date"}, nil
	case LitDateTime:
		t, err := fieldtype.ParseDateTime(lit.Text)
		if err != nil {
			return Param{}, malformed("invalid date-time literal")
		}
		return Param{Value: t, Cast: "timestamptz"}, nil
	case LitDateLiteral:
		start, _, err := DateLiteralRange(lit, b.now().UTC())
		if err != nil {
			return Param{}, err
		}
		return Param{Value: start, Cast: "timestamptz"}, nil
	}
	return Param{}, malformed("invalid literal")
}

func (b *binder) typeLiteral(owner metadata.Object, f metadata.Field, lit Literal) (Param, error) {
	// spec: soql-03
	k := f.Kind()
	switch lit.Kind {
	case LitNull:
		return Param{Value: nil}, nil
	case LitBool:
		if k != fieldtype.Checkbox {
			return Param{}, wrongType(f)
		}
		return Param{Value: lit.Text == "true", Cast: "bool"}, nil
	case LitNumber:
		if !k.Numeric() {
			return Param{}, wrongType(f)
		}
		prec, scale := f.NumericBounds()
		s, err := fieldtype.Decimal(json.Number(lit.Text), prec, scale)
		if err != nil {
			return Param{}, wrongType(f)
		}
		return Param{Value: s, Cast: "numeric"}, nil
	case LitString:
		if k == fieldtype.Id || k.Reference() {
			return b.idParam(owner, f, lit.Text)
		}
		if !textLike(k) && k != fieldtype.MultiselectPicklist && k != fieldtype.TextArea {
			return Param{}, wrongType(f)
		}
		return Param{Value: lit.Text, Cast: "text"}, nil
	case LitDate:
		if k != fieldtype.Date && k != fieldtype.DateTime {
			return Param{}, wrongType(f)
		}
		if _, err := fieldtype.ParseDate(lit.Text); err != nil {
			return Param{}, wrongType(f)
		}
		// Spec: a date literal is a calendar day. On date-time fields it is
		// the UTC midnight instant (flagged deviation from a full-day range).
		t, err := fieldtype.ParseDateTime(lit.Text + "T00:00:00Z")
		if err != nil {
			return Param{}, wrongType(f)
		}
		cast := "timestamptz"
		if k == fieldtype.Date {
			cast = "date"
		}
		return Param{Value: t, Cast: cast}, nil
	case LitDateTime:
		if k != fieldtype.DateTime {
			return Param{}, wrongType(f)
		}
		t, err := fieldtype.ParseDateTime(lit.Text)
		if err != nil {
			return Param{}, wrongType(f)
		}
		return Param{Value: t, Cast: "timestamptz"}, nil
	case LitDateLiteral:
		if !isTemporal(f) {
			return Param{}, wrongType(f)
		}
		start, _, err := DateLiteralRange(lit, b.now().UTC())
		if err != nil {
			return Param{}, err
		}
		cast := "timestamptz"
		if k == fieldtype.Date {
			cast = "date"
		}
		return Param{Value: start, Cast: cast}, nil
	}
	return Param{}, wrongType(f)
}

func (b *binder) idParam(owner metadata.Object, f metadata.Field, input string) (Param, error) {
	// spec: soql-03
	prefix := owner.Prefix
	if f.Kind().Reference() && len(f.ReferenceTo) > 0 {
		if o, err := b.snap.Object(f.ReferenceTo[0]); err == nil {
			prefix = o.Prefix
		}
	}
	id, err := recordid.Normalize(prefix, input)
	if err != nil {
		return Param{}, contract.FieldProblem("INVALID_QUERY_FILTER_OPERATOR", "Invalid ID filter", f.Name)
	}
	return Param{Value: id, Cast: "text"}, nil
}

// resolveValue binds a group/having/order operand without assigning select keys.
func (b *binder) resolveValue(v ValueExpr, obj metadata.Object, ctx *bindCtx) (BoundValue, error) {
	switch t := v.(type) {
	case *FieldRef:
		path, f, err := b.resolvePath(obj, t.Path)
		if err != nil {
			return BoundValue{}, err
		}
		return BoundValue{Path: path, Field: f, Key: f.Name}, nil
	case *Aggregate:
		fn := strings.ToUpper(t.Func)
		var path []PathStep
		var f metadata.Field
		if t.Arg != nil {
			p, fl, err := b.resolvePath(obj, t.Arg.Path)
			if err != nil {
				return BoundValue{}, err
			}
			path, f = p, fl
			if err := checkAggKind(fn, f); err != nil {
				return BoundValue{}, err
			}
		} else if fn != "COUNT" {
			return BoundValue{}, malformed("expected field argument for " + fn)
		}
		return BoundValue{Path: path, Field: f, Agg: fn}, nil
	case *Call:
		if len(t.Args) != 1 {
			return BoundValue{}, malformed("function " + t.Func + " expects one field")
		}
		ref, ok := t.Args[0].(*FieldRef)
		if !ok {
			return BoundValue{}, malformed("function " + t.Func + " expects a field")
		}
		path, f, err := b.resolvePath(obj, ref.Path)
		if err != nil {
			return BoundValue{}, err
		}
		label := strings.EqualFold(t.Func, "toLabel")
		fn := t.Func
		if label {
			fn = "toLabel"
		} else {
			fn = strings.ToUpper(t.Func)
		}
		return BoundValue{Path: path, Field: f, Func: fn, Label: label}, nil
	}
	return BoundValue{}, malformed("expected field or function")
}

// bindOrderValue resolves an ORDER BY key, honouring select aliases.
func (b *binder) bindOrderValue(v ValueExpr, obj metadata.Object, ctx *bindCtx) (BoundValue, error) {
	if ref, ok := v.(*FieldRef); ok && len(ref.Path) == 1 {
		if av, ok := ctx.aliases[ref.Path[0]]; ok {
			return av, nil
		}
	}
	return b.resolveValue(v, obj, ctx)
}
