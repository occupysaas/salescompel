package metadata

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/fieldtype"
	"salescompel/internal/recordid"
)

// Plan 34 sharing catalog: the OwnerId field on every record object, the
// UserRole, Group and GroupMember setup objects, and the AccountShare and
// ContactShare share objects (docs/compat/plan-34-sharing.md D34-06, D34-12,
// D34-13, D34-15). Migration 8 repeats these definitions for existing tenants;
// TestCatalogSeedParity keeps them in step.

// Internal org-wide default values (D34-16).
const (
	AccessNone               = "None"
	AccessRead               = "Read"
	AccessEdit               = "Edit"
	AccessControlledByParent = "ControlledByParent"
)

var (
	accessLevels      = []string{"Read", "Edit", "All"}
	childAccessLevels = []string{"None", "Read", "Edit"}
	// GroupTypes are the group types Plan 34 models; only Regular is
	// writable over the API (D34-12).
	GroupTypes = []string{"Regular", "Role", "RoleAndSubordinates", "RoleAndSubordinatesInternal", "Organization"}
	rowCauses  = []string{"Owner", "Manual", "Rule", "ImplicitChild", "ImplicitParent"}
)

// ownerColumnSQL is the owner column on every record table. It is nullable
// in the database because a tenant with records and no users has no owner
// to backfill (migration 8); the write path always sets it.
const ownerColumnSQL = "owner_id varchar(18)"

// OwnerField is the OwnerId field every record object carries. It is
// defaulted on create to the running user (shr-01).
func OwnerField() Field {
	f := Complete(Field{Name: "OwnerId", Label: "Owner ID", Column: "owner_id", Length: 18, Createable: true, Updateable: true,
		ReferenceTo: []string{"User"}, RelationshipName: ptr("Owner"),
		Options: Options{Kind: fieldtype.Lookup, DeleteBehavior: "Restrict"}})
	f.DefaultedOnCreate = true
	return f
}

// IsShareObject reports whether name is a share object (AccountShare,
// ContactShare).
func IsShareObject(name string) bool {
	_, ok := ShareParent(name)
	return ok
}

// ShareParent returns the record object a share object grants access to.
func ShareParent(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "accountshare":
		return "Account", true
	case "contactshare":
		return "Contact", true
	}
	return "", false
}

// IsSharingSetupObject reports whether name is UserRole, Group or GroupMember.
func IsSharingSetupObject(name string) bool {
	switch strings.ToLower(name) {
	case "userrole", "group", "groupmember":
		return true
	}
	return false
}

// sharingObjects returns the Plan 34 setup objects. Their completion and
// Standard/Schema/Queryable flags are applied by SetupObjects.
func sharingObjects() []Object {
	id := Field{Name: "Id", Column: "id", Options: Options{Kind: fieldtype.Id}}
	named := func(length int) Field {
		f := setupField("Name", "Name", "name", fieldtype.Text, length, false, true)
		f.NameField = true
		return f
	}
	childAccess := func(name, label, column string) Field {
		f := setupPicklist(name, label, column, childAccessLevels, true)
		f.Nillable = true
		return f
	}
	role := []Field{id, named(80),
		setupField("DeveloperName", "Developer Name", "developer_name", fieldtype.Text, 80, false, true),
		setupLookup("ParentRoleId", "Parent Role ID", "parent_role_id", "UserRole", "ParentRole", "ChildRoles", "Restrict", true, true, true),
		setupField("RollupDescription", "Description", "rollup_description", fieldtype.Text, 80, true, true),
		childAccess("ContactAccessForAccountOwner", "Contact Access Level for Account Owner", "contact_access_for_account_owner"),
		childAccess("OpportunityAccessForAccountOwner", "Opportunity Access Level for Account Owner", "opportunity_access_for_account_owner"),
		childAccess("CaseAccessForAccountOwner", "Case Access Level for Account Owner", "case_access_for_account_owner"),
	}
	groupType := setupPicklist("Type", "Type", "type", GroupTypes, false)
	groupType.Createable = true
	group := []Field{id, named(40),
		setupField("DeveloperName", "Developer Name", "developer_name", fieldtype.Text, 80, true, true),
		groupType,
		setupField("RelatedId", "Related ID", "related_id", fieldtype.Text, 18, true, false),
		setupField("DoesIncludeBosses", "Include Bosses", "does_include_bosses", fieldtype.Checkbox, 0, false, true),
	}
	member := []Field{id,
		setupLookup("GroupId", "Group ID", "group_id", "Group", "Group", "GroupMembers", "Cascade", false, true, false),
		setupField("UserOrGroupId", "User/Group ID", "user_or_group_id", fieldtype.Text, 18, false, false),
	}
	member[2].Createable = true
	shareCommon := func(parentField, parentLabel, parentColumn, parentObject, levelField, levelLabel, levelColumn string) []Field {
		level := setupPicklist(levelField, levelLabel, levelColumn, accessLevels, true)
		cause := setupPicklist("RowCause", "Row Cause", "row_cause", rowCauses, false)
		cause.Createable = true
		parent := setupLookup(parentField, parentLabel, parentColumn, parentObject, parentObject, "Shares", "Cascade", false, true, false)
		grantee := setupField("UserOrGroupId", "User/Group ID", "user_or_group_id", fieldtype.Text, 18, false, false)
		grantee.Createable = true
		return []Field{id, parent, grantee, level, cause}
	}
	accountShare := append(shareCommon("AccountId", "Account ID", "account_id", "Account", "AccountAccessLevel", "Account Access", "account_access_level"),
		childAccess("ContactAccessLevel", "Contact Access", "contact_access_level"),
		childAccess("OpportunityAccessLevel", "Opportunity Access", "opportunity_access_level"),
		childAccess("CaseAccessLevel", "Case Access", "case_access_level"))
	contactShare := shareCommon("ContactId", "Contact ID", "contact_id", "Contact", "ContactAccessLevel", "Contact Access", "contact_access_level")
	return []Object{
		{Name: "UserRole", Label: "Role", PluralLabel: "Roles", Table: "user_roles", Prefix: "00E", Fields: role, Createable: true, Updateable: true, Deletable: true},
		{Name: "Group", Label: "Group", PluralLabel: "Groups", Table: "groups", Prefix: "00G", Fields: group, Createable: true, Updateable: true, Deletable: true},
		{Name: "GroupMember", Label: "Group Member", PluralLabel: "Group Members", Table: "group_members", Prefix: "011", Fields: member, Createable: true, Deletable: true},
		{Name: "AccountShare", Label: "Account Share", PluralLabel: "Account Share", Table: "account_shares", Prefix: "00r", Fields: accountShare, Createable: true, Updateable: true, Deletable: true},
		{Name: "ContactShare", Label: "Contact Share", PluralLabel: "Contact Share", Table: "contact_shares", Prefix: "03s", Fields: contactShare, Createable: true, Updateable: true, Deletable: true},
	}
}

// userRoleField and orgAccessFields extend User and Organization (D34-15,
// D34-20).
func userRoleField() Field {
	return setupLookup("UserRoleId", "Role ID", "user_role_id", "UserRole", "UserRole", "Users", "Restrict", true, true, true)
}

func orgAccessFields() []Field {
	values := []string{AccessNone, AccessRead, AccessEdit, AccessControlledByParent}
	return []Field{
		setupPicklist("DefaultAccountAccess", "Default Account Access", "default_account_access", values, false),
		setupPicklist("DefaultContactAccess", "Default Contact Access", "default_contact_access", values, false),
	}
}

// seedSharing gives a new tenant its Organization group ("All Internal
// Users"), matching migration 8. Users join it through the closure rebuild
// that runs whenever a user is created.
func seedSharing(ctx context.Context, tx pgx.Tx, tenant string) error {
	id, e := recordid.New("00G")
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "INSERT INTO sc_setup.groups(id,tenant_id,name,type) VALUES($1,$2,'All Internal Users','Organization')", id, tenant)
	return e
}
