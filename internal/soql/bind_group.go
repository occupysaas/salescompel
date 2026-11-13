package soql

// valueSig identifies a bound value's SQL expression by catalog identity,
// ignoring select aliases: two operands sharing a signature compile to the
// same grouped expression. spec: soql-05
func valueSig(v BoundValue) string {
	label := "0"
	if v.Label {
		label = "1"
	}
	return pathKeyOf(v.Path) + "|" + v.Field.Name + "|" + v.Agg + "|" + v.Func + "|" + label
}

// checkGrouped enforces that every per-row SELECT, HAVING and ORDER BY operand
// in a GROUP BY query is a group key; aggregates are always legal. PostgreSQL
// would otherwise reject the statement at execution time. spec: soql-05
func checkGrouped(out *Bound) error {
	keys := make(map[string]bool, len(out.GroupBy))
	for _, g := range out.GroupBy {
		keys[valueSig(g)] = true
	}
	for _, c := range out.Columns {
		if c.Agg != "" {
			continue
		}
		v := BoundValue{Path: c.Path, Field: c.Field, Func: c.Func, Label: c.Label}
		if err := checkGroupedValue(v, keys); err != nil {
			return err
		}
	}
	for _, o := range out.Order {
		if err := checkGroupedValue(o.Value, keys); err != nil {
			return err
		}
	}
	return checkGroupedExpr(out.Having, keys)
}

func checkGroupedValue(v BoundValue, keys map[string]bool) error {
	if v.Agg != "" {
		return nil
	}
	if !keys[valueSig(v)] {
		return malformed("field is not grouped or aggregated")
	}
	return nil
}

func checkGroupedExpr(e BoundExpr, keys map[string]bool) error {
	switch t := e.(type) {
	case *BoundAnd:
		for _, item := range t.Items {
			if err := checkGroupedExpr(item, keys); err != nil {
				return err
			}
		}
	case *BoundOr:
		for _, item := range t.Items {
			if err := checkGroupedExpr(item, keys); err != nil {
				return err
			}
		}
	case *BoundNot:
		return checkGroupedExpr(t.Expr, keys)
	case *BoundCmp:
		return checkGroupedValue(t.Left, keys)
	case *BoundIn:
		return checkGroupedValue(t.Left, keys)
	case *BoundIncludes:
		return checkGroupedValue(t.Left, keys)
	}
	return nil
}
