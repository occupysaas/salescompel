// schemafixture is an acceptance-only owner helper, never part of the server.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"regexp"
	"salescompel/internal/migration"
	"strings"
	"time"
)

type operation struct{ Tenant, Kind, Name, Object string }

func run(ctx context.Context) error {
	if os.Getenv("CANDIDATE_DISPOSABLE") != "yes" {
		return errors.New("disposable database required")
	}
	id := os.Getenv("CANDIDATE_RUN_ID")
	if !regexp.MustCompile(`^[a-f0-9]{24}$`).MatchString(id) {
		return errors.New("run binding required")
	}
	p, e := pgxpool.New(ctx, os.Getenv("MIGRATION_DATABASE_URL"))
	if e != nil {
		return e
	}
	defer p.Close()
	rrole, arole := "sc_r_"+id, "sc_a_"+id
	if len(os.Args) != 2 {
		return errors.New("action required")
	}
	switch os.Args[1] {
	case "bootstrap":
		tx, e := p.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		for _, name := range []string{"salescompel_owner", "salescompel_records", "salescompel_identity"} {
			var exists bool
			if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT FROM pg_roles WHERE rolname=$1)", name).Scan(&exists); e != nil {
				return e
			}
			if !exists {
				if _, e = tx.Exec(ctx, "CREATE ROLE "+pgx.Identifier{name}.Sanitize()+" NOLOGIN"); e != nil {
					return e
				}
			}
			var safe bool
			if e = tx.QueryRow(ctx, "SELECT NOT(rolcanlogin OR rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb OR rolreplication) FROM pg_roles WHERE rolname=$1", name).Scan(&safe); e != nil || !safe {
				return errors.New("unsafe fixture group")
			}
		}
		urls := map[string]string{}
		for _, item := range []struct{ name, group, key string }{{rrole, "salescompel_records", "records"}, {arole, "salescompel_identity", "identity"}} {
			var b [32]byte
			if _, e = rand.Read(b[:]); e != nil {
				return e
			}
			password := hex.EncodeToString(b[:])
			if _, e = tx.Exec(ctx, "CREATE ROLE "+pgx.Identifier{item.name}.Sanitize()+" LOGIN PASSWORD '"+password+"' IN ROLE "+pgx.Identifier{item.group}.Sanitize()); e != nil {
				return e
			}
			u, e := url.Parse(os.Getenv("MIGRATION_DATABASE_URL"))
			if e != nil {
				return e
			}
			u.User = url.UserPassword(item.name, password)
			urls[item.key] = u.String()
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
		if e = migration.Up(ctx, p); e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(urls)
	case "metadata-cleanup":
		var ops []operation
		if e = json.Unmarshal([]byte(os.Getenv("CANDIDATE_SCHEMA_OPERATIONS")), &ops); e != nil {
			return e
		}
		tx, e := p.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(739102)"); e != nil {
			return e
		}
		owned := func(name string) bool {
			return strings.HasPrefix(name, "P27_"+id[:12]) || strings.HasPrefix(name, "P28_"+id[:12])
		}
		for i := len(ops) - 1; i >= 0; i-- {
			op := ops[i]
			if !(owned(op.Name) || ((op.Kind == "field" || op.Kind == "stored") && owned(op.Object))) || !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*__c$`).MatchString(op.Name) || (op.Tenant != "00D"+id[:12]+"EAA" && op.Tenant != "00D"+id[12:]+"EAA") {
				return errors.New("unowned schema cleanup")
			}
			tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", op.Tenant)
			if op.Kind == "field" {
				var col, schema, table, obj string
				e = tx.QueryRow(ctx, "SELECT f.physical_column,o.physical_schema,o.physical_table,o.id FROM sc_catalog.fields f JOIN sc_catalog.objects o ON o.id=f.object_id AND o.tenant_id=f.tenant_id WHERE f.tenant_id=$1 AND f.api_name=$2 AND o.api_name=$3", op.Tenant, op.Name, op.Object).Scan(&col, &schema, &table, &obj)
				if errors.Is(e, pgx.ErrNoRows) {
					continue
				}
				if e != nil {
					return e
				}
				if !regexp.MustCompile(`^f_[a-f0-9]{32}$`).MatchString(col) {
					return errors.New("invalid owned column")
				}
				if _, e = tx.Exec(ctx, "ALTER TABLE "+pgx.Identifier{schema, table}.Sanitize()+" DROP COLUMN "+pgx.Identifier{col}.Sanitize()); e != nil {
					return e
				}
				if _, e = tx.Exec(ctx, "DELETE FROM sc_catalog.fields WHERE tenant_id=$1 AND object_id=$2 AND api_name=$3", op.Tenant, obj, op.Name); e != nil {
					return e
				}
			} else if op.Kind == "stored" {
				if _, e = tx.Exec(ctx, "DELETE FROM sc_catalog.artifacts a USING sc_catalog.objects o WHERE a.tenant_id=$1 AND o.tenant_id=a.tenant_id AND o.id=a.object_id AND o.api_name=$2 AND a.api_name=$3", op.Tenant, op.Object, op.Name); e != nil {
					return e
				}
			} else if op.Kind == "object" {
				var schema, table, object string
				e = tx.QueryRow(ctx, "SELECT physical_schema,physical_table,id FROM sc_catalog.objects WHERE tenant_id=$1 AND api_name=$2 AND NOT standard", op.Tenant, op.Name).Scan(&schema, &table, &object)
				if errors.Is(e, pgx.ErrNoRows) {
					continue
				}
				if e != nil {
					return e
				}
				if schema != "sc_records" || !regexp.MustCompile(`^o_[a-f0-9]{32}$`).MatchString(table) {
					return errors.New("invalid owned table")
				}
				if _, e = tx.Exec(ctx, "DROP TABLE "+pgx.Identifier{schema, table}.Sanitize()); e != nil {
					return e
				}
				if _, e = tx.Exec(ctx, "DELETE FROM sc_catalog.objects WHERE tenant_id=$1 AND id=$2", op.Tenant, object); e != nil {
					return e
				}
			} else {
				return errors.New("unknown schema fixture")
			}
		}
		return tx.Commit(ctx)
	case "roles-cleanup":
		for _, role := range []string{rrole, arole} {
			if _, e = p.Exec(ctx, "DROP ROLE IF EXISTS "+pgx.Identifier{role}.Sanitize()); e != nil {
				return e
			}
		}
		return nil
	}
	return fmt.Errorf("unknown fixture action")
}
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if run(ctx) != nil {
		fmt.Fprintln(os.Stderr, "schema fixture operation failed")
		os.Exit(1)
	}
}
