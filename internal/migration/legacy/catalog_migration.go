package legacy

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

const catalogVersion = 3

// ensureCatalog runs inside Open's advisory-locked transaction. It never commits.
func EnsureCatalog(ctx context.Context, tx pgx.Tx) error {
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
		if !((version == 1 && legacyMatches) || (version == 2 && currentMatches)) {
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
	if version == 2 {
		if err = checkCustomColumnBoundsV2(ctx, tx); err != nil {
			return err
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
	if err = upgradeExternalIDs(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_catalog(singleton,version,definition) VALUES(true,$1,$2) ON CONFLICT(singleton) DO UPDATE SET version=EXCLUDED.version,definition=EXCLUDED.definition`, catalogVersion, string(current)); err != nil {
		return err
	}
	return checkCustomColumnBounds(ctx, tx)
}
func checkCustomColumnBoundsV2(ctx context.Context, tx pgx.Tx) error {
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

// upgradeExternalIDs holds the exclusive table lock until the startup transaction commits.
func upgradeExternalIDs(ctx context.Context, tx pgx.Tx) error {
	type row struct{ tenant, id, key, canonical string }
	rows, err := tx.Query(ctx, "SELECT tenant_id,id,compat_key FROM accounts WHERE compat_key IS NOT NULL")
	if err != nil {
		return errors.New("external-ID migration read failed")
	}
	pending := []row{}
	seen := map[[2]string]bool{}
	for rows.Next() {
		var r row
		if err = rows.Scan(&r.tenant, &r.id, &r.key); err != nil {
			rows.Close()
			return errors.New("external-ID migration read failed")
		}
		r.canonical, err = canonicalExternalID(r.key)
		if err != nil || r.canonical == "" {
			rows.Close()
			return errors.New("blank or invalid external IDs require operator remediation before catalog upgrade")
		}
		identity := [2]string{r.tenant, r.canonical}
		if seen[identity] {
			rows.Close()
			return errors.New("colliding external IDs require operator remediation before catalog upgrade")
		}
		seen[identity] = true
		pending = append(pending, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return errors.New("external-ID migration read failed")
	}
	if _, err = tx.Exec(ctx, "ALTER TABLE accounts ADD COLUMN compat_key_canonical text"); err != nil {
		return errors.New("external-ID migration schema failed")
	}
	for _, r := range pending {
		if _, err = tx.Exec(ctx, "UPDATE accounts SET compat_key_canonical=$1 WHERE tenant_id=$2 AND id=$3", r.canonical, r.tenant, r.id); err != nil {
			return errors.New("external-ID migration write failed")
		}
	}
	_, err = tx.Exec(ctx, `ALTER TABLE accounts
 ADD CONSTRAINT accounts_compat_key_pair CHECK ((compat_key IS NULL) = (compat_key_canonical IS NULL)),
 ADD CONSTRAINT accounts_tenant_id_compat_key_canonical_key UNIQUE(tenant_id,compat_key_canonical),
 DROP CONSTRAINT accounts_tenant_id_compat_key_key`)
	if err != nil {
		return errors.New("external-ID migration constraints failed")
	}
	return nil
}
func checkCustomColumnBounds(ctx context.Context, tx pgx.Tx) error {
	if err := checkCustomColumnBoundsV2(ctx, tx); err != nil {
		return err
	}
	var columnOK, uniqueOK, pairOK, oldUnique bool
	err := tx.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='accounts'::regclass AND attname='compat_key_canonical' AND NOT attisdropped AND atttypid='text'::regtype AND NOT attnotnull),
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='accounts'::regclass AND conname='accounts_tenant_id_compat_key_canonical_key' AND contype='u' AND convalidated AND pg_get_constraintdef(oid)='UNIQUE (tenant_id, compat_key_canonical)'),
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='accounts'::regclass AND conname='accounts_compat_key_pair' AND contype='c' AND convalidated AND pg_get_constraintdef(oid)='CHECK (((compat_key IS NULL) = (compat_key_canonical IS NULL)))'),
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='accounts'::regclass AND conname='accounts_tenant_id_compat_key_key')`).Scan(&columnOK, &uniqueOK, &pairOK, &oldUnique)
	if err != nil {
		return errors.New("database external-ID schema check failed")
	}
	if !columnOK || !uniqueOK || !pairOK || oldUnique {
		return errors.New("database external-ID schema mismatch")
	}
	return nil
}
