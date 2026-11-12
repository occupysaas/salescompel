package soql

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/contract"
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
	if b.Aggregate {
		return nil, tooComplicated("aggregate queries are not yet supported")
	}
	c := &compiler{joinMap: map[string]string{}}
	return c.compile(b)
}

func (c *compiler) compile(b *Bound) (*Plan, error) {
	root := "r"
	tenantArg := c.addArg(b.Object.Tenant)

	// Compile WHERE and ORDER BY first so ensureJoin runs before FROM is built.
	where := fmt.Sprintf("%s.tenant_id=$%d", root, tenantArg)
	if b.Where != nil {
		w, err := c.compileExpr(b.Where, root, b.Object)
		if err != nil {
			return nil, err
		}
		where += " AND (" + w + ")"
	}
	order, err := c.compileOrder(b.Order, root, b.Object)
	if err != nil {
		return nil, err
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
	if order != "" {
		sql += " ORDER BY " + order
	} else {
		sql += " ORDER BY " + root + ".id"
	}

	limit := 2000
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
		parts = append(parts, c.fieldExpr(owner, BoundValue{
			Path: col.Path, Field: col.Field, Agg: col.Agg, Func: col.Func, Label: col.Label,
		}))
	}
	joinIDs := map[string]int{}
	base := len(parts)
	for i, js := range c.joins {
		parts = append(parts, js.alias+".id")
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
	order, err := c.compileOrder(b.Order, root, b.Object)
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

func (c *compiler) fieldExpr(owner string, v BoundValue) string {
	if v.Agg != "" {
		return aggExpr(owner, v)
	}
	if v.Func != "" {
		return funcExpr(owner, v)
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

func funcExpr(owner string, v BoundValue) string {
	col := owner + "." + pgx.Identifier{v.Field.Column}.Sanitize()
	if v.Label {
		return col
	}
	switch v.Func {
	case "DAY_ONLY":
		return "to_char(" + col + ` AT TIME ZONE 'UTC','YYYY-MM-DD')`
	case "CALENDAR_YEAR":
		return "EXTRACT(YEAR FROM " + col + ` AT TIME ZONE 'UTC')::int`
	case "CALENDAR_MONTH":
		return "EXTRACT(MONTH FROM " + col + ` AT TIME ZONE 'UTC')::int`
	case "CALENDAR_QUARTER":
		return "EXTRACT(QUARTER FROM " + col + ` AT TIME ZONE 'UTC')::int`
	case "DAY_IN_MONTH":
		return "EXTRACT(DAY FROM " + col + ` AT TIME ZONE 'UTC')::int`
	case "DAY_IN_WEEK":
		return "EXTRACT(DOW FROM " + col + ` AT TIME ZONE 'UTC')::int`
	case "DAY_IN_YEAR":
		return "EXTRACT(DOY FROM " + col + ` AT TIME ZONE 'UTC')::int`
	case "HOUR_IN_DAY":
		return "EXTRACT(HOUR FROM " + col + ` AT TIME ZONE 'UTC')::int`
	case "WEEK_IN_MONTH", "WEEK_IN_YEAR":
		return "EXTRACT(WEEK FROM " + col + ` AT TIME ZONE 'UTC')::int`
	default:
		return col
	}
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

func (c *compiler) compileOrder(order []BoundOrder, root string, obj metadata.Object) (string, error) {
	if len(order) == 0 {
		return "", nil
	}
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
		expr := c.fieldExpr(owner, o.Value)
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
	parts = append(parts, root+".id")
	return strings.Join(parts, ","), nil
}
