package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"salescompel/internal/migration"
)

func migrateCommand(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 2 || (args[1] != "status" && args[1] != "up") {
		return errors.New("usage: salescompel migrate status|up")
	}
	url := os.Getenv("MIGRATION_DATABASE_URL")
	if url == "" {
		return errors.New("MIGRATION_DATABASE_URL required")
	}
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return errors.New("invalid migration database configuration")
	}
	defer p.Close()
	if args[1] == "up" {
		return migration.Up(ctx, p)
	}
	rows, e := migration.Status(ctx, p)
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(rows)
}
