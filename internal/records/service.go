package records

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/query"
	"salescompel/internal/soql"
)

type Service struct{ Pool *pgxpool.Pool }
type Tx struct {
	db        pgx.Tx
	principal Principal
	catalog   metadata.Snapshot
	execution policy.ExecutionContext
}

func (s Service) Within(ctx context.Context, execution policy.ExecutionContext, fn func(*Tx) error) error {
	return WithTenantTx(ctx, s.Pool, execution, func(db pgx.Tx) error {
		catalog, e := metadata.Load(ctx, db, execution.Principal.Tenant)
		if e != nil {
			return e
		}
		return fn(Bind(db, execution, catalog))
	})
}
func Bind(db pgx.Tx, e policy.ExecutionContext, c metadata.Snapshot) *Tx {
	p := e.Principal
	p.Catalog = &c
	return &Tx{db: db, principal: p, catalog: c, execution: e}
}
func (t *Tx) Create(ctx context.Context, object string, data map[string]any) (string, error) {
	return create(ctx, t.db, t.principal, object, data)
}
func (t *Tx) Update(ctx context.Context, object, id string, data map[string]any) error {
	return update(ctx, t.db, t.principal, object, id, data)
}
func (t *Tx) Delete(ctx context.Context, object, id string) error {
	return deleteRecord(ctx, t.db, t.principal, object, id)
}
func (t *Tx) Select(ctx context.Context, q query.Query) ([]map[string]any, error) {
	return query.Select(ctx, t.db, t.execution, t.catalog, q)
}

// Query executes SOQL text through the engine.
func (t *Tx) Query(ctx context.Context, text string, opt soql.Options) (soql.Result, error) {
	return soql.Run(ctx, t.db, t.execution, t.catalog, text, opt)
}

// QueryAST executes a parsed SOQL AST through the engine.
func (t *Tx) QueryAST(ctx context.Context, q *soql.Query, opt soql.Options) (soql.Result, error) {
	return soql.RunAST(ctx, t.db, t.execution, t.catalog, q, opt)
}
func DBError(err error) error { return dbError(err) }
func CanonicalFields(p Principal, object string, data map[string]any) (map[string]any, error) {
	return canonicalFields(p, object, data)
}
