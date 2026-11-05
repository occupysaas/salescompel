package legacy

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"reflect"
)

//go:embed identity_schema.sql
var identitySQL string

//go:embed identity_schema.json
var identityExpected []byte

func ValidateIdentity(ctx context.Context, tx pgx.Tx) error {
	var policies bool
	if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND (c.relrowsecurity OR c.relforcerowsecurity OR EXISTS(SELECT FROM pg_policy p WHERE p.polrelid=c.oid)))`).Scan(&policies); e != nil {
		return e
	}
	if policies {
		return errors.New("unexpected legacy row security configuration; migration refused")
	}
	var actual []byte
	if e := tx.QueryRow(ctx, identitySQL).Scan(&actual); e != nil {
		return e
	}
	var got, want any
	if json.Unmarshal(actual, &got) != nil || json.Unmarshal(identityExpected, &want) != nil || !reflect.DeepEqual(got, want) {
		return errors.New("legacy identity schema mismatch; no upgrade was applied")
	}
	return nil
}
