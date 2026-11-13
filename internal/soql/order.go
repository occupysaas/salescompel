package soql

import "github.com/jackc/pgx/v5"

// sortExpr renders an ORDER BY key. Plain fields sort on the stored value so
// numeric, temporal and boolean ordering is semantic rather than textual;
// functions and aggregates sort on their computed expression.
func (c *compiler) sortExpr(owner string, v BoundValue) string {
	if v.Agg != "" || v.Func != "" {
		return c.fieldExpr(owner, v)
	}
	return owner + "." + pgx.Identifier{v.Field.Column}.Sanitize()
}
