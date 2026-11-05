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
