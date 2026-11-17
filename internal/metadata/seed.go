package metadata

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/fieldtype"
	"salescompel/internal/recordid"
)

func ptr[T any](v T) *T { return &v }

func picklist(values ...string) []PicklistValue {
	out := make([]PicklistValue, len(values))
	for i, v := range values {
		out[i] = PicklistValue{Active: true, Label: v, Value: v}
	}
	return out
}

// std builds a writable standard field of kind k.
func std(name, label, column string, k fieldtype.Kind, length int) Field {
	return Field{Name: name, Label: label, Column: column, Nillable: true, Createable: true, Updateable: true, Length: length, Options: Options{Kind: k}}
}

func address(prefix, label, column string) []Field {
	parent := prefix + "Address"
	part := func(name, plabel, col string, k fieldtype.Kind, length int) Field {
		f := std(prefix+name, label+" "+plabel, column+"_"+col, k, length)
		f.CompoundFieldName = ptr(parent)
		return f
	}
	return []Field{
		part("Street", "Street", "street", fieldtype.TextArea, 255),
		part("City", "City", "city", fieldtype.Text, 40),
		part("State", "State/Province", "state", fieldtype.Text, 80),
		part("PostalCode", "Zip/Postal Code", "postal_code", fieldtype.Text, 20),
		part("Country", "Country", "country", fieldtype.Text, 80),
		{Name: parent, Label: label + " Address", Nillable: true, Options: Options{Kind: fieldtype.Address, Components: []string{prefix + "Street", prefix + "City", prefix + "State", prefix + "PostalCode", prefix + "Country"}}},
	}
}

// Standard picklist values follow Salesforce's documented defaults for the
// standard Account Type, Industry and Contact Salutation fields
// (x-source: Salesforce Object Reference, Account and Contact).
var (
	accountTypes = picklist("Prospect", "Customer - Direct", "Customer - Channel", "Channel Partner / Reseller", "Installation Partner", "Technology Partner", "Other")
	industries   = picklist("Agriculture", "Apparel", "Banking", "Biotechnology", "Chemicals", "Communications", "Construction", "Consulting", "Education", "Electronics", "Energy", "Engineering", "Entertainment", "Environmental", "Finance", "Food & Beverage", "Government", "Healthcare", "Hospitality", "Insurance", "Machinery", "Manufacturing", "Media", "Not For Profit", "Other", "Recreation", "Retail", "Shipping", "Technology", "Telecommunications", "Transportation", "Utilities")
	salutations  = picklist("Mr.", "Ms.", "Mrs.", "Dr.", "Prof.", "Mx.")
)

// StandardObjects returns the seeded Account and Contact definitions, then
// the Plan 33 setup objects. Plan 27 fields keep their order, labels and
// columns; Plan 28 fields follow.
func StandardObjects() []Object {
	id := Field{Name: "Id", Column: "id", Options: Options{Kind: fieldtype.Id}}
	accountName := Field{Name: "Name", Column: "name", Length: 255, Createable: true, Updateable: true, NameField: true, Options: Options{Kind: fieldtype.Text}}
	compatText := Field{Name: "Compat_Text__c", Column: "compat_text", Nillable: true, Length: 80, Createable: true, Updateable: true, Options: Options{Kind: fieldtype.Text}}
	compatKey := Field{Name: "Compat_Key__c", Column: "compat_key", Nillable: true, ExternalID: true, Length: 80, Createable: true, Updateable: true, Options: Options{Kind: fieldtype.Text}}
	accountType := std("Type", "Account Type", "type", fieldtype.Picklist, 255)
	accountType.PicklistValues = accountTypes
	industry := std("Industry", "Industry", "industry", fieldtype.Picklist, 255)
	industry.PicklistValues = industries
	revenue := std("AnnualRevenue", "Annual Revenue", "annual_revenue", fieldtype.Currency, 0)
	revenue.Precision = 18
	employees := std("NumberOfEmployees", "Employees", "number_of_employees", fieldtype.Number, 0)
	employees.Type, employees.Digits = "int", 8
	account := []Field{id, accountName, compatText, compatKey,
		std("Phone", "Account Phone", "phone", fieldtype.Phone, 40),
		std("Fax", "Account Fax", "fax", fieldtype.Phone, 40),
		std("Website", "Website", "website", fieldtype.Url, 255),
		accountType, industry, revenue, employees,
		std("Description", "Account Description", "description", fieldtype.LongTextArea, 32000),
	}
	account = append(account, address("Billing", "Billing", "billing")...)
	account = append(account, address("Shipping", "Shipping", "shipping")...)

	first := Field{Name: "FirstName", Column: "first_name", Nillable: true, Length: 255, Createable: true, Updateable: true, CompoundFieldName: ptr("Name"), Options: Options{Kind: fieldtype.Text}}
	last := Field{Name: "LastName", Column: "last_name", Length: 255, Createable: true, Updateable: true, CompoundFieldName: ptr("Name"), Options: Options{Kind: fieldtype.Text}}
	accountID := Field{Name: "AccountId", Column: "account_id", Nillable: true, Length: 18, Createable: true, Updateable: true, ReferenceTo: []string{"Account"}, RelationshipName: ptr("Account"), Options: Options{Kind: fieldtype.Lookup, DeleteBehavior: "Restrict", ChildRelationshipName: "Contacts"}}
	salutation := std("Salutation", "Salutation", "salutation", fieldtype.Picklist, 255)
	salutation.PicklistValues = salutations
	salutation.CompoundFieldName = ptr("Name")
	fullName := Field{Name: "Name", Label: "Full Name", Column: "name", Length: 511, NameField: true, Options: Options{Kind: fieldtype.PersonName, Generated: true, Components: []string{"Salutation", "FirstName", "LastName"}}}
	birthdate := std("Birthdate", "Birthdate", "birthdate", fieldtype.Date, 0)
	contact := []Field{id, first, last, accountID, salutation, fullName,
		std("Title", "Title", "title", fieldtype.Text, 128),
		std("Department", "Department", "department", fieldtype.Text, 80),
		std("Email", "Email", "email", fieldtype.Email, 80),
		std("Phone", "Business Phone", "phone", fieldtype.Phone, 40),
		std("MobilePhone", "Mobile Phone", "mobile_phone", fieldtype.Phone, 40),
		birthdate,
		std("Description", "Contact Description", "description", fieldtype.LongTextArea, 32000),
	}
	contact = append(contact, address("Mailing", "Mailing", "mailing")...)
	for _, fs := range [][]Field{account, contact} {
		for i := range fs {
			fs[i] = Complete(fs[i])
		}
	}
	account, contact = append(account, OwnerField()), append(contact, OwnerField())
	return append([]Object{
		{Name: "Account", Label: "Account", PluralLabel: "Accounts", Standard: true, Schema: "public", Table: "accounts", Prefix: "001", Fields: account},
		{Name: "Contact", Label: "Contact", PluralLabel: "Contacts", Standard: true, Schema: "public", Table: "contacts", Prefix: "003", Fields: contact},
	}, SetupObjects()...)
}

func insertField(ctx context.Context, tx pgx.Tx, tenant, obj string, f Field, ordinal int) error {
	if f.Name == "OwnerId" && f.Options.Kind == fieldtype.Lookup {
		f = OwnerField() // Complete would clear DefaultedOnCreate
	} else {
		f = Complete(f)
	}
	id := f.ID
	if id == "" {
		id = randomID()
	}
	raw, e := EncodeDefinition(f)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES($1,$2,$3,$4,$5,$6,$7)", id, tenant, obj, f.Name, f.Column, string(raw), ordinal)
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
	if _, e = tx.Exec(ctx, "INSERT INTO sc_catalog.settings(tenant_id) VALUES($1)", tenant); e != nil {
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
	if e = seedProfiles(ctx, tx, tenant); e != nil {
		return e
	}
	return seedSharing(ctx, tx, tenant)
}

var systemFields = map[string]bool{
	"id": true, "createddate": true, "createdbyid": true, "lastmodifieddate": true,
	"lastmodifiedbyid": true, "systemmodstamp": true, "isdeleted": true,
}

// Permissionable reports whether field-level security can be set on f. It is
// false for Id, required (non-nillable), name, generated and system fields.
// Migration 7 repeats this rule in SQL for existing tenants.
func Permissionable(f Field) bool {
	return f.Nillable && !f.NameField && !f.Options.Generated && !systemFields[strings.ToLower(f.Name)]
}

// Plan 33 seeded profiles. access is "edit" (CRUD on objects, Read and Edit on
// fields), "read" (Read only) or "" (no object or field grants).
var seededProfiles = []struct {
	name                           string
	modifyAll, viewAll, api, users bool
	access                         string
}{
	{"System Administrator", true, true, true, true, ""},
	{"Standard User", false, false, true, false, "edit"},
	{"Read Only", false, false, true, false, "read"},
	{"Minimum Access", false, false, false, false, ""},
}

// seedProfiles gives a new tenant the four profiles, each owning one
// permission set, matching migration 7. Standard User and Read Only cover
// Account and Contact; Standard User has no grant on Account.Compat_Text__c.
func seedProfiles(ctx context.Context, tx pgx.Tx, tenant string) error {
	if _, e := tx.Exec(ctx, "INSERT INTO sc_setup.revisions(tenant_id,revision) VALUES($1,1)", tenant); e != nil {
		return e
	}
	for _, sp := range seededProfiles {
		profile, e := recordid.New("00e")
		if e != nil {
			return e
		}
		set, e := recordid.New("0PS")
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO sc_setup.profiles(id,tenant_id,name,permission_set_id) VALUES($1,$2,$3,$4)", profile, tenant, sp.name, set); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO sc_setup.permission_sets(id,tenant_id,name,label,is_owned_by_profile,profile_id,permissions_modify_all_data,permissions_view_all_data,permissions_api_enabled,permissions_manage_users)
			VALUES($1,$2,$3,$4,true,$5,$6,$7,$8,$9)`, set, tenant, "X"+profile, sp.name, profile, sp.modifyAll, sp.viewAll, sp.api, sp.users); e != nil {
			return e
		}
		if sp.access == "" {
			continue
		}
		edit := sp.access == "edit"
		var ids, objects, fields []string
		var edits []bool
		for _, o := range StandardObjects() {
			if o.Name != "Account" && o.Name != "Contact" {
				continue
			}
			id, e := recordid.New("110")
			if e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, "INSERT INTO sc_setup.object_permissions(id,tenant_id,parent_id,sobject_type,permissions_read,permissions_create,permissions_edit,permissions_delete) VALUES($1,$2,$3,$4,true,$5,$5,$5)", id, tenant, set, o.Name, edit); e != nil {
				return e
			}
			for _, f := range o.Fields {
				if !Permissionable(f) || (edit && o.Name == "Account" && f.Name == "Compat_Text__c") {
					continue
				}
				id, e := recordid.New("01k")
				if e != nil {
					return e
				}
				ids, objects, fields = append(ids, id), append(objects, o.Name), append(fields, o.Name+"."+f.Name)
				edits = append(edits, edit && (f.Createable || f.Updateable))
			}
		}
		if _, e = tx.Exec(ctx, `INSERT INTO sc_setup.field_permissions(id,tenant_id,parent_id,sobject_type,field,permissions_read,permissions_edit)
			SELECT id,$2,$3,o,f,true,ed FROM unnest($1::text[],$4::text[],$5::text[],$6::boolean[]) AS g(id,o,f,ed)`, ids, tenant, set, objects, fields, edits); e != nil {
			return e
		}
	}
	return nil
}
