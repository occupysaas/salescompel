package metadata

import (
	"strings"

	"salescompel/internal/fieldtype"
)

// SetupSchema holds the Plan 33 setup objects' tables.
const SetupSchema = "sc_setup"

// Picklist values for the user and org settings (docs/compat D33-22 to
// D33-25). They are reconstructed lists, not Salesforce's authoritative ones
// (G33-07); correcting them changes data only.
var (
	TimeZoneSidKeys = strings.Fields(`Pacific/Kiritimati Pacific/Chatham Pacific/Auckland Pacific/Enderbury
Pacific/Tongatapu Pacific/Fiji Asia/Kamchatka Pacific/Norfolk
Pacific/Guadalcanal Australia/Lord_Howe Australia/Sydney
Pacific/Port_Moresby Australia/Adelaide Australia/Darwin Australia/Brisbane
Asia/Seoul Asia/Tokyo Asia/Hong_Kong Asia/Kuala_Lumpur Asia/Manila
Asia/Shanghai Asia/Singapore Asia/Taipei Australia/Perth Asia/Bangkok
Asia/Ho_Chi_Minh Asia/Jakarta Asia/Rangoon Asia/Dhaka Asia/Kathmandu
Asia/Colombo Asia/Kolkata Asia/Karachi Asia/Tashkent Asia/Yekaterinburg
Asia/Kabul Asia/Baku Asia/Dubai Asia/Tbilisi Asia/Yerevan Asia/Tehran
Africa/Nairobi Asia/Baghdad Asia/Kuwait Asia/Riyadh Europe/Minsk
Europe/Moscow Africa/Cairo Africa/Johannesburg Asia/Beirut Asia/Jerusalem
Europe/Athens Europe/Bucharest Europe/Helsinki Europe/Istanbul
Africa/Algiers Europe/Amsterdam Europe/Berlin Europe/Brussels Europe/Paris
Europe/Prague Europe/Rome Europe/Lisbon Africa/Casablanca Atlantic/Canary
Europe/Dublin Europe/London GMT Atlantic/Cape_Verde Atlantic/South_Georgia
America/St_Johns America/Sao_Paulo America/Argentina/Buenos_Aires
America/Santiago Atlantic/Bermuda America/Halifax America/Puerto_Rico
America/Caracas America/Bogota America/Indiana/Indianapolis
America/New_York America/Panama America/Lima America/Mexico_City
America/Chicago America/El_Salvador America/Denver America/Phoenix
America/Los_Angeles America/Tijuana America/Anchorage Pacific/Honolulu
Pacific/Niue Pacific/Pago_Pago`)
	LocaleSidKeys = strings.Fields(`en_US en_GB en_CA en_AU en_IN de de_DE de_CH fr fr_FR fr_CA es es_ES
es_MX it it_IT ja ja_JP ko ko_KR zh_CN zh_TW pt_BR pt_PT nl_NL sv sv_SE da da_DK
fi fi_FI no no_NO ru ru_RU th th_TH`)
	LanguageLocaleKeys = strings.Fields("en_US de es fr it ja sv ko zh_TW zh_CN pt_BR nl_NL da th fi ru es_MX no")
	EmailEncodingKeys  = strings.Fields("UTF-8 ISO-8859-1 Shift_JIS ISO-2022-JP EUC-JP ks_c_5601-1987 Big5 Big5-HKSCS GB2312 x-SJIS_0213")
)

// setupField builds a setup-object field; edit makes it createable and
// updateable.
func setupField(name, label, column string, k fieldtype.Kind, length int, nillable, edit bool) Field {
	f := Field{Name: name, Label: label, Column: column, Nillable: nillable, Createable: edit, Updateable: edit, Length: length, Options: Options{Kind: k}}
	if k == fieldtype.Checkbox {
		f.Nillable, f.DefaultValue = false, false
	}
	return f
}

func setupPicklist(name, label, column string, values []string, edit bool) Field {
	f := setupField(name, label, column, fieldtype.Picklist, 40, false, edit)
	f.PicklistValues, f.RestrictedPicklist = picklist(values...), true
	return f
}

// setupLookup is a reference field; delete is the database's ON DELETE rule.
func setupLookup(name, label, column, target, relationship, child, delete string, nillable, create, update bool) Field {
	f := Field{Name: name, Label: label, Column: column, Nillable: nillable, Createable: create, Updateable: update, Length: 18,
		ReferenceTo: []string{target}, RelationshipName: ptr(relationship),
		Options: Options{Kind: fieldtype.Lookup, DeleteBehavior: delete, ChildRelationshipName: child}}
	return f
}

// systemPermissionFields are the four enforced system permissions (D33-18).
func systemPermissionFields(edit bool) []Field {
	return []Field{
		setupField("PermissionsModifyAllData", "Modify All Data", "permissions_modify_all_data", fieldtype.Checkbox, 0, false, edit),
		setupField("PermissionsViewAllData", "View All Data", "permissions_view_all_data", fieldtype.Checkbox, 0, false, edit),
		setupField("PermissionsApiEnabled", "API Enabled", "permissions_api_enabled", fieldtype.Checkbox, 0, false, edit),
		setupField("PermissionsManageUsers", "Manage Users", "permissions_manage_users", fieldtype.Checkbox, 0, false, edit),
	}
}

func objectPermissionFields() []Field {
	return []Field{
		setupField("PermissionsRead", "Read", "permissions_read", fieldtype.Checkbox, 0, false, true),
		setupField("PermissionsCreate", "Create", "permissions_create", fieldtype.Checkbox, 0, false, true),
		setupField("PermissionsEdit", "Edit", "permissions_edit", fieldtype.Checkbox, 0, false, true),
		setupField("PermissionsDelete", "Delete", "permissions_delete", fieldtype.Checkbox, 0, false, true),
		setupField("PermissionsViewAllRecords", "View All Records", "permissions_view_all_records", fieldtype.Checkbox, 0, false, true),
		setupField("PermissionsModifyAllRecords", "Modify All Records", "permissions_modify_all_records", fieldtype.Checkbox, 0, false, true),
	}
}

// SetupObjects returns the seven Plan 33 setup objects, stored in sc_setup.
// Access follows the spec's Objects table; setup objects never receive
// ObjectPermissions or FieldPermissions grants (D33-32).
//
// Organization is a view over the tenant settings. Profile's Permissions*
// fields live on the profile-owned permission set.
func SetupObjects() []Object {
	id := Field{Name: "Id", Column: "id", Options: Options{Kind: fieldtype.Id}}
	named := func(length int, edit bool) Field {
		f := setupField("Name", "Name", "name", fieldtype.Text, length, false, edit)
		f.NameField = true
		return f
	}
	organization := []Field{id, named(80, false),
		setupPicklist("TimeZoneSidKey", "Default Time Zone", "time_zone_sid_key", TimeZoneSidKeys, false),
		setupPicklist("DefaultLocaleSidKey", "Default Locale", "default_locale_sid_key", LocaleSidKeys, false),
		setupPicklist("LanguageLocaleKey", "Default Language", "language_locale_key", LanguageLocaleKeys, false),
		setupField("DefaultCurrencyIsoCode", "Currency ISO Code", "currency_iso_code", fieldtype.Text, 3, false, false),
		setupField("InstanceName", "Instance Name", "instance_name", fieldtype.Text, 5, true, false),
		setupField("IsSandbox", "Is Sandbox", "is_sandbox", fieldtype.Checkbox, 0, false, false),
		setupField("TrialExpirationDate", "Trial Expiration Date", "trial_expiration_date", fieldtype.DateTime, 0, true, false),
		setupField("NamespacePrefix", "Namespace Prefix", "namespace_prefix", fieldtype.Text, 15, true, false),
		setupField("OrganizationType", "Edition", "organization_type", fieldtype.Text, 40, false, false),
	}
	first := setupField("FirstName", "First Name", "first_name", fieldtype.Text, 40, true, true)
	last := setupField("LastName", "Last Name", "last_name", fieldtype.Text, 80, false, true)
	first.CompoundFieldName, last.CompoundFieldName = ptr("Name"), ptr("Name")
	fullName := Field{Name: "Name", Label: "Full Name", Column: "name", Length: 121, NameField: true, Options: Options{Kind: fieldtype.PersonName, Generated: true, Components: []string{"FirstName", "LastName"}}}
	user := []Field{id,
		setupField("Username", "Username", "username", fieldtype.Text, 80, false, true),
		first, last, fullName,
		setupField("Alias", "Alias", "alias", fieldtype.Text, 8, false, true),
		setupField("Email", "Email", "email", fieldtype.Email, 128, false, true),
		setupField("IsActive", "Active", "is_active", fieldtype.Checkbox, 0, false, true),
		setupLookup("ProfileId", "Profile ID", "profile_id", "Profile", "Profile", "Users", "Restrict", false, true, true),
		setupPicklist("TimeZoneSidKey", "Time Zone", "time_zone_sid_key", TimeZoneSidKeys, true),
		setupPicklist("LocaleSidKey", "Locale", "locale_sid_key", LocaleSidKeys, true),
		setupPicklist("LanguageLocaleKey", "Language", "language_locale_key", LanguageLocaleKeys, true),
		setupPicklist("EmailEncodingKey", "Email Encoding", "email_encoding_key", EmailEncodingKeys, true),
	}
	user[7].DefaultValue = true // IsActive defaults to true, as the column does
	profile := append([]Field{id, named(255, false),
		setupField("Description", "Description", "description", fieldtype.Text, 255, true, false),
	}, systemPermissionFields(false)...)
	permissionSet := append([]Field{id, named(80, true),
		setupField("Label", "Label", "label", fieldtype.Text, 80, false, true),
		setupField("Description", "Description", "description", fieldtype.Text, 255, true, true),
		setupField("IsOwnedByProfile", "Is Owned By Profile", "is_owned_by_profile", fieldtype.Checkbox, 0, false, false),
		setupLookup("ProfileId", "Profile ID", "profile_id", "Profile", "Profile", "PermissionSets", "Cascade", true, false, false),
	}, systemPermissionFields(true)...)
	objectPerms := append([]Field{id,
		setupLookup("ParentId", "Parent ID", "parent_id", "PermissionSet", "Parent", "ObjectPerms", "Cascade", false, true, false),
		setupField("SobjectType", "SObject Type Name", "sobject_type", fieldtype.Text, 255, false, true),
	}, objectPermissionFields()...)
	objectPerms[2].Updateable = false
	fieldPerms := []Field{id,
		setupLookup("ParentId", "Parent ID", "parent_id", "PermissionSet", "Parent", "FieldPerms", "Cascade", false, true, false),
		setupField("SobjectType", "SObject Type Name", "sobject_type", fieldtype.Text, 255, false, false),
		setupField("Field", "Field Name", "field", fieldtype.Text, 255, false, false),
		setupField("PermissionsRead", "Read Access", "permissions_read", fieldtype.Checkbox, 0, false, true),
		setupField("PermissionsEdit", "Edit Access", "permissions_edit", fieldtype.Checkbox, 0, false, true),
	}
	fieldPerms[2].Createable, fieldPerms[3].Createable = true, true
	assignment := []Field{id,
		setupLookup("AssigneeId", "Assignee ID", "assignee_id", "User", "Assignee", "PermissionSetAssignments", "Cascade", false, true, false),
		setupLookup("PermissionSetId", "Permission Set ID", "permission_set_id", "PermissionSet", "PermissionSet", "Assignments", "Cascade", false, true, false),
	}
	objects := []Object{
		{Name: "Organization", Label: "Organization", PluralLabel: "Organizations", Table: "organization", Prefix: "00D", Fields: organization},
		{Name: "User", Label: "User", PluralLabel: "Users", Table: "users", Prefix: "005", Fields: user, Createable: true, Updateable: true},
		{Name: "Profile", Label: "Profile", PluralLabel: "Profiles", Table: "profiles", Prefix: "00e", Fields: profile},
		{Name: "PermissionSet", Label: "Permission Set", PluralLabel: "Permission Sets", Table: "permission_sets", Prefix: "0PS", Fields: permissionSet, Createable: true, Updateable: true, Deletable: true},
		{Name: "ObjectPermissions", Label: "Object Permissions", PluralLabel: "Object Permissions", Table: "object_permissions", Prefix: "110", Fields: objectPerms, Createable: true, Updateable: true, Deletable: true},
		{Name: "FieldPermissions", Label: "Field Permissions", PluralLabel: "Field Permissions", Table: "field_permissions", Prefix: "01k", Fields: fieldPerms, Createable: true, Updateable: true, Deletable: true},
		{Name: "PermissionSetAssignment", Label: "Permission Set Assignment", PluralLabel: "Permission Set Assignments", Table: "permission_set_assignments", Prefix: "0Pa", Fields: assignment, Createable: true, Deletable: true},
	}
	for i := range objects {
		objects[i].Standard, objects[i].Schema, objects[i].Queryable = true, SetupSchema, true
		for j := range objects[i].Fields {
			objects[i].Fields[j] = Complete(objects[i].Fields[j])
		}
	}
	return objects
}

// setupFlags holds each setup object's API access, which the catalog tables
// don't store; Load applies it.
var setupFlags = func() map[string]Object {
	out := map[string]Object{}
	for _, o := range SetupObjects() {
		out[strings.ToLower(o.Name)] = Object{Createable: o.Createable, Updateable: o.Updateable, Deletable: o.Deletable, Queryable: o.Queryable}
	}
	return out
}()

// IsSetupObject reports whether name is one of the seven setup objects.
func IsSetupObject(name string) bool {
	_, ok := setupFlags[strings.ToLower(name)]
	return ok
}
