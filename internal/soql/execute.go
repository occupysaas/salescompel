package soql

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

// Result is one page of query results in REST JSON form.
type Result struct {
	TotalSize   int              `json:"totalSize"`
	Done        bool             `json:"done"`
	Records     []map[string]any `json:"records"`
	NextLocator string           `json:"-"`
}

// Options controls query execution.
type Options struct {
	BatchSize int
	Now       Clock
	// User owns any locator the first page stores.
	User string
}

// Run parses, binds, compiles and executes a SOQL query.
// spec: soql-01
func Run(ctx context.Context, tx pgx.Tx, exec policy.ExecutionContext, snap metadata.Snapshot, text string, opt Options) (Result, error) {
	q, err := Parse(text)
	if err != nil {
		return Result{}, err
	}
	return RunAST(ctx, tx, exec, snap, q, opt)
}

// RunAST binds, compiles and executes a parsed SOQL query.
// spec: soql-01
func RunAST(ctx context.Context, tx pgx.Tx, exec policy.ExecutionContext, snap metadata.Snapshot, q *Query, opt Options) (Result, error) {
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	if opt.User == "" {
		opt.User = exec.Principal.User
	}
	bound, err := Bind(q, snap, exec.Principal, now)
	if err != nil {
		return Result{}, err
	}
	return RunBound(ctx, tx, bound, opt)
}

// RunBound compiles and executes an already-bound query.
func RunBound(ctx context.Context, tx pgx.Tx, bound *Bound, opt Options) (Result, error) {
	plan, err := Compile(bound)
	if err != nil {
		return Result{}, err
	}
	return executePlan(ctx, tx, bound, plan, opt)
}

func executePlan(ctx context.Context, tx pgx.Tx, bound *Bound, plan *Plan, opt Options) (Result, error) {
	batch := clampBatch(opt.BatchSize)
	if bound.CountOnly {
		rows, err := tx.Query(ctx, plan.Stmt.SQL, plan.Stmt.Args...)
		if err != nil {
			return Result{}, dbErr(err)
		}
		return countOnlyResult(rows)
	}
	if bound.Aggregate {
		return executeAggregate(ctx, tx, bound, plan)
	}
	if bound.Offset != nil {
		// OFFSET queries are not paged: one capped page, no locator.
		limit := batch
		if bound.Limit != nil && *bound.Limit < limit {
			limit = *bound.Limit
		}
		plan.Stmt.Args[plan.LimitArg] = limit
		records, err := runRows(ctx, tx, bound, plan)
		if err != nil {
			return Result{}, err
		}
		return Result{TotalSize: len(records), Done: true, Records: records}, nil
	}
	return executePaged(ctx, tx, bound, plan, batch, opt.User)
}

// clampBatch applies the documented 200–2,000 batch bounds; zero means the
// 2,000 default.
func clampBatch(batch int) int {
	if batch <= 0 {
		return 2000
	}
	if batch < 200 {
		return 200
	}
	if batch > 2000 {
		return 2000
	}
	return batch
}

// collectRows shapes one result set. It returns the hidden root IDs needed to
// group child subquery rows back to their parents.
func collectRows(ctx context.Context, tx pgx.Tx, bound *Bound, plan *Plan) ([]map[string]any, []string, error) {
	rows, err := tx.Query(ctx, plan.Stmt.SQL, plan.Stmt.Args...)
	if err != nil {
		return nil, nil, dbErr(err)
	}
	defer rows.Close()
	records := make([]map[string]any, 0, 16)
	var parentIDs []string
	for rows.Next() {
		vals, e := rows.Values()
		if e != nil {
			return nil, nil, dbErr(e)
		}
		records = append(records, shapeRow(bound, plan, vals))
		if plan.RootID != nil {
			id, ok := vals[*plan.RootID].(string)
			if !ok {
				return nil, nil, malformed("parent id is unavailable for child query")
			}
			parentIDs = append(parentIDs, id)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, nil, dbErr(err)
	}
	return records, parentIDs, nil
}

// runRows collects a page and fills its child relationships.
func runRows(ctx context.Context, tx pgx.Tx, bound *Bound, plan *Plan) ([]map[string]any, error) {
	records, parentIDs, err := collectRows(ctx, tx, bound, plan)
	if err != nil {
		return nil, err
	}
	if err = executeChildren(ctx, tx, bound.Children, records, parentIDs); err != nil {
		return nil, err
	}
	return records, nil
}

// countOnlyResult consumes the single COUNT() row and returns the count on
// the response envelope with no records. spec: soql-02
func countOnlyResult(rows pgx.Rows) (Result, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Result{}, dbErr(err)
		}
		// LIMIT 0 on a COUNT() query yields no row; the count is zero.
		return Result{TotalSize: 0, Done: true, Records: []map[string]any{}}, nil
	}
	vals, err := rows.Values()
	if err != nil {
		return Result{}, dbErr(err)
	}
	n, ok := vals[0].(int64)
	if !ok {
		return Result{}, malformed("count result is unavailable")
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Result{}, dbErr(err)
	}
	return Result{TotalSize: int(n), Done: true, Records: []map[string]any{}}, nil
}

// executeChildren runs one statement per relationship for the whole parent
// page, never one statement per parent, and groups the rows back by parent ID.
// spec: child-01
func executeChildren(ctx context.Context, tx pgx.Tx, children []BoundChild, records []map[string]any, parentIDs []string) error {
	if len(children) == 0 {
		return nil
	}
	groups := make([]map[string][]map[string]any, len(children))
	for i, child := range children {
		groups[i] = map[string][]map[string]any{}
		if len(parentIDs) == 0 {
			continue
		}
		compiled, err := compileChildPlan(child, parentIDs)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, compiled.stmt.SQL, compiled.stmt.Args...)
		if err != nil {
			return dbErr(err)
		}
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				rows.Close()
				return dbErr(err)
			}
			parent, ok := vals[0].(string)
			if !ok {
				rows.Close()
				return malformed("child parent id is unavailable")
			}
			// Strip the hidden parent id and row number before normal shaping.
			record := shapeRow(child.Query, compiled.plan, vals[1:len(vals)-1])
			groups[i][parent] = append(groups[i][parent], record)
			if len(groups[i][parent]) > maxChildRows {
				rows.Close()
				return tooComplicated("child relationship exceeds candidate limit; add a LIMIT to the subquery")
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return dbErr(err)
		}
		rows.Close()
	}
	for i, child := range children {
		for j, record := range records {
			rows := groups[i][parentIDs[j]]
			if len(rows) == 0 {
				record[child.Key] = nil
				continue
			}
			record[child.Key] = map[string]any{
				"totalSize": len(rows), "done": true, "records": rows,
			}
		}
	}
	return nil
}

// shapeRow builds one REST JSON record from a result row.
func shapeRow(bound *Bound, plan *Plan, vals []any) map[string]any {
	attrs := map[string]any{"type": bound.Object.Name}
	if bound.Aggregate {
		attrs["type"] = "AggregateResult"
	}
	rec := map[string]any{"attributes": attrs}
	type nestOp struct {
		col BoundColumn
		raw any
	}
	// Parent-path values nest into relationship objects. They never occupy the
	// flat result key, so a root leaf and a parent leaf with the same name
	// (Name and Account.Name) cannot overwrite or delete each other.
	var pathCols []nestOp
	colIdx := 0
	for i, col := range bound.Columns {
		var value any
		if comps, ok := plan.Compound[i]; ok {
			raw := vals[colIdx : colIdx+len(comps)]
			colIdx += len(comps)
			value = AddressValue(col.Field, comps, raw)
		} else {
			value = wireColumnValue(col, vals[colIdx])
			colIdx++
		}
		if len(col.Path) > 0 {
			pathCols = append(pathCols, nestOp{col: col, raw: value})
			continue
		}
		rec[col.Key] = value
	}

	// Detect null parent levels from hidden join id columns.
	joinNull := map[string]bool{}
	for alias, idx := range plan.JoinIDs {
		joinNull[alias] = vals[idx] == nil
	}

	// Nest parent-path columns into their relationship objects.
	for _, op := range pathCols {
		nestPath(rec, op.col, op.raw, joinNull, plan)
	}

	if !bound.Aggregate {
		if id, ok := rec["Id"].(string); ok {
			rec["attributes"].(map[string]any)["url"] = "/services/data/v65.0/sobjects/" + bound.Object.Name + "/" + id
		}
	}
	return rec
}

// nestPath places a parent-path field into the nested REST JSON structure.
func nestPath(rec map[string]any, col BoundColumn, raw any, joinNull map[string]bool, plan *Plan) {
	cur := rec
	for depth, step := range col.Path {
		rel := "relationship"
		if step.Via.RelationshipName != nil {
			rel = *step.Via.RelationshipName
		}
		alias := plan.PathAlias[pathKeyOf(col.Path[:depth+1])]
		if alias != "" && joinNull[alias] {
			cur[rel] = nil
			return
		}
		if depth == len(col.Path)-1 {
			if existing, ok := cur[rel].(map[string]any); ok {
				existing[col.Key] = raw
			} else {
				cur[rel] = map[string]any{
					col.Key:      raw,
					"attributes": map[string]any{"type": step.Object.Name},
				}
			}
			return
		}
		next, ok := cur[rel].(map[string]any)
		if !ok {
			next = map[string]any{"attributes": map[string]any{"type": step.Object.Name}}
			cur[rel] = next
		}
		cur = next
	}
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	return contract.Problem(500, "SERVER_UNAVAILABLE", "Storage operation failed")
}
