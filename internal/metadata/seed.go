package metadata

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

func StandardObjects() []Object {
	return []Object{
		{Name: "Account", Label: "Account", PluralLabel: "Accounts", Standard: true, Schema: "public", Table: "accounts", Prefix: "001", Fields: []Field{
			{Name: "Id", Type: "id", Column: "id"}, {Name: "Name", Type: "string", Column: "name", Length: 255}, {Name: "Compat_Text__c", Type: "string", Column: "compat_text", Nillable: true, Length: 80}, {Name: "Compat_Key__c", Type: "string", Column: "compat_key", Nillable: true, ExternalID: true, Length: 80}}},
		{Name: "Contact", Label: "Contact", PluralLabel: "Contacts", Standard: true, Schema: "public", Table: "contacts", Prefix: "003", Fields: []Field{
			{Name: "Id", Type: "id", Column: "id"}, {Name: "FirstName", Type: "string", Column: "first_name", Nillable: true, Length: 255}, {Name: "LastName", Type: "string", Column: "last_name", Length: 255}, {Name: "AccountId", Type: "reference", Column: "account_id", Nillable: true, Length: 18, ReferenceTo: []string{"Account"}}}},
	}
}
func insertField(ctx context.Context, tx pgx.Tx, tenant, obj string, f Field, ordinal int) error {
	f.Label = f.Name
	if f.ReferenceTo == nil {
		f.ReferenceTo = []string{}
	}
	f.Createable = f.Name != "Id"
	f.Updateable = f.Createable
	id := f.ID
	if id == "" {
		id = randomID()
	}
	raw, _ := json.Marshal(f)
	_, e := tx.Exec(ctx, "INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES($1,$2,$3,$4,$5,$6,$7)", id, tenant, obj, f.Name, f.Column, string(raw), ordinal)
	return e
}
func SeedTenant(ctx context.Context, tx pgx.Tx, tenant string) error {
	if _, e := tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenant); e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, "INSERT INTO sc_catalog.revisions(tenant_id,revision) VALUES($1,1) ON CONFLICT DO NOTHING", tenant)
	if e != nil || tag.RowsAffected() == 0 {
		return e
	}
	for _, o := range StandardObjects() {
		id := randomID()
		if _, e = tx.Exec(ctx, `INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) VALUES($1,$2,$3,$4,$5,true,$6,$7,$8)`, id, tenant, o.Name, o.Label, o.PluralLabel, o.Schema, o.Table, o.Prefix); e != nil {
			return e
		}
		for i, f := range o.Fields {
			if e = insertField(ctx, tx, tenant, id, f, i); e != nil {
				return e
			}
		}
	}
	return nil
}
