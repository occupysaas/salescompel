package platform

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"regexp"
)

// Require the exact application privilege matrix, including direct grants in
// addition to inherited group privileges. No record connection may read secrets.
func runtimeTablesSafe(ctx context.Context, p *pgxpool.Pool, group string) (bool, error) {
	rows, e := p.Query(ctx, `SELECT n.nspname,c.relname,c.relrowsecurity,c.relforcerowsecurity,
 (SELECT count(*)=1 AND bool_and(polname='tenant_isolation' AND polcmd='*' AND polpermissive AND polroles=ARRAY[0::oid] AND pg_get_expr(polqual,polrelid)=$1 AND pg_get_expr(polwithcheck,polrelid)=$1) FROM pg_policy WHERE polrelid=c.oid),
 ARRAY(SELECT a.attname::text FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped AND has_column_privilege(current_user,c.oid,a.attnum,'UPDATE') ORDER BY a.attnum),
 ARRAY[has_any_column_privilege(current_user,c.oid,'SELECT'),has_any_column_privilege(current_user,c.oid,'INSERT'),has_any_column_privilege(current_user,c.oid,'UPDATE'),has_any_column_privilege(current_user,c.oid,'REFERENCES')],
 ARRAY[has_table_privilege(current_user,c.oid,'SELECT'),has_table_privilege(current_user,c.oid,'INSERT'),has_table_privilege(current_user,c.oid,'UPDATE'),has_table_privilege(current_user,c.oid,'DELETE'),has_table_privilege(current_user,c.oid,'TRUNCATE'),has_table_privilege(current_user,c.oid,'REFERENCES'),has_table_privilege(current_user,c.oid,'TRIGGER'),has_table_privilege(current_user,c.oid,'MAINTAIN')]
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname IN ('public','sc_catalog','sc_records','sc_query','sc_control') AND c.relkind IN ('r','p','v','m')`, "(tenant_id = NULLIF(current_setting('salescompel.tenant_id'::text, true), ''::text))")
	if e != nil {
		return false, e
	}
	defer rows.Close()
	catalog := map[string]bool{"objects": true, "fields": true, "revisions": true, "artifacts": true, "settings": true, "auto_numbers": true}
	identityRead := map[string]bool{"principals": true, "oauth_apps": true, "oauth_app_jwt_users": true}
	identityWrite := map[string]bool{"oauth_requests": true, "oauth_codes": true, "oauth_refresh_tokens": true, "oauth_access_tokens": true, "oauth_jwt_ids": true}
	for rows.Next() {
		var schema, table string
		var rls, force, closed bool
		var got, columns []bool
		var writable []string
		if e = rows.Scan(&schema, &table, &rls, &force, &closed, &writable, &columns, &got); e != nil {
			return false, e
		}
		want := make([]bool, 8)
		record := (schema == "public" && (table == "accounts" || table == "contacts")) || (schema == "sc_records" && regexp.MustCompile(`^o_[a-f0-9]{32}$`).MatchString(table))
		meta := schema == "sc_catalog" && catalog[table]
		locator := schema == "sc_query" && table == "locators"
		if (record || meta || locator) && (!rls || !force || !closed) {
			return false, nil
		}
		if schema == "sc_control" && table == "schema_migrations" {
			want[0] = true
		}
		if group == "salescompel_records" {
			if record {
				for i := 0; i < 4; i++ {
					want[i] = true
				}
			} else if meta {
				want[0] = true
				// Auto-number counters advance inside record transactions.
				want[2] = table == "auto_numbers"
			} else if locator {
				// Locators are append-only from the runtime: SELECT, INSERT, DELETE.
				want[0], want[1], want[3] = true, true, true
			}
		} else if schema == "public" {
			if identityRead[table] {
				want[0] = true
			}
			if identityWrite[table] {
				for i := 0; i < 4; i++ {
					want[i] = true
				}
			}
			if table == "users" {
				want[0] = true
				want[2] = true
			}
		}
		appLockColumn := group == "salescompel_identity" && schema == "public" && table == "oauth_apps"
		if appLockColumn && (len(writable) != 1 || writable[0] != "name") {
			return false, nil
		}
		for i, wantIndex := range []int{0, 1, 2, 5} {
			if !want[wantIndex] && columns[i] && !(appLockColumn && wantIndex == 2) {
				return false, nil
			}
		}
		for i := range want {
			if got[i] != want[i] {
				return false, nil
			}
		}
	}
	return true, rows.Err()
}
