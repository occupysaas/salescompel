package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

func Load(ctx context.Context, tx pgx.Tx, tenant string) (Snapshot, error) {
	out := Snapshot{Tenant: tenant, Objects: []Object{}}
	if _, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(739102)"); e != nil {
		return out, e
	}
	if e := tx.QueryRow(ctx, "SELECT revision FROM sc_catalog.revisions WHERE tenant_id=$1", tenant).Scan(&out.Revision); e != nil {
		return out, e
	}
	rows, e := tx.Query(ctx, `SELECT o.id,o.api_name,o.label,o.plural_label,o.standard,o.physical_schema,o.physical_table,o.prefix,COALESCE((SELECT jsonb_agg(f.definition ORDER BY f.ordinal,f.api_name) FROM sc_catalog.fields f WHERE f.tenant_id=o.tenant_id AND f.object_id=o.id),'[]'::jsonb) FROM sc_catalog.objects o WHERE o.tenant_id=$1 ORDER BY o.api_name`, tenant)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		o := Object{Tenant: tenant, Createable: true, Updateable: true, Deletable: true, Queryable: true}
		var data []byte
		if e = rows.Scan(&o.ID, &o.Name, &o.Label, &o.PluralLabel, &o.Standard, &o.Schema, &o.Table, &o.Prefix, &data); e != nil {
			return out, e
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&o.Fields); e != nil {
			return out, e
		}
		for _, f := range o.Fields {
			if f.Type != "id" && f.Type != "string" && f.Type != "reference" {
				return out, errors.New("unsupported active field type")
			}
		}
		// Physical field mappings are separate from public describe JSON.
		out.Objects = append(out.Objects, o)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	for i := range out.Objects {
		o := &out.Objects[i]
		r, e := tx.Query(ctx, "SELECT id,api_name,physical_column FROM sc_catalog.fields WHERE tenant_id=$1 AND object_id=$2", tenant, o.ID)
		if e != nil {
			return out, e
		}
		for r.Next() {
			var id, name, col string
			r.Scan(&id, &name, &col)
			for j := range o.Fields {
				if o.Fields[j].Name == name {
					o.Fields[j].Column = col
					o.Fields[j].ID = id
				}
			}
		}
		e = r.Err()
		r.Close()
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
