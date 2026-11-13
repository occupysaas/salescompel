package soql

import (
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/metadata"
)

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
