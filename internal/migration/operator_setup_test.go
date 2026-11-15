package migration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/migration/legacy"
)

// TestFreshOperatorSetup exercises the documented non-superuser migration login.
func TestFreshOperatorSetup(t *testing.T) {
	admin := testPool(t)
	ctx := context.Background()
	operator := fmt.Sprintf("migration_operator_%d", time.Now().UnixNano())
	role := pgx.Identifier{operator}.Sanitize()
	password := operator // Disposable test credential for a disposable database.
	if _, err := admin.Exec(ctx, "CREATE ROLE "+role+" LOGIN PASSWORD '"+password+"'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP OWNED BY "+role); err != nil {
			t.Errorf("drop test operator privileges: %v", err)
		}
		if _, err := admin.Exec(ctx, "DROP ROLE "+role); err != nil {
			t.Errorf("drop test operator: %v", err)
		}
	})
	var database string
	if err := admin.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"GRANT salescompel_owner TO " + role,
		"GRANT CREATE ON DATABASE " + pgx.Identifier{database}.Sanitize() + " TO salescompel_owner",
		"REVOKE CREATE ON SCHEMA public FROM PUBLIC",
		"GRANT USAGE, CREATE ON SCHEMA public TO salescompel_owner",
		"GRANT USAGE ON SCHEMA public TO salescompel_records, salescompel_identity",
	} {
		if _, err := admin.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = operator
	cfg.ConnConfig.Password = password
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=salescompel_owner"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var sessionUser, currentUser string
	if err := pool.QueryRow(ctx, "SELECT session_user, current_user").Scan(&sessionUser, &currentUser); err != nil {
		t.Fatal(err)
	}
	if sessionUser != operator || currentUser != "salescompel_owner" {
		t.Fatalf("unexpected roles: session=%s current=%s", sessionUser, currentUser)
	}
	if err := Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := CheckHead(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, pool); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	var unsafe bool
	err = admin.QueryRow(ctx, `SELECT
		has_schema_privilege('salescompel_records', 'public', 'CREATE') OR
		has_schema_privilege('salescompel_identity', 'public', 'CREATE') OR
		EXISTS(SELECT FROM unnest(ARRAY['salescompel_records','salescompel_identity']) AS role_name,
		 unnest(ARRAY['sc_control','sc_catalog','sc_records','sc_query','sc_setup']) AS schema_name
		 WHERE has_schema_privilege(role_name, schema_name, 'CREATE')) OR
		has_table_privilege('salescompel_records', 'public.users', 'SELECT') OR
		has_any_column_privilege('salescompel_records', 'public.users', 'SELECT') OR
		has_table_privilege('salescompel_identity', 'sc_setup.users', 'SELECT') OR
		has_function_privilege('salescompel_identity', 'sc_setup.create_identity(varchar,text,text)', 'EXECUTE') OR
		has_function_privilege('salescompel_identity', 'sc_setup.rename_identity(varchar,text,text)', 'EXECUTE') OR
		has_function_privilege('salescompel_records', 'sc_setup.id_checksum(text)', 'EXECUTE') OR
		has_table_privilege('salescompel_identity', 'sc_catalog.objects', 'SELECT') OR
		pg_has_role('salescompel_records', 'salescompel_owner', 'MEMBER') OR
		pg_has_role('salescompel_identity', 'salescompel_owner', 'MEMBER')`).Scan(&unsafe)
	if err != nil || unsafe {
		t.Fatalf("unsafe runtime privileges: %t, %v", unsafe, err)
	}
	var usable bool
	err = admin.QueryRow(ctx, `SELECT
		has_table_privilege('salescompel_records', 'sc_catalog.objects', 'SELECT') AND
		has_table_privilege('salescompel_records', 'public.accounts', 'SELECT,INSERT,UPDATE,DELETE') AND
		has_table_privilege('salescompel_identity', 'public.users', 'SELECT,UPDATE') AND
		NOT EXISTS(SELECT FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		 WHERE c.relkind='r' AND (n.nspname IN ('sc_catalog','sc_records') OR
		 (n.nspname='public' AND c.relname IN ('accounts','contacts')))
		 AND (NOT c.relrowsecurity OR NOT c.relforcerowsecurity))`).Scan(&usable)
	if err != nil || !usable {
		t.Fatalf("missing runtime privileges or row security: %t, %v", usable, err)
	}
	var identityColumns string
	if err = admin.QueryRow(ctx, `SELECT string_agg(attname, ',' ORDER BY attnum) FROM pg_attribute
		WHERE attrelid = 'sc_setup.users'::regclass AND attnum > 0 AND NOT attisdropped
		AND has_column_privilege('salescompel_identity', attrelid, attnum, 'SELECT')`).Scan(&identityColumns); err != nil {
		t.Fatal(err)
	}
	if identityColumns != "id,tenant_id,is_active,profile_id" {
		t.Fatalf("identity reads sc_setup.users columns %q", identityColumns)
	}
	var executable string
	if err = admin.QueryRow(ctx, `SELECT coalesce(string_agg(r.rolname || ':' || n.nspname || '.' || p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ')', ',' ORDER BY r.rolname, p.proname), '')
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		CROSS JOIN (SELECT rolname FROM pg_roles WHERE rolname IN ('salescompel_records', 'salescompel_identity')) r
		WHERE n.nspname IN ('public', 'sc_control', 'sc_catalog', 'sc_records', 'sc_query', 'sc_setup')
		AND has_function_privilege(r.rolname, p.oid, 'EXECUTE')`).Scan(&executable); err != nil {
		t.Fatal(err)
	}
	if executable != "salescompel_records:sc_setup.create_identity(p_id character varying, p_tenant text, p_username text),salescompel_records:sc_setup.rename_identity(p_id character varying, p_tenant text, p_username text)" {
		t.Fatalf("runtime EXECUTE grants: %s", executable)
	}
}

func TestLegacyOperatorRecoversAfterMigrationTwo(t *testing.T) {
	admin := testPool(t)
	ctx := context.Background()
	operator := fmt.Sprintf("migration_operator_%d", time.Now().UnixNano())
	role := pgx.Identifier{operator}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE ROLE "+role+" LOGIN PASSWORD '"+operator+"'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP OWNED BY "+role); err != nil {
			t.Errorf("drop test operator privileges: %v", err)
		}
		if _, err := admin.Exec(ctx, "DROP ROLE "+role); err != nil {
			t.Errorf("drop test operator: %v", err)
		}
	})
	var database string
	if err := admin.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"GRANT salescompel_owner TO " + role,
		"GRANT CREATE ON DATABASE " + pgx.Identifier{database}.Sanitize() + " TO " + role,
		"REVOKE CREATE ON SCHEMA public FROM PUBLIC",
		"GRANT USAGE, CREATE ON SCHEMA public TO " + role + ", salescompel_owner",
		"GRANT USAGE ON SCHEMA public TO salescompel_records, salescompel_identity",
	} {
		if _, err := admin.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = operator
	cfg.ConnConfig.Password = operator
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Install(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO public.tenants(id) VALUES('retained')"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	err = Up(ctx, pool)
	var pgErr *pgconn.PgError
	if err == nil || !strings.Contains(err.Error(), "migration 3 failed") || !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("expected migration 3 failure, got %v", err)
	}
	entries, err := Status(ctx, pool)
	if err != nil || len(entries) < 3 || !entries[1].Applied || entries[2].Applied {
		t.Fatalf("unexpected migration status after failure: %v, %v", entries, err)
	}
	if _, err := admin.Exec(ctx, "GRANT CREATE ON SCHEMA sc_control, sc_catalog, sc_records TO salescompel_owner"); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, pool); err != nil {
		t.Fatalf("retry migration: %v", err)
	}
	if err := CheckHead(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var retained bool
	if err := admin.QueryRow(ctx, "SELECT EXISTS(SELECT FROM public.tenants WHERE id='retained')").Scan(&retained); err != nil || !retained {
		t.Fatalf("legacy tenant not retained: %t, %v", retained, err)
	}
}
