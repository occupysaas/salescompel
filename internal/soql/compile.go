package soql

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/contract"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

// Statement is a parameterized SQL query safe to log: SQL contains $n
// placeholders, never literal values.
type Statement struct {
	SQL  string
	Args []any
}

// Plan is a compiled query with the metadata the shaper needs.
// Compile returns *Plan (rather than bare Statement) because the shaper
// needs JoinIDs, PathAlias and Compound alongside the SQL.
type Plan struct {
	Stmt      Statement
	Columns   []BoundColumn
	JoinIDs   map[string]int           // join alias -> SQL result index of hidden id
	PathAlias map[string]string        // path key -> join alias
	Compound  map[int][]metadata.Field // column index -> compound component fields
	LimitArg  int                      // index into Stmt.Args of the LIMIT parameter
	RootID    *int                     // hidden root id result index when children are present
}

type joinSpec struct {
	alias string
	table string
	on    string
}

type compiler struct {
	joins   []joinSpec
	joinMap map[string]string
	args    []any
	nJoin   int
}

// Compile turns a bound query into a parameterized SQL statement.
// spec: soql-01
func Compile(b *Bound) (*Plan, error) {
	if b == nil {
		return nil, malformed("empty bound query")
	}
	c := &compiler{joinMap: map[string]string{}}
	return c.compile(b)
}

func (c *compiler) compile(b *Bound) (*Plan, error) {
	root := "r"
	tenantArg := c.addArg(b.Object.Tenant)
	pageArg := c.pageArg(b)

	// Compile WHERE and ORDER BY first so ensureJoin runs before FROM is built.
	// Locator pages use captured IDs, never the re-evaluated user WHERE. spec: page-01
	where := fmt.Sprintf("%s.tenant_id=$%d", root, tenantArg)
	if b.Where != nil && b.PageIDs == nil {
		w, err := c.compileExpr(b.Where, root, b.Object)
		if err != nil {
			return nil, err
		}
		where += " AND (" + w + ")"
	}
	if b.PageIDs != nil {
		where += fmt.Sprintf(" AND %s.id = ANY($%d::text[])", root, pageArg)
	}
	order, err := c.compileOrder(b.Order, root, b.Object, b.GroupBy, b.Aggregate)
	if err != nil {
		return nil, err
	}
	if b.PageIDs != nil {
		order = fmt.Sprintf("array_position($%d::text[], %s.id::text)", pageArg, root)
	}
	group, err := c.compileGroupBy(b.GroupBy, root)
	if err != nil {
		return nil, err
	}
	having := ""
	if b.Having != nil {
		having, err = c.compileExpr(b.Having, root, b.Object)
		if err != nil {
			return nil, err
		}
	}

	selParts, compound, joinIDs, err := c.compileSelection(b, root)
	if err != nil {
		return nil, err
	}
	var rootID *int
	if len(b.Children) > 0 {
		idx := len(selParts)
		rootID = &idx
		selParts = append(selParts, root+".id")
	}

	from := b.Object.TableSQL() + " " + root
	for _, js := range c.joins {
		from += " LEFT JOIN " + js.table + " " + js.alias + " ON " + js.on
	}

	sql := "SELECT " + strings.Join(selParts, ",") + " FROM " + from + " WHERE " + where
	if group != "" {
		sql += " GROUP BY " + group
	}
	if having != "" {
		sql += " HAVING " + having
	}
	if order != "" {
		sql += " ORDER BY " + order
	} else if !b.Aggregate {
		sql += " ORDER BY " + root + ".id"
	}

	limit := 2000
	if b.Aggregate {
		limit = maxAggregateGroups
	}
	if b.Limit != nil {
		limit = *b.Limit
	}
	limitArg := c.addArg(limit)
	sql += fmt.Sprintf(" LIMIT $%d", limitArg)
	if b.Offset != nil {
		sql += fmt.Sprintf(" OFFSET $%d", c.addArg(*b.Offset))
	}

	pathAlias := map[string]string{}
	for k, v := range c.joinMap {
		pathAlias[k] = v
	}
	return &Plan{
		Stmt:      Statement{SQL: sql, Args: c.args},
		Columns:   b.Columns,
		JoinIDs:   joinIDs,
		PathAlias: pathAlias,
		Compound:  compound,
		LimitArg:  limitArg - 1, // zero-based index into Args
		RootID:    rootID,
	}, nil
}

func (c *compiler) compileSelection(b *Bound, root string) ([]string, map[int][]metadata.Field, map[string]int, error) {
	var parts []string
	compound := map[int][]metadata.Field{}
	for i, col := range b.Columns {
		ownerObj := b.Object
		owner := root
		if len(col.Path) > 0 {
			alias, err := c.ensureJoin(root, col.Path)
			if err != nil {
				return nil, nil, nil, err
			}
			owner = alias
			ownerObj = col.Path[len(col.Path)-1].Object
		}
		if col.Field.Compound() {
			comps, exprs, err := c.compoundExprs(owner, ownerObj, col.Field)
			if err != nil {
				return nil, nil, nil, err
			}
			compound[i] = comps
			parts = append(parts, exprs...)
			continue
		}
		parts = append(parts, c.selectFieldExpr(owner, BoundValue{
			Path: col.Path, Field: col.Field, Agg: col.Agg, Func: col.Func, Label: col.Label,
		}))
	}
	joinIDs := map[string]int{}
	base := len(parts)
	for i, js := range c.joins {
		if b.Aggregate {
			// MIN(id) is legal under GROUP BY and stays NULL when the left
			// join missed, preserving null parent paths in the shaper.
			parts = append(parts, "MIN("+js.alias+".id)")
		} else {
			parts = append(parts, js.alias+".id")
		}
		joinIDs[js.alias] = base + i
	}
	return parts, compound, joinIDs, nil
}

type childPlan struct {
	stmt Statement
	plan *Plan
}

// CompileChild compiles one child query for all parent IDs on a parent page.
// ROW_NUMBER applies LIMIT independently to each parent and leaves one extra
// row at the candidate ceiling so the executor can reject overflow.
// spec: child-01
func CompileChild(child BoundChild, parentIDs []string) (Statement, error) {
	plan, err := compileChildPlan(child, parentIDs)
	if err != nil {
		return Statement{}, err
	}
	return plan.stmt, nil
}

func compileChildPlan(child BoundChild, parentIDs []string) (*childPlan, error) {
	b := child.Query
	if b == nil {
		return nil, malformed("empty child query")
	}
	var refField metadata.Field
	for _, field := range b.Object.Fields {
		if strings.EqualFold(field.Name, child.Relationship.Field) {
			refField = field
			break
		}
	}
	if refField.Column == "" {
		return nil, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", child.Relationship.Field)
	}
	root := "r"
	c := &compiler{joinMap: map[string]string{}}
	tenantArg := c.addArg(b.Object.Tenant)
	parentsArg := c.addArg(parentIDs)
	ref := root + "." + pgx.Identifier{refField.Column}.Sanitize()
	where := fmt.Sprintf("%s.tenant_id=$%d AND %s = ANY($%d::text[])", root, tenantArg, ref, parentsArg)
	if b.Where != nil {
		w, err := c.compileExpr(b.Where, root, b.Object)
		if err != nil {
			return nil, err
		}
		where += " AND (" + w + ")"
	}
	order, err := c.compileOrder(b.Order, root, b.Object, nil, false)
	if err != nil {
		return nil, err
	}
	if order == "" {
		order = root + ".id"
	}
	parts, compound, joinIDs, err := c.compileSelection(b, root)
	if err != nil {
		return nil, err
	}
	baseParts := append([]string{ref + " AS __parent_id"}, parts...)
	baseParts = append(baseParts, "ROW_NUMBER() OVER (PARTITION BY "+ref+" ORDER BY "+order+") AS __child_rank")

	from := b.Object.TableSQL() + " " + root
	for _, js := range c.joins {
		from += " LEFT JOIN " + js.table + " " + js.alias + " ON " + js.on
	}
	baseSQL := "SELECT " + strings.Join(baseParts, ",") + " FROM " + from + " WHERE " + where
	fetch := maxChildRows + 1
	if b.Limit != nil && *b.Limit <= maxChildRows {
		fetch = *b.Limit
	}
	rankArg := c.addArg(fetch)
	sql := "SELECT * FROM (" + baseSQL + ") __children WHERE __child_rank <= $" + itoa(rankArg) +
		" ORDER BY __parent_id,__child_rank"
	return &childPlan{
		stmt: Statement{SQL: sql, Args: c.args},
		plan: &Plan{Columns: b.Columns, JoinIDs: joinIDs, PathAlias: c.joinMap, Compound: compound},
	}, nil
}

func (c *compiler) compoundExprs(owner string, obj metadata.Object, f metadata.Field) ([]metadata.Field, []string, error) {
	var comps []metadata.Field
	var exprs []string
	for _, name := range f.Options.Components {
		var cf metadata.Field
		found := false
		for _, cand := range obj.Fields {
			if strings.EqualFold(cand.Name, name) {
				cf, found = cand, true
				break
			}
		}
		if !found {
			return nil, nil, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", name)
		}
		comps = append(comps, cf)
		exprs = append(exprs, SelectExpression(owner, cf))
	}
	return comps, exprs, nil
}

// fieldExpr renders an ORDER BY value. Plain fields use the SELECT projection
// form, while aggregates and functions stay raw so ordering and grouping are
// exact.
func (c *compiler) fieldExpr(owner string, v BoundValue) string {
	if v.Agg != "" {
		return aggExpr(owner, v)
	}
	if v.Func != "" {
		return c.funcExpr(owner, v)
	}
	return SelectExpression(owner, v.Field)
}

// selectFieldExpr renders a SELECT-list value. Numeric aggregates are cast to
// text so WireValue can encode them exactly with DecimalJSON; predicates keep
// the raw numeric expression. spec: soql-05
func (c *compiler) selectFieldExpr(owner string, v BoundValue) string {
	if v.Agg != "" {
		expr := aggExpr(owner, v)
		if numericAggregate(v) {
			return "(" + expr + ")::text"
		}
		return expr
	}
	if v.Func != "" {
		return c.funcExpr(owner, v)
	}
	return SelectExpression(owner, v.Field)
}

func aggExpr(owner string, v BoundValue) string {
	col := owner + "." + pgx.Identifier{v.Field.Column}.Sanitize()
	switch v.Agg {
	case "COUNT":
		if v.Field.Name == "" {
			return "COUNT(*)"
		}
		return "COUNT(" + col + ")"
	case "COUNT_DISTINCT":
		return "COUNT(DISTINCT " + col + ")"
	default:
		return v.Agg + "(" + col + ")"
	}
}

// funcExpr renders a toLabel or date-function result. spec: soql-02, soql-05
func (c *compiler) funcExpr(owner string, v BoundValue) string {
	col := owner + "." + pgx.Identifier{v.Field.Column}.Sanitize()
	if v.Label {
		return c.labelExpr(col, v.Field)
	}
	return dateFuncSQL(v.Func, col)
}

// labelExpr maps picklist API values to their tenant labels. Values and labels
// are metadata, not query text, but stay parameters so no literal is inlined.
func (c *compiler) labelExpr(col string, f metadata.Field) string {
	if len(f.PicklistValues) == 0 {
		return col
	}
	if f.Kind() == fieldtype.MultiselectPicklist {
		return c.multiLabelExpr(col, f)
	}
	var b strings.Builder
	b.WriteString("CASE ")
	b.WriteString(col)
	for _, pv := range f.PicklistValues {
		label := pv.Label
		if label == "" {
			label = pv.Value
		}
		b.WriteString(" WHEN $")
		b.WriteString(itoa(c.addArg(pv.Value)))
		b.WriteString(" THEN $")
		b.WriteString(itoa(c.addArg(label)))
	}
	b.WriteString(" ELSE ")
	b.WriteString(col)
	b.WriteString(" END")
	return b.String()
}

// multiLabelExpr maps every element of a semicolon-joined multi-select value,
// preserving order and falling back to the API value for unlisted elements.
func (c *compiler) multiLabelExpr(col string, f metadata.Field) string {
	var values strings.Builder
	for i, pv := range f.PicklistValues {
		label := pv.Label
		if label == "" {
			label = pv.Value
		}
		if i > 0 {
			values.WriteString(",")
		}
		values.WriteString("($")
		values.WriteString(itoa(c.addArg(pv.Value)))
		values.WriteString("::text,$")
		values.WriteString(itoa(c.addArg(label)))
		values.WriteString("::text)")
	}
	return "(SELECT string_agg(COALESCE(m.label,u.e),';' ORDER BY u.ord) FROM unnest(string_to_array(" +
		col + ",';')) WITH ORDINALITY AS u(e,ord) LEFT JOIN (VALUES " + values.String() +
		") AS m(value,label) ON m.value=u.e)"
}

func (c *compiler) ensureJoin(root string, path []PathStep) (string, error) {
	if len(path) == 0 {
		return root, nil
	}
	key := pathKeyOf(path)
	if alias, ok := c.joinMap[key]; ok {
		return alias, nil
	}
	prev := root
	for i, step := range path {
		prefix := pathKeyOf(path[:i+1])
		if alias, ok := c.joinMap[prefix]; ok {
			prev = alias
			continue
		}
		c.nJoin++
		alias := fmt.Sprintf("p%d", c.nJoin)
		refCol := pgx.Identifier{step.Via.Column}.Sanitize()
		on := fmt.Sprintf("%s.tenant_id = %s.tenant_id AND %s.id = %s.%s", alias, root, alias, prev, refCol)
		c.joins = append(c.joins, joinSpec{alias: alias, table: step.Object.TableSQL(), on: on})
		c.joinMap[prefix] = alias
		prev = alias
	}
	return c.joinMap[key], nil
}

func (c *compiler) addArg(v any) int {
	c.args = append(c.args, v)
	return len(c.args)
}

func (c *compiler) compileOrder(order []BoundOrder, root string, obj metadata.Object, group []BoundValue, aggregate bool) (string, error) {
	parts := make([]string, 0, len(order)+1)
	for _, o := range order {
		owner := root
		if len(o.Value.Path) > 0 {
			alias, err := c.ensureJoin(root, o.Value.Path)
			if err != nil {
				return "", err
			}
			owner = alias
		}
		expr := c.sortExpr(owner, o.Value)
		part := expr
		if o.Desc {
			part += " DESC"
		} else {
			part += " ASC"
		}
		switch o.Nulls {
		case NullFirst:
			part += " NULLS FIRST"
		case NullLast:
			part += " NULLS LAST"
		default:
			// Plan 25 product default: nulls first in both directions.
			part += " NULLS FIRST"
		}
		parts = append(parts, part)
	}
	if aggregate {
		// Aggregate queries tie-break on the group keys, not Id: Id is not
		// grouped and would make the statement invalid. spec: soql-05, soql-06
		for _, g := range group {
			owner := root
			if len(g.Path) > 0 {
				alias, err := c.ensureJoin(root, g.Path)
				if err != nil {
					return "", err
				}
				owner = alias
			}
			parts = append(parts, c.sortExpr(owner, g))
		}
	} else {
		parts = append(parts, root+".id")
	}
	return strings.Join(parts, ","), nil
}

// compileGroupBy renders GROUP BY expressions. Group keys use raw SQL types
// (not the SELECT projections' text casts) so date functions and fields group
// exactly; projections may re-express the same grouped column. spec: soql-05
func (c *compiler) compileGroupBy(group []BoundValue, root string) (string, error) {
	parts := make([]string, 0, len(group))
	for _, g := range group {
		if g.Agg != "" {
			return "", malformed("aggregate functions are not groupable")
		}
		owner := root
		if len(g.Path) > 0 {
			alias, err := c.ensureJoin(root, g.Path)
			if err != nil {
				return "", err
			}
			owner = alias
		}
		if g.Func != "" {
			parts = append(parts, c.funcExpr(owner, g))
			continue
		}
		parts = append(parts, owner+"."+pgx.Identifier{g.Field.Column}.Sanitize())
	}
	return strings.Join(parts, ","), nil
}
