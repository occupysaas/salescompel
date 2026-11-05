package platform

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/migration"
)

func bootstrapLegacyTest(ctx context.Context, url string) (*Store, error) {
	if url == "" {
		return nil, errors.New("DATABASE_URL required")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	ok := false
	defer func() {
		if !ok {
			pool.Close()
		}
	}()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, errors.New("database unavailable")
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(739102)"); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return nil, err
	}
	if err = ensureCatalog(ctx, tx); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	ok = true
	return &Store{pool: pool, passwordWork: make(chan struct{}, 4)}, nil
}

func Open(ctx context.Context, url string) (*Store, error) {
	old, e := bootstrapLegacyTest(ctx, url)
	if e != nil {
		return nil, e
	}
	old.Close()
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, e
	}
	defer p.Close()
	if e = migration.Up(ctx, p); e != nil {
		return nil, e
	}
	return OpenOperator(ctx, url)
}
