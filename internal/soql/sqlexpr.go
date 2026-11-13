package soql

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

// compileExpr renders a bound boolean tree as a SQL predicate.
// spec: soql-03
func (c *compiler) compileExpr(e BoundExpr, root string, obj metadata.Object) (string, error) {
	switch t := e.(type) {
	case *BoundAnd:
		parts := make([]string, 0, len(t.Items))
		for _, it := range t.Items {
			p, err := c.compileExpr(it, root, obj)
			if err != nil {
				return "", err
			}
			parts = append(parts, "("+p+")")
		}
		return strings.Join(parts, " AND "), nil
	case *BoundOr:
		parts := make([]string, 0, len(t.Items))
		for _, it := range t.Items {
			p, err := c.compileExpr(it, root, obj)
			if err != nil {
				return "", err
			}
			parts = append(parts, "("+p+")")
		}
		return strings.Join(parts, " OR "), nil
	case *BoundNot:
		// SOQL NOT on a nullable comparison includes null rows (2-valued logic).
		if cmp, ok := t.Expr.(*BoundCmp); ok && cmp.Right.Value != nil && cmp.Left.Field.Nillable && cmp.Left.Agg == "" && cmp.Left.Func == "" {
			neg := *cmp
			switch neg.Op {
			case "=":
				neg.Op = "!="
				return c.compileCmp(&neg, root)
			case "!=", "<>":
				neg.Op = "="
				return c.compileCmp(&neg, root)
			case "<":
				neg.Op = ">="
			case ">":
				neg.Op = "<="
			case "<=":
				neg.Op = ">"
			case ">=":
				neg.Op = "<"
			default:
				// LIKE and other ops: wrap with OR IS NULL for null-inclusive NOT.
				p, err := c.compileCmp(cmp, root)
				if err != nil {
					return "", err
				}
				left, err := c.valueSQL(cmp.Left, root)
				if err != nil {
					return "", err
				}
				return "(" + left + " IS NULL OR NOT (" + p + "))", nil
			}
			result, err := c.compileCmp(&neg, root)
			if err != nil {
				return "", err
			}
			left, err := c.valueSQL(cmp.Left, root)
			if err != nil {
				return "", err
			}
			return "(" + left + " IS NULL OR " + result + ")", nil
		}
		if inn, ok := t.Expr.(*BoundIn); ok && inn.Left.Field.Nillable && inn.Left.Agg == "" && inn.Left.Func == "" {
			inn2 := *inn
			inn2.Not = !inn.Not
			return c.compileIn(&inn2, root)
		}
		p, err := c.compileExpr(t.Expr, root, obj)
		if err != nil {
			return "", err
		}
		return "NOT (" + p + ")", nil
	case *BoundCmp:
		return c.compileCmp(t, root)
	case *BoundIn:
		return c.compileIn(t, root)
	case *BoundIncludes:
		return c.compileIncludes(t, root)
	}
	return "", malformed("expected filter expression")
}

func (c *compiler) valueSQL(v BoundValue, root string) (string, error) {
	owner := root
	if len(v.Path) > 0 {
		alias, err := c.ensureJoin(root, v.Path)
		if err != nil {
			return "", err
		}
		owner = alias
	}
	if v.Agg != "" {
		return aggExpr(owner, v), nil
	}
	if v.Func != "" {
		return c.funcExpr(owner, v), nil
	}
	return owner + "." + pgx.Identifier{v.Field.Column}.Sanitize(), nil
}

// caseInsensitiveKind reports kinds compared with case-insensitive equality.
func caseInsensitiveKind(k fieldtype.Kind) bool {
	switch k {
	case fieldtype.Text, fieldtype.TextArea, fieldtype.Email, fieldtype.Phone,
		fieldtype.Url, fieldtype.PersonName, fieldtype.AutoNumber, fieldtype.Picklist:
		return true
	}
	return false
}

func (c *compiler) compileCmp(cmp *BoundCmp, root string) (string, error) {
	left, err := c.valueSQL(cmp.Left, root)
	if err != nil {
		return "", err
	}
	if cmp.Right.Value == nil {
		if cmp.Op == "=" {
			return left + " IS NULL", nil
		}
		return left + " IS NOT NULL", nil
	}
	param := c.addArg(cmp.Right.Value)
	p := "$" + itoa(param)
	if cmp.Op == "LIKE" {
		return left + " ILIKE " + p + ` ESCAPE '\'`, nil
	}
	k := cmp.Left.Field.Kind()
	isNeq := cmp.Op == "!=" || cmp.Op == "<>"
	nullWrap := isNeq && cmp.Left.Field.Nillable && cmp.Left.Agg == "" && cmp.Left.Func == ""
	var pred string
	if cmp.Left.Label {
		if cmp.Op != "=" && !isNeq {
			return "", malformed("toLabel is only valid with equality operators")
		}
		if cmp.Op == "=" {
			pred = "lower(" + left + " COLLATE pg_c_utf8) = lower(" + p + " COLLATE pg_c_utf8)"
		} else {
			pred = "lower(" + left + " COLLATE pg_c_utf8) != lower(" + p + " COLLATE pg_c_utf8)"
		}
	} else if (cmp.Op == "=" || isNeq) && caseInsensitiveKind(k) {
		if cmp.Op == "=" {
			pred = "lower(" + left + " COLLATE pg_c_utf8) = lower(" + p + " COLLATE pg_c_utf8)"
		} else {
			pred = "lower(" + left + " COLLATE pg_c_utf8) != lower(" + p + " COLLATE pg_c_utf8)"
		}
	} else {
		op := cmp.Op
		if isNeq {
			op = "!="
		}
		if cmp.Right.Cast == "numeric" || cmp.Right.Cast == "bool" || cmp.Right.Cast == "date" || cmp.Right.Cast == "timestamptz" {
			pred = left + " " + op + " " + p + "::" + cmp.Right.Cast
		} else {
			pred = left + " " + op + " " + p
		}
	}
	if nullWrap {
		return "(" + left + " IS NULL OR " + pred + ")", nil
	}
	return pred, nil
}

func (c *compiler) compileIn(in *BoundIn, root string) (string, error) {
	left, err := c.valueSQL(in.Left, root)
	if err != nil {
		return "", err
	}
	if in.Semi != nil {
		return c.compileSemi(in, left, root)
	}
	params := make([]string, 0, len(in.Values))
	for _, v := range in.Values {
		if v.Value == nil {
			return "", malformed("null is not allowed in IN lists")
		}
		params = append(params, "$"+itoa(c.addArg(v.Value)))
	}
	list := "(" + strings.Join(params, ",") + ")"
	if !in.Not {
		return left + " IN " + list, nil
	}
	notIn := left + " NOT IN " + list
	if in.Left.Agg == "" && in.Left.Func == "" && in.Left.Field.Nillable {
		return "(" + left + " IS NULL OR " + notIn + ")", nil
	}
	return notIn, nil
}

func (c *compiler) compileSemi(in *BoundIn, left, root string) (string, error) {
	sub := in.Semi
	// Sub-compiler continues the outer arg numbering so placeholder
	// positions in the sub-SQL match the combined args slice.
	sc := &compiler{joinMap: map[string]string{}, args: c.args}
	subRoot := "s"
	tenantArg := sc.addArg(sub.Object.Tenant)
	col := subRoot + "." + pgx.Identifier{sub.Columns[0].Field.Column}.Sanitize()
	from := sub.Object.TableSQL() + " " + subRoot
	where := subRoot + ".tenant_id=$" + itoa(tenantArg)
	if sub.Where != nil {
		w, err := sc.compileExpr(sub.Where, subRoot, sub.Object)
		if err != nil {
			return "", err
		}
		where += " AND (" + w + ")"
	}
	// Carry sub-compiler joins (from parent-path filters) into the subquery FROM.
	for _, js := range sc.joins {
		from += " LEFT JOIN " + js.table + " " + js.alias + " ON " + js.on
	}
	c.args = sc.args
	if in.Not {
		// NOT EXISTS is NULL-safe: NOT IN is poisoned by NULLs in the sub-select.
		subSQL := "SELECT 1 FROM " + from + " WHERE " + where + " AND " + col + " = " + left
		return "NOT EXISTS (" + subSQL + ")", nil
	}
	subSQL := "SELECT " + col + " FROM " + from + " WHERE " + where
	return left + " IN (" + subSQL + ")", nil
}

func (c *compiler) compileIncludes(inc *BoundIncludes, root string) (string, error) {
	left, err := c.valueSQL(inc.Left, root)
	if err != nil {
		return "", err
	}
	arr := "string_to_array(lower(" + left + " COLLATE pg_c_utf8), ';')"
	groups := make([]string, 0, len(inc.Values))
	for _, v := range inc.Values {
		s, _ := v.Value.(string)
		elems := strings.Split(s, ";")
		folded := make([]string, 0, len(elems))
		for _, e := range elems {
			folded = append(folded, strings.ToLower(strings.TrimSpace(e)))
		}
		param := c.addArg(folded)
		groups = append(groups, arr+" @> $"+itoa(param)+"::text[]")
	}
	body := "(" + strings.Join(groups, " OR ") + ")"
	if inc.Exclude {
		if inc.Left.Field.Nillable {
			return "(" + left + " IS NULL OR NOT " + body + ")", nil
		}
		return "NOT " + body, nil
	}
	return body, nil
}

func itoa(n int) string { return strconv.Itoa(n) }
