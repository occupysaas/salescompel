package platform

import (
	"context"
	"fmt"
	"strings"
)

func (s *Store) Select(ctx context.Context, p Principal, q Query) ([]map[string]any, error) {
	if _, err := Describe(p, q.Object); err != nil {
		return nil, err
	}
	if len(q.Fields) == 0 || q.Limit < 1 || q.Limit > 2000 {
		return nil, invalid("MALFORMED_QUERY", "Invalid selection")
	}
	expressions := []string{}
	join := false
	for _, name := range q.Fields {
		if q.Object == "Contact" && name == "Account.Name" {
			expressions = append(expressions, "a.name")
			join = true
			continue
		}
		f, err := field(p, q.Object, name)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, "r."+f.Column)
	}
	from := table(q.Object) + " r"
	if join {
		from += " LEFT JOIN accounts a ON a.tenant_id=r.tenant_id AND a.id=r.account_id"
	}
	sql := "SELECT " + strings.Join(expressions, ",") + " FROM " + from + " WHERE r.tenant_id=$1"
	args := []any{p.Tenant}
	if q.HasWhere {
		f, err := field(p, q.Object, q.WhereField)
		if err != nil {
			return nil, err
		}
		if q.WhereValue == nil {
			sql += " AND r." + f.Column + " IS NULL"
		} else {
			sql += " AND r." + f.Column + "=$2"
			args = append(args, *q.WhereValue)
		}
	}
	if q.Order != "" {
		f, err := field(p, q.Object, q.Order)
		if err != nil {
			return nil, err
		}
		sql += " ORDER BY r." + f.Column
		if q.Desc {
			sql += " DESC"
		}
		sql += " NULLS LAST,r.id"
	} else {
		sql += " ORDER BY r.id"
	}
	limit := q.Limit
	if !q.ExplicitLimit {
		limit++
	}
	args = append(args, limit)
	sql += fmt.Sprintf(" LIMIT $%d", len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		values, e := rows.Values()
		if e != nil {
			return nil, dbError(e)
		}
		record := map[string]any{"attributes": map[string]any{"type": q.Object}}
		for i, name := range q.Fields {
			if name == "Account.Name" {
				if values[i] == nil {
					record["Account"] = nil
				} else {
					record["Account"] = map[string]any{"Name": values[i], "attributes": map[string]any{"type": "Account"}}
				}
			} else {
				record[name] = values[i]
			}
		}
		if id, ok := record["Id"].(string); ok {
			record["attributes"].(map[string]any)["url"] = "/services/data/v65.0/sobjects/" + q.Object + "/" + id
		}
		out = append(out, record)
	}
	if err = rows.Err(); err != nil {
		return nil, dbError(err)
	}
	if len(out) > q.Limit {
		return nil, invalid("QUERY_TOO_COMPLICATED", "Result exceeds candidate limit; add LIMIT or a narrower filter")
	}
	return out, nil
}
