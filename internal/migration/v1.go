package migration

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"salescompel/internal/migration/legacy"
)

func adopt(ctx context.Context, tx pgx.Tx) error {
	var tables []string
	rows, e := tx.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname=current_schema() ORDER BY tablename")
	if e != nil {
		return e
	}
	for rows.Next() {
		var n string
		rows.Scan(&n)
		tables = append(tables, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	base := map[string]bool{"accounts": true, "contacts": true, "tenants": true, "principals": true, "platform_catalog": true}
	identity := map[string]bool{"users": true, "oauth_apps": true, "oauth_app_jwt_users": true, "oauth_requests": true, "oauth_codes": true, "oauth_refresh_tokens": true, "oauth_access_tokens": true, "oauth_jwt_ids": true}
	b, i := 0, 0
	for _, t := range tables {
		if base[t] {
			b++
		} else if identity[t] {
			i++
		} else {
			return errors.New("unexplained application table; migration refused")
		}
	}
	if len(tables) > 0 && (b != 5 || (i != 0 && i != 8)) {
		return errors.New("partial legacy database; migration refused")
	}
	if e = legacy.Install(ctx, tx); e != nil {
		return e
	}
	return legacy.ValidateIdentity(ctx, tx)
}
