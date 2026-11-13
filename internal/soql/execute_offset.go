package soql

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// executeOffset serves an OFFSET query. page-01 forbids a locator for OFFSET,
// so there is at most one page, but totalSize is still the full post-LIMIT/
// OFFSET result size rather than the rows that fit in that page.
// spec: page-01
func executeOffset(ctx context.Context, tx pgx.Tx, bound *Bound, batch int) (Result, error) {
	total, err := offsetTotal(ctx, tx, bound)
	if err != nil {
		return Result{}, err
	}
	plan, err := Compile(bound)
	if err != nil {
		return Result{}, err
	}
	limit := batch
	if bound.Limit != nil && *bound.Limit < limit {
		limit = *bound.Limit
	}
	plan.Stmt.Args[plan.LimitArg] = limit
	records, err := runRows(ctx, tx, bound, plan)
	if err != nil {
		return Result{}, err
	}
	return Result{TotalSize: total, Done: true, Records: records}, nil
}

// offsetTotal counts the query's result after its own LIMIT and OFFSET. The
// compiled ID projection is wrapped in count(*) so the exact membership is
// measured without fetching it.
func offsetTotal(ctx context.Context, tx pgx.Tx, bound *Bound) (int, error) {
	idField, err := idFieldOf(bound.Object)
	if err != nil {
		return 0, err
	}
	m := *bound
	m.Columns = []BoundColumn{{Key: "Id", Field: idField}}
	m.Children = nil
	m.PageIDs = nil
	if m.Limit == nil {
		// The compiler's implicit 2,000-row page default must not cap a count
		// that is meant to measure the whole post-OFFSET membership.
		max := int(^uint(0) >> 1)
		m.Limit = &max
	}
	plan, err := Compile(&m)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM ("+plan.Stmt.SQL+") __offset_total", plan.Stmt.Args...).Scan(&n); err != nil {
		return 0, dbErr(err)
	}
	return int(n), nil
}
