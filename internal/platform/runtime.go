package platform

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/metadata"
	"salescompel/internal/migration"
	"salescompel/internal/policy"
	"salescompel/internal/records"
)

type StoreConfig struct{ RecordsURL, IdentityURL string }

func checkedPool(ctx context.Context, url, group string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, errors.New("separate DATABASE_URL and AUTH_DATABASE_URL required")
	}
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, errors.New("invalid runtime database configuration")
	}
	if e = migration.CheckHead(ctx, p); e != nil {
		p.Close()
		return nil, e
	}
	fail := func() (*pgxpool.Pool, error) { p.Close(); return nil, errors.New("unsafe runtime database privileges") }
	var safe bool
	e = p.QueryRow(ctx, `SELECT current_user=session_user AND session_user=$2 AND pg_has_role(current_user,$1,'MEMBER') AND NOT EXISTS(SELECT FROM pg_roles r WHERE pg_has_role(current_user,r.oid,'MEMBER') AND (r.rolsuper OR r.rolbypassrls OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolname='salescompel_owner')) AND NOT EXISTS(SELECT FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname IN ('public','sc_catalog','sc_records','sc_control') AND pg_has_role(current_user,c.relowner,'MEMBER')) AND NOT has_schema_privilege(current_user,'public','CREATE') AND NOT has_schema_privilege(current_user,'sc_catalog','CREATE') AND NOT has_schema_privilege(current_user,'sc_records','CREATE')`, group, p.Config().ConnConfig.User).Scan(&safe)
	if e != nil || !safe {
		return fail()
	}
	other := "salescompel_identity"
	if group == "salescompel_identity" {
		other = "salescompel_records"
	}
	var cross bool
	if e = p.QueryRow(ctx, "SELECT pg_has_role(current_user,$1,'MEMBER')", other).Scan(&cross); e != nil || cross {
		return fail()
	}
	if group == "salescompel_records" {
		if e = p.QueryRow(ctx, "SELECT has_table_privilege(current_user,'public.users','SELECT') OR has_table_privilege(current_user,'public.accounts','TRUNCATE') OR has_table_privilege(current_user,'sc_catalog.objects','INSERT,UPDATE,DELETE')").Scan(&cross); e != nil || cross {
			return fail()
		}
	} else {
		if e = p.QueryRow(ctx, "SELECT has_table_privilege(current_user,'public.accounts','SELECT,INSERT,UPDATE,DELETE') OR EXISTS(SELECT FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='sc_catalog' AND c.relname='objects' AND has_table_privilege(current_user,c.oid,'SELECT'))").Scan(&cross); e != nil || cross {
			return fail()
		}
	}
	if e = migration.CheckHead(ctx, p); e != nil {
		p.Close()
		return nil, e
	}
	if safe, e = runtimeTablesSafe(ctx, p, group); e != nil || !safe {
		return fail()
	}
	return p, nil
}
func OpenRuntime(ctx context.Context, c StoreConfig) (*Store, error) {
	r, e := checkedPool(ctx, c.RecordsURL, "salescompel_records")
	if e != nil {
		return nil, e
	}
	a, e := checkedPool(ctx, c.IdentityURL, "salescompel_identity")
	if e != nil {
		r.Close()
		return nil, e
	}
	var ru, au string
	r.QueryRow(ctx, "SELECT session_user").Scan(&ru)
	a.QueryRow(ctx, "SELECT session_user").Scan(&au)
	if ru == au {
		r.Close()
		a.Close()
		return nil, errors.New("record and identity logins must differ")
	}
	return &Store{pool: r, auth: a, passwordWork: make(chan struct{}, 4)}, nil
}
func (s *Store) authPool() *pgxpool.Pool {
	if s.auth != nil {
		return s.auth
	}
	return s.pool
}
func (s *Store) begin(ctx context.Context) (pgx.Tx, error) {
	if s.tx != nil {
		return s.tx.Begin(ctx)
	}
	return s.pool.Begin(ctx)
}
func (s *Store) within(ctx context.Context, p Principal, fn func(*Store, Principal) error) error {
	if s.tx != nil {
		return fn(s, p)
	}
	execution, e := policy.ForAPI(p, "65.0")
	if e != nil {
		return e
	}
	return records.WithTenantTx(ctx, s.pool, execution, func(tx pgx.Tx) error {
		catalog, e := metadata.Load(ctx, tx, p.Tenant)
		if e != nil {
			return e
		}
		p.Catalog = &catalog
		clone := *s
		clone.tx = tx
		return fn(&clone, p)
	})
}
func (s *Store) Describe(ctx context.Context, p Principal, object string) (Object, error) {
	var out Object
	e := s.within(ctx, p, func(_ *Store, p Principal) error { var e error; out, e = records.Describe(p, object); return e })
	return out, e
}
func (s *Store) Catalog(ctx context.Context, p Principal) (metadata.Snapshot, error) {
	var out metadata.Snapshot
	e := s.within(ctx, p, func(_ *Store, p Principal) error { out = *p.Catalog; return nil })
	return out, e
}

func (s *Store) recordTx(p Principal) *records.Tx {
	e, _ := policy.ForAPI(p, "65.0")
	return records.Bind(s.tx, e, *p.Catalog)
}
