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
	bound, err := Bind(q, snap, exec.Principal, now)
	if err != nil {
		return Result{}, err
	}
	plan, err := Compile(bound)
	if err != nil {
		return Result{}, err
	}
	return executePlan(ctx, tx, bound, plan, opt)
}

func executePlan(ctx context.Context, tx pgx.Tx, bound *Bound, plan *Plan, opt Options) (Result, error) {
	batch := opt.BatchSize
	if batch <= 0 {
		batch = 2000
	}
	if batch < 200 {
		batch = 200
	}
	if batch > 2000 {
		batch = 2000
	}

	// Fetch one extra row past the page cap so overflow is detectable.
	// When the user LIMIT already fits in one batch, use it directly.
	fetchLimit := batch + 1
	if bound.Limit != nil && *bound.Limit <= batch {
		fetchLimit = *bound.Limit
	}
	plan.Stmt.Args[plan.LimitArg] = fetchLimit

	rows, err := tx.Query(ctx, plan.Stmt.SQL, plan.Stmt.Args...)
	if err != nil {
		return Result{}, dbErr(err)
	}
	defer rows.Close()

	records := make([]map[string]any, 0, 16)
	for rows.Next() {
		vals, e := rows.Values()
		if e != nil {
			return Result{}, dbErr(e)
		}
		records = append(records, shapeRow(bound, plan, vals))
	}
	if err = rows.Err(); err != nil {
		return Result{}, dbErr(err)
	}

	total := len(records)
	// Done true only when the full result fits in one page.
	if bound.Limit == nil || *bound.Limit > batch {
		if total > batch {
			return Result{}, tooComplicated("result exceeds one page; add LIMIT or a narrower filter")
		}
	}
	if bound.Limit != nil && total > *bound.Limit {
		return Result{}, tooComplicated("result exceeds one page; add LIMIT or a narrower filter")
	}
	return Result{TotalSize: total, Done: true, Records: records}, nil
}

// shapeRow builds one REST JSON record from a result row.
func shapeRow(bound *Bound, plan *Plan, vals []any) map[string]any {
	rec := map[string]any{
		"attributes": map[string]any{"type": bound.Object.Name},
	}
	colIdx := 0
	for i, col := range bound.Columns {
		if comps, ok := plan.Compound[i]; ok {
			raw := vals[colIdx : colIdx+len(comps)]
			colIdx += len(comps)
			rec[col.Key] = AddressValue(col.Field, comps, raw)
			continue
		}
		v := vals[colIdx]
		colIdx++
		rec[col.Key] = WireValue(col.Field, v)
	}

	// Detect null parent levels from hidden join id columns.
	joinNull := map[string]bool{}
	for alias, idx := range plan.JoinIDs {
		joinNull[alias] = vals[idx] == nil
	}

	// Nest parent-path columns into their relationship objects.
	type nestOp struct {
		col BoundColumn
		raw any
	}
	var pathCols []nestOp
	for i, col := range bound.Columns {
		if len(col.Path) == 0 {
			continue
		}
		raw := rec[col.Key]
		delete(rec, col.Key)
		if comps, ok := plan.Compound[i]; ok {
			raw = AddressValue(col.Field, comps, raw.([]any))
		}
		pathCols = append(pathCols, nestOp{col: col, raw: raw})
	}
	for _, op := range pathCols {
		nestPath(rec, op.col, op.raw, joinNull, plan)
	}

	if id, ok := rec["Id"].(string); ok {
		rec["attributes"].(map[string]any)["url"] = "/services/data/v65.0/sobjects/" + bound.Object.Name + "/" + id
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
