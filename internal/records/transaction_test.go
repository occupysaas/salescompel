package records

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"salescompel/internal/policy"
	"testing"
)

func TestTenantTxCleanup(t *testing.T) {
	ctx := context.Background()
	cfg, e := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	cfg.MaxConns = 1
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	execution, e := policy.ForAPI(policy.Principal{Tenant: "tenant-a", Role: "admin"}, "65.0")
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"commit", "rollback", "panic", "cancel"} {
		func() {
			defer func() { recover() }()
			e = WithTenantTx(ctx, p, execution, func(tx pgx.Tx) error {
				var tenant string
				if e := tx.QueryRow(ctx, "SELECT current_setting('salescompel.tenant_id')").Scan(&tenant); e != nil || tenant != "tenant-a" {
					t.Fatal(tenant, e)
				}
				switch mode {
				case "rollback":
					return errors.New("rollback")
				case "panic":
					panic("test")
				case "cancel":
					cancelled, cancel := context.WithCancel(ctx)
					cancel()
					_, e := tx.Exec(cancelled, "SELECT 1")
					return e
				}
				return nil
			})
			if mode == "commit" && e != nil {
				t.Fatal(e)
			}
		}()
		var tenant *string
		if e = p.QueryRow(ctx, "SELECT nullif(current_setting('salescompel.tenant_id',true),'')").Scan(&tenant); e != nil || tenant != nil {
			t.Fatal(mode, tenant, e)
		}
	}
}
