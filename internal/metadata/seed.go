package metadata

import (
	"context"
	"github.com/jackc/pgx/v5"
	"salescompel/internal/fieldtype"
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

// StandardObjects returns the seeded Account and Contact definitions. Plan 27
// fields keep their order, labels and columns; Plan 28 fields follow.
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
	accountID := Field{Name: "AccountId", Column: "account_id", Nillable: true, Length: 18, Createable: true, Updateable: true, ReferenceTo: []string{"Account"}, RelationshipName: ptr("Account"), Options: Options{Kind: fieldtype.Lookup, DeleteBehavior: "Restrict"}}
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
	return []Object{
		{Name: "Account", Label: "Account", PluralLabel: "Accounts", Standard: true, Schema: "public", Table: "accounts", Prefix: "001", Fields: account},
		{Name: "Contact", Label: "Contact", PluralLabel: "Contacts", Standard: true, Schema: "public", Table: "contacts", Prefix: "003", Fields: contact},
	}
}

func insertField(ctx context.Context, tx pgx.Tx, tenant, obj string, f Field, ordinal int) error {
	f = Complete(f)
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
	return nil
}
