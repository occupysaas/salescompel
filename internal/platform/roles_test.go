package platform

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"salescompel/internal/migration"
	"testing"
)

func TestRuntimeRoleRejectsOwner(t *testing.T) {
	_, e := OpenRuntime(context.Background(), StoreConfig{RecordsURL: os.Getenv("TEST_DATABASE_URL"), IdentityURL: os.Getenv("TEST_DATABASE_URL")})
	if e == nil {
		t.Fatal("accepted owner credentials")
	}
}
func TestRoleSeparation(t *testing.T) {
	ctx := context.Background()
	url := os.Getenv("TEST_DATABASE_URL")
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if e = migration.Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	for i, dsn := range limitedURLs(t) {
		q, e := pgxpool.New(ctx, dsn)
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			var n int
			if e = q.QueryRow(ctx, "SELECT count(*) FROM public.accounts").Scan(&n); e != nil || n != 0 {
				t.Fatal("missing-context must see no records", n, e)
			}
			if _, e = q.Exec(ctx, "SELECT password_hash FROM public.users"); e == nil {
				t.Fatal("records can read secrets")
			}
		} else {
			if _, e = q.Exec(ctx, "SELECT * FROM public.accounts"); e == nil {
				t.Fatal("identity can read records")
			}
		}
		q.Close()

	}
}
func limitedURLs(t *testing.T) []string {
	t.Helper()
	r, a := os.Getenv("TEST_RECORDS_URL"), os.Getenv("TEST_IDENTITY_URL")
	if r != "" && a != "" {
		return []string{r, a}
	}
	u, e := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	u.User = url.User("sc_records_login")
	r = u.String()
	u.User = url.User("sc_identity_login")
	return []string{r, u.String()}
}
func TestRuntimeLimitedConnections(t *testing.T) {
	u := limitedURLs(t)
	s, e := OpenRuntime(context.Background(), StoreConfig{u[0], u[1]})
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
}
func TestRLSWithCheck(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	u := limitedURLs(t)
	p, e := pgxpool.New(context.Background(), u[0])
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", a.Tenant); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "INSERT INTO public.accounts(tenant_id,id,name)VALUES($1,'001000000000001AAA','Forbidden')", b.Tenant); e == nil {
		t.Fatal("wrong-tenant INSERT escaped WITH CHECK")
	}
}
func TestRLSOmittedTenantPredicate(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()
	for _, p := range []Principal{a, b} {
		if _, e := s.Create(ctx, p, "Account", map[string]any{"Name": "RLS row"}); e != nil {
			t.Fatal(e)
		}
	}
	u := limitedURLs(t)
	p, e := pgxpool.New(ctx, u[0])
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", a.Tenant)
	var n int
	var tenant string
	if e = tx.QueryRow(ctx, "SELECT count(*),min(tenant_id) FROM public.accounts").Scan(&n, &tenant); e != nil || n != 1 || tenant != a.Tenant {
		t.Fatal("omitted predicate leaked records", n, tenant, e)
	}
	var metadataCount int
	var metadataTenant string
	if e = tx.QueryRow(ctx, "SELECT count(*),min(tenant_id) FROM sc_catalog.objects").Scan(&metadataCount, &metadataTenant); e != nil || metadataCount != 2 || metadataTenant != a.Tenant {
		t.Fatal("metadata RLS leaked definitions", metadataCount, metadataTenant, e)
	}
	if _, e = tx.Exec(ctx, "UPDATE public.accounts SET tenant_id=$1", b.Tenant); e == nil {
		t.Fatal("tenant-changing UPDATE escaped WITH CHECK")
	}
}
func TestRuntimeRejectsUnexpectedIdentityGrant(t *testing.T) {
	s := testStore(t)
	u := limitedURLs(t)
	cfg, e := pgxpool.ParseConfig(u[0])
	if e != nil {
		t.Fatal(e)
	}
	role := pgx.Identifier{cfg.ConnConfig.User}.Sanitize()
	ctx := context.Background()
	if _, e = s.pool.Exec(ctx, "GRANT SELECT ON public.oauth_access_tokens TO "+role); e != nil {
		t.Fatal(e)
	}
	defer s.pool.Exec(ctx, "REVOKE SELECT ON public.oauth_access_tokens FROM "+role)
	runtime, e := OpenRuntime(ctx, StoreConfig{u[0], u[1]})
	if e == nil {
		runtime.Close()
		t.Fatal("accepted an extra credential-table grant")
	}
}
func TestRuntimeRejectsStartupRoleMask(t *testing.T) {
	u := limitedURLs(t)
	owner, e := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	q := owner.Query()
	q.Set("role", "salescompel_records")
	owner.RawQuery = q.Encode()
	s, e := OpenRuntime(context.Background(), StoreConfig{owner.String(), u[1]})
	if e == nil {
		s.Close()
		t.Fatal("startup role hid privileged session_user")
	}
}
func TestRuntimeRejectsColumnSecretGrant(t *testing.T) {
	s := testStore(t)
	u := limitedURLs(t)
	cfg, e := pgxpool.ParseConfig(u[0])
	if e != nil {
		t.Fatal(e)
	}
	role := pgx.Identifier{cfg.ConnConfig.User}.Sanitize()
	ctx := context.Background()
	if _, e = s.pool.Exec(ctx, "GRANT SELECT(password_hash) ON public.users TO "+role); e != nil {
		t.Fatal(e)
	}
	defer s.pool.Exec(ctx, "REVOKE SELECT(password_hash) ON public.users FROM "+role)
	runtime, e := OpenRuntime(ctx, StoreConfig{u[0], u[1]})
	if e == nil {
		runtime.Close()
		t.Fatal("column credential grant escaped role checks")
	}
}
