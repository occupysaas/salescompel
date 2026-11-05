package records

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/policy"
	"time"
)

func WithTenantTx(ctx context.Context, pool *pgxpool.Pool, execution policy.ExecutionContext, fn func(pgx.Tx) error) error {
	if e := execution.Validate(); e != nil {
		return e
	}
	tx, e := pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tx.Rollback(cleanup)
	}()
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(739102)"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", execution.Principal.Tenant); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
