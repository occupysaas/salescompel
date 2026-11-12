package platform

import (
	"context"

	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"

	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/metadata"
	"salescompel/internal/migration"
)

//go:embed schema.sql
var schema string

type Store struct {
	pool         *pgxpool.Pool
	auth         *pgxpool.Pool
	tx           pgx.Tx
	passwordWork chan struct{}
}

func OpenOperator(ctx context.Context, url string) (*Store, error) {
	if url == "" {
		return nil, errors.New("MIGRATION_DATABASE_URL required")
	}
	pool, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, errors.New("invalid database configuration")
	}
	var owner bool
	if e = pool.QueryRow(ctx, "SELECT pg_has_role(current_user,'salescompel_owner','MEMBER')").Scan(&owner); e != nil || !owner {
		pool.Close()
		return nil, errors.New("operator connection requires salescompel_owner membership")
	}
	if e = migration.CheckHead(ctx, pool); e != nil {
		pool.Close()
		return nil, e
	}
	return &Store{pool: pool, passwordWork: make(chan struct{}, 4)}, nil
}
func (s *Store) Close() {
	s.pool.Close()
	if s.auth != nil && s.auth != s.pool {
		s.auth.Close()
	}
}
func digest(token string) string { v := sha256.Sum256([]byte(token)); return hex.EncodeToString(v[:]) }
func (s *Store) Provision(ctx context.Context, token string, p Principal) error {
	if len(token) < 16 || p.Tenant == "" || p.User == "" || (p.Role != "admin" && p.Role != "restricted") {
		return errors.New("token must be at least 16 characters; tenant, user and valid role required")
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "INSERT INTO tenants(id) VALUES($1) ON CONFLICT DO NOTHING", p.Tenant); err != nil {
		return err
	}
	if err = metadata.SeedTenant(ctx, tx, p.Tenant); err != nil {
		return err
	}
	// A token cannot silently be reassigned to a different principal.
	_, err = tx.Exec(ctx, "INSERT INTO principals(token_hash,tenant_id,user_id,role) VALUES($1,$2,$3,$4) ON CONFLICT(token_hash) DO NOTHING", digest(token), p.Tenant, p.User, p.Role)
	if err != nil {
		return err
	}
	var actual Principal
	err = tx.QueryRow(ctx, "SELECT tenant_id,user_id,role FROM principals WHERE token_hash=$1", digest(token)).Scan(&actual.Tenant, &actual.User, &actual.Role)
	if err != nil {
		return err
	}
	if actual.Tenant != p.Tenant || actual.User != p.User || actual.Role != p.Role {
		return errors.New("token already assigned")
	}
	return tx.Commit(ctx)
}
func (s *Store) Authenticate(ctx context.Context, token string) (Principal, error) {
	return s.AuthenticateAt(ctx, token, time.Now())
}

// SetCurrency changes a tenant's corporate currency (operator store only).
func (s *Store) SetCurrency(ctx context.Context, tenant, code string) error {
	return metadata.SetCurrency(ctx, s.pool, tenant, code)
}
