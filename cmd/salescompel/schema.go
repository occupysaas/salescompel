package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
	"salescompel/internal/migration"
	"strings"
)

// schemaRequest is one parsed schema command.
type schemaRequest struct {
	Tenant string
	Object metadata.ObjectInput
	Field  metadata.FieldInput
	Stored []byte // stored-not-enforced definition source, when given
	Code   string // currency code
}

// definitionFile is the JSON form accepted by --definition for active types.
type definitionFile struct {
	Name, Label, Type string
	Length            int
	Precision, Scale  int
	VisibleLines      int `json:"visibleLines"`
	Values            []struct {
		Value, Label string
		Default      bool
	}
	Restricted, Required, Reparentable bool
	Default                            *string
	ReferenceTo                        string `json:"referenceTo"`
	DeleteBehavior                     string `json:"deleteBehavior"`
	DisplayFormat                      string `json:"displayFormat"`
	StartingNumber                     *int64 `json:"startingNumber"`
	RelationshipName                   string `json:"relationshipName"`
}

var schemaFlags = map[string][]string{
	"object":   {"tenant", "name", "label", "plural-label", "name-type", "display-format", "starting-number"},
	"field":    {"tenant", "object", "name", "label", "type", "length", "precision", "scale", "visible-lines", "values", "restricted", "default", "required", "reference-to", "delete-behavior", "reparentable", "display-format", "starting-number", "relationship-name", "definition"},
	"inspect":  {"tenant"},
	"currency": {"tenant", "code"},
}

func values(list string) []metadata.PicklistValue {
	out := []metadata.PicklistValue{}
	for _, v := range strings.Split(list, ",") {
		out = append(out, metadata.PicklistValue{Value: v, Label: v, Active: true})
	}
	return out
}

// parseSchemaArgs validates flags for one schema command without touching
// the database. Unknown, duplicate and irrelevant flags are rejected.
func parseSchemaArgs(kind string, args []string) (schemaRequest, error) {
	allowed, ok := schemaFlags[kind]
	if !ok {
		return schemaRequest{}, errors.New("unknown schema command")
	}
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	str := map[string]*string{}
	ints := map[string]*int{}
	bools := map[string]*bool{}
	var start int64
	for _, name := range allowed {
		switch name {
		case "length", "precision", "scale", "visible-lines":
			ints[name] = fs.Int(name, 0, name)
		case "restricted", "required", "reparentable":
			bools[name] = fs.Bool(name, false, name)
		case "starting-number":
			fs.Int64Var(&start, name, 1, name)
		default:
			str[name] = fs.String(name, "", name)
		}
	}
	seen := map[string]bool{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			k := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
			if seen[k] {
				return schemaRequest{}, errors.New("duplicate schema flag")
			}
			seen[k] = true
		}
	}
	if e := fs.Parse(args); e != nil || fs.NArg() != 0 || *str["tenant"] == "" {
		return schemaRequest{}, errors.New("invalid schema arguments; tenant required")
	}
	r := schemaRequest{Tenant: *str["tenant"]}
	switch kind {
	case "object":
		r.Object = metadata.ObjectInput{Name: *str["name"], Label: *str["label"], PluralLabel: *str["plural-label"], NameType: *str["name-type"], DisplayFormat: *str["display-format"]}
		if r.Object.NameType == "AutoNumber" {
			r.Object.StartingNumber = start
		} else if seen["starting-number"] {
			return r, errors.New("starting number applies only to an AutoNumber name")
		}
	case "currency":
		r.Code = *str["code"]
	case "field":
		if path := *str["definition"]; path != "" {
			for k := range seen {
				if k != "tenant" && k != "object" && k != "definition" {
					return r, errors.New("--definition accepts only --tenant and --object")
				}
			}
			return parseDefinition(r, *str["object"], path)
		}
		f := metadata.FieldInput{Object: *str["object"], Name: *str["name"], Label: *str["label"], Type: *str["type"],
			Length: *ints["length"], Precision: *ints["precision"], Scale: *ints["scale"], VisibleLines: *ints["visible-lines"],
			Restricted: *bools["restricted"], Required: *bools["required"], Reparentable: *bools["reparentable"],
			ReferenceTo: *str["reference-to"], DeleteBehavior: *str["delete-behavior"], DisplayFormat: *str["display-format"], RelationshipName: *str["relationship-name"]}
		if seen["values"] {
			f.Values = values(*str["values"])
		}
		if seen["default"] {
			d := *str["default"]
			f.Default = &d
		}
		if f.Type == "AutoNumber" {
			f.StartingNumber = start
		} else if seen["starting-number"] {
			return r, errors.New("starting number applies only to AutoNumber fields")
		}
		r.Field = f
	}
	return r, nil
}

func parseDefinition(r schemaRequest, object, path string) (schemaRequest, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return r, errors.New("cannot read definition file")
	}
	var head struct{ Type string }
	if json.Unmarshal(raw, &head) != nil {
		return r, errors.New("definition must be a JSON object")
	}
	if k, e := fieldtype.ParseKind(head.Type); e == nil && k.Stored() {
		r.Field.Object, r.Stored = object, raw
		return r, nil
	}
	var d definitionFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&d); e != nil {
		return r, errors.New("invalid definition: " + e.Error())
	}
	f := metadata.FieldInput{Object: object, Name: d.Name, Label: d.Label, Type: d.Type, Length: d.Length, Precision: d.Precision, Scale: d.Scale,
		VisibleLines: d.VisibleLines, Restricted: d.Restricted, Required: d.Required, Reparentable: d.Reparentable, Default: d.Default,
		ReferenceTo: d.ReferenceTo, DeleteBehavior: d.DeleteBehavior, DisplayFormat: d.DisplayFormat, RelationshipName: d.RelationshipName}
	for _, v := range d.Values {
		f.Values = append(f.Values, metadata.PicklistValue{Value: v.Value, Label: v.Label, Active: true})
		if v.Default {
			if f.Default != nil {
				return r, errors.New("only one default picklist value is allowed")
			}
			def := v.Value
			f.Default = &def
		}
	}
	if d.StartingNumber != nil {
		f.StartingNumber = *d.StartingNumber
	} else if d.Type == "AutoNumber" {
		f.StartingNumber = 1
	}
	r.Field = f
	return r, nil
}

func schemaCommand(ctx context.Context, args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: schema object create|field create|currency set|inspect")
	}
	kind, offset := args[1], 2
	switch kind {
	case "inspect":
	case "object", "field":
		if len(args) < 3 || args[2] != "create" {
			return errors.New("only additive schema create is supported")
		}
		offset = 3
	case "currency":
		if len(args) < 3 || args[2] != "set" {
			return errors.New("usage: schema currency set --tenant T --code EUR")
		}
		offset = 3
	default:
		return errors.New("unknown schema command")
	}
	req, e := parseSchemaArgs(kind, args[offset:])
	if e != nil {
		return e
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
	var v any
	switch kind {
	case "object":
		v, e = metadata.CreateObject(ctx, pool, req.Tenant, req.Object)
	case "field":
		if req.Stored != nil {
			v, e = metadata.CreateStoredField(ctx, pool, req.Tenant, req.Field.Object, req.Stored)
		} else {
			v, e = metadata.CreateField(ctx, pool, req.Tenant, req.Field)
		}
	case "currency":
		e = metadata.SetCurrency(ctx, pool, req.Tenant, req.Code)
		v = map[string]string{"tenant": req.Tenant, "currency": req.Code}
	case "inspect":
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", req.Tenant)
		s, err := metadata.Load(ctx, tx, req.Tenant)
		if err != nil {
			return err
		}
		v = metadata.Inspect(s)
	}
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(v)
}
