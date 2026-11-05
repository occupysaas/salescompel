package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"salescompel/internal/metadata"
	"salescompel/internal/migration"
	"strings"
)

func schemaCommand(ctx context.Context, args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: schema object create|field create|inspect")
	}
	url := os.Getenv("MIGRATION_DATABASE_URL")
	if url == "" {
		return errors.New("MIGRATION_DATABASE_URL required")
	}
	pool, e := pgxpool.New(ctx, url)
	if e != nil {
		return errors.New("invalid owner database configuration")
	}
	defer pool.Close()
	var owner bool
	if e = pool.QueryRow(ctx, "SELECT pg_has_role(current_user,'salescompel_owner','MEMBER')").Scan(&owner); e != nil || !owner {
		return errors.New("schema operations require owner membership")
	}
	if e = migration.CheckHead(ctx, pool); e != nil {
		return e
	}
	kind := args[1]
	offset := 2
	if kind != "inspect" {
		if len(args) < 3 || args[2] != "create" {
			return errors.New("only additive schema create is supported")
		}
		offset = 3
	}
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenant := fs.String("tenant", "", "tenant")
	name := fs.String("name", "", "API name")
	label := fs.String("label", "", "label")
	plural := fs.String("plural-label", "", "plural label")
	object := fs.String("object", "", "object")
	typ := fs.String("type", "", "type")
	length := fs.Int("length", 0, "length")
	seen := map[string]bool{}
	for _, arg := range args[offset:] {
		if strings.HasPrefix(arg, "-") {
			k := "--" + strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
			if seen[k] {
				return errors.New("duplicate schema flag")
			}
			seen[k] = true
		}
	}
	if e = fs.Parse(args[offset:]); e != nil || fs.NArg() != 0 || *tenant == "" {
		return errors.New("invalid schema arguments; tenant required")
	}
	switch kind {
	case "object":
		if seen["--object"] || seen["--type"] || seen["--length"] {
			return errors.New("unsupported object property")
		}
		v, e := metadata.CreateObject(ctx, pool, *tenant, metadata.ObjectInput{Name: *name, Label: *label, PluralLabel: *plural})
		if e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(v)
	case "field":
		if seen["--plural-label"] {
			return errors.New("unsupported field property")
		}
		v, e := metadata.CreateField(ctx, pool, *tenant, metadata.FieldInput{Object: *object, Name: *name, Label: *label, Type: *typ, Length: *length})
		if e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(v)
	case "inspect":
		if len(seen) != 1 {
			return errors.New("inspect accepts only --tenant")
		}
		tx, e := pool.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", *tenant)
		s, e := metadata.Load(ctx, tx, *tenant)
		if e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(s)
	}
	return errors.New("unknown schema command")
}
