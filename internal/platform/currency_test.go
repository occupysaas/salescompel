package platform

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestProvisionCurrency(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	p := Principal{Tenant: fmt.Sprintf("cur%d", time.Now().UnixNano()), User: "u", Role: "admin"}
	if err := s.Provision(ctx, "token-"+p.Tenant, p); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCurrency(ctx, p.Tenant, "JPY"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCurrency(ctx, p.Tenant, "yen"); err == nil {
		t.Fatal("invalid currency accepted")
	}
	var code string
	if err := s.pool.QueryRow(ctx, "SELECT currency_iso_code FROM sc_catalog.settings WHERE tenant_id=$1", p.Tenant).Scan(&code); err != nil || code != "JPY" {
		t.Fatalf("currency %q %v", code, err)
	}
}
