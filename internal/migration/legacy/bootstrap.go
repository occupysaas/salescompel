// Package legacy freezes the v1-v3 database adoption rules. Runtime never imports it.
package legacy

import (
	"context"
	_ "embed"
	"github.com/jackc/pgx/v5"
)

//go:embed schema.sql
var Schema string

func fieldProblem(code, message, field string) error {
	return &APIError{Status: 400, Code: code, Message: message, Fields: []string{field}}
}
func Install(ctx context.Context, tx pgx.Tx) error {
	if _, e := tx.Exec(ctx, Schema); e != nil {
		return e
	}
	return EnsureCatalog(ctx, tx)
}
