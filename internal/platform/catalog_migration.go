package platform

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// legacyCatalog is frozen from e9709d5's own catalog serialization.
//
//go:embed catalog_v1.json
var legacyCatalog string

const catalogVersion = 2

// ensureCatalog runs inside Open's advisory-locked transaction. It never commits.
func ensureCatalog(ctx context.Context, tx pgx.Tx) error {
	current, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	var version int
	var legacyMatches, currentMatches bool
	err = tx.QueryRow(ctx, `SELECT version, definition=$1::jsonb, definition=$2::jsonb FROM platform_catalog WHERE singleton`, legacyCatalog, string(current)).Scan(&version, &legacyMatches, &currentMatches)
	fresh := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !fresh {
		return err
	}
	if !fresh {
		if version == catalogVersion && currentMatches {
			return checkCustomColumnBounds(ctx, tx)
		}
		if version != 1 || !legacyMatches {
			return errors.New("database catalog version mismatch")
		}
	}
	// Prevent writes between the precheck and ALTER, including catalog-less data.
	if _, err = tx.Exec(ctx, "LOCK TABLE accounts, contacts IN ACCESS EXCLUSIVE MODE"); err != nil {
		return err
	}
	if fresh {
		var populated bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM accounts) OR EXISTS(SELECT 1 FROM contacts)").Scan(&populated); err != nil {
			return err
		}
		if populated {
			return errors.New("database catalog missing for existing records")
		}
	}
	var oversized bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE char_length(compat_text)>80 OR char_length(compat_key)>80)`).Scan(&oversized); err != nil {
		return err
	}
	if oversized {
		return errors.New("oversized custom-field values require operator remediation before catalog upgrade")
	}
	// No USING cast: PostgreSQL must reject rather than truncate any invalid data.
	if _, err = tx.Exec(ctx, `ALTER TABLE accounts ALTER COLUMN compat_text TYPE varchar(80), ALTER COLUMN compat_key TYPE varchar(80)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_catalog(singleton,version,definition) VALUES(true,$1,$2) ON CONFLICT(singleton) DO UPDATE SET version=EXCLUDED.version,definition=EXCLUDED.definition`, catalogVersion, string(current)); err != nil {
		return err
	}
	return checkCustomColumnBounds(ctx, tx)
}
func checkCustomColumnBounds(ctx context.Context, tx pgx.Tx) error {
	var matching int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM pg_attribute WHERE attrelid='accounts'::regclass AND attname IN ('compat_text','compat_key') AND NOT attisdropped AND atttypid='varchar'::regtype AND atttypmod=84`).Scan(&matching)
	if err != nil {
		return err
	}
	if matching != 2 {
		return errors.New("database custom-field schema mismatch")
	}
	return nil
}
