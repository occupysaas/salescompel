package migration

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
)

// errCollation explains a migration 1 refusal caused by the database's
// default collation rather than by its schema (issue #43).
var errCollation = errors.New("database default collation orders text differently from the recorded legacy schema, as byte-order collations such as C, POSIX and C.UTF-8 do; create the database with a linguistic collation such as ICU en-US; no upgrade was applied")

// adoptChecked is migration 1: the frozen adoption step, preceded by the
// collation check.
func adoptChecked(ctx context.Context, tx pgx.Tx) error {
	if e := checkCollation(ctx, tx); e != nil {
		return e
	}
	return adopt(ctx, tx)
}

// checkCollation runs before migration 1. The frozen adoption check
// (legacy.ValidateIdentity) sorts constraint definitions with the database's
// default collation and compares the order exactly with
// legacy/identity_schema.json, which was recorded under a linguistic
// collation. This sorts the recorded lists the same way the check will and
// refuses early, with an actionable error, if the order differs. It lives
// outside the files hashed into migration 1's checksum, so released installs
// keep validating.
func checkCollation(ctx context.Context, tx pgx.Tx) error {
	raw, e := sources.ReadFile("legacy/identity_schema.json")
	if e != nil {
		return e
	}
	var tables map[string]struct {
		Constraints []string `json:"constraints"`
	}
	if e = json.Unmarshal(raw, &tables); e != nil {
		return e
	}
	for _, name := range slices.Sorted(maps.Keys(tables)) {
		recorded := tables[name].Constraints
		if len(recorded) < 2 {
			continue
		}
		var sorted []string
		if e = tx.QueryRow(ctx, "SELECT array_agg(c ORDER BY c) FROM unnest($1::text[]) AS c", recorded).Scan(&sorted); e != nil {
			return e
		}
		if !slices.Equal(sorted, recorded) {
			return errCollation
		}
	}
	return nil
}
