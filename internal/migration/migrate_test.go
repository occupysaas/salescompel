package migration

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL required")
	}
	root, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	name := fmt.Sprintf("migration_%d", time.Now().UnixNano())
	if _, e = root.Exec(ctx, "CREATE DATABASE "+name); e != nil {
		t.Fatal(e)
	}
	cfg, _ := pgxpool.ParseConfig(url)
	cfg.ConnConfig.Database = name
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		p.Close()
		r, _ := pgxpool.New(ctx, url)
		defer r.Close()
		r.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
	})
	return p
}
func TestFreshMigration(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e := CheckHead(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	entries, e := Status(ctx, p)
	if e != nil || len(entries) < 1 || !entries[0].Applied || len(entries[0].Checksum) != 64 {
		t.Fatal(entries, e)
	}
}
func TestMigrationHistory(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	if e := Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Exec(ctx, "UPDATE sc_control.schema_migrations SET checksum='tampered' WHERE version=1"); e != nil {
		t.Fatal(e)
	}
	if e := Up(ctx, p); e == nil {
		t.Fatal("accepted changed history")
	}
}
func TestUnmigratedHead(t *testing.T) {
	p := testPool(t)
	if e := CheckHead(context.Background(), p); e == nil {
		t.Fatal("accepted unmigrated database")
	}
}
func TestMigrationRejectsUnexplained(t *testing.T) {
	p := testPool(t)
	ctx := context.Background()
	p.Exec(ctx, "CREATE TABLE public.unknown(id int)")
	if e := Up(ctx, p); e == nil {
		t.Fatal("overwrote unexplained database")
	}
}
