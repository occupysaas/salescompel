package platform

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The share views run as the caller, so the limited records login must be
// able to read them: they may not call functions it can't execute
// (sc_setup.id_checksum). The computed Owner row ID must still carry the
// right checksum.
func TestRuntimeReadsShareViews(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	if _, e := s.CreateUserWithProfile(ctx, a.Tenant, "views."+a.Tenant+"@example.com", "long-password", "System Administrator"); e != nil {
		t.Fatal(e)
	}
	account, e := s.Create(ctx, a, "Account", map[string]any{"Name": "Share view"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(ctx, a, "Contact", map[string]any{"LastName": "Share view", "AccountId": account}); e != nil {
		t.Fatal(e)
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
	for _, view := range []string{"account_shares", "contact_shares"} {
		var id string
		if e = tx.QueryRow(ctx, "SELECT id FROM sc_setup."+view+" WHERE row_cause='Owner' LIMIT 1").Scan(&id); e != nil {
			t.Fatalf("%s as the records login: %v", view, e)
		}
		var want string
		if e = s.pool.QueryRow(ctx, "SELECT left($1::text,15)||sc_setup.id_checksum(left($1::text,15))", id).Scan(&want); e != nil || want != id {
			t.Fatalf("%s owner row ID %s, checksum gives %s (%v)", view, id, want, e)
		}
	}
}
