package query

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"strings"
)

func Select(ctx context.Context, tx pgx.Tx, execution policy.ExecutionContext, catalog metadata.Snapshot, q Query) ([]map[string]any, error) {
	p := execution.Principal
	p.Catalog = &catalog
	var err error
	q, err = canonicalQuery(p, q)
	if err != nil {
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
		expressions = append(expressions, "r."+pgx.Identifier{f.Column}.Sanitize())
	}
	from := table(p, q.Object) + " r"
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
			sql += " AND r." + pgx.Identifier{f.Column}.Sanitize() + " IS NULL"
		} else {
			if f.Type == "string" {
				sql += " AND lower(r." + f.Column + " COLLATE pg_c_utf8) = lower($2 COLLATE pg_c_utf8)"
			} else {
				sql += " AND r." + pgx.Identifier{f.Column}.Sanitize() + "=$2"
			}
			args = append(args, *q.WhereValue)
		}
	}
	if q.Order != "" {
		f, err := field(p, q.Object, q.Order)
		if err != nil {
			return nil, err
		}
		sql += " ORDER BY r." + pgx.Identifier{f.Column}.Sanitize()
		if q.Desc {
			sql += " DESC"
		}
		if q.Nulls == NullLast {
			sql += " NULLS LAST,r.id"
		} else {
			sql += " NULLS FIRST,r.id"
		}
	} else {
		sql += " ORDER BY r.id"
	}
	limit := q.Limit
	if !q.ExplicitLimit {
		limit++
	}
	args = append(args, limit)
	sql += fmt.Sprintf(" LIMIT $%d", len(args))
	rows, err := tx.Query(ctx, sql, args...)
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
				f, _ := field(p, q.Object, name)
				record[f.Name] = values[i]
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

func table(p Principal, object string) string { o, _ := p.Catalog.Object(object); return o.TableSQL() }
func dbError(e error) error {
	if e == nil {
		return nil
	}
	return contract.Problem(500, "SERVER_UNAVAILABLE", "Storage operation failed")
}
