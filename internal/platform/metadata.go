package platform

// OptionalBool distinguishes an observed false value from an unobserved property.
// Value has no meaning unless Known is true.
type OptionalBool struct {
	Known bool
	Value bool
}

// CustomFieldDeclaration describes captured Metadata API properties, not the
// active storage/describe catalog or its enforcement behavior.
type CustomFieldDeclaration struct {
	FullName      string
	Type          string
	Length        int
	Required      bool
	ExternalID    bool
	Unique        bool
	CaseSensitive OptionalBool
}

// FieldGrantDeclaration is a permission-set declaration, not a principal's
// effective access. It implies neither assignment nor object/record access.
type FieldGrantDeclaration struct {
	PermissionSet string
	Field         string
	Readable      bool
	Editable      bool
}

// DeclaredCustomField returns observed properties for an exact synthetic full
// name. A false result means unknown, not absent. Values come from Plan 6's
// matching read/retrieve observations; see docs/compat/plan-7-declared-metadata.md.
// Returned values have no shared mutable state.
func DeclaredCustomField(fullName string) (CustomFieldDeclaration, bool) {
	switch fullName {
	case "Account.Compat_Text__c":
		return CustomFieldDeclaration{
			FullName: fullName, Type: "Text", Length: 80,
			Required: false, ExternalID: false, Unique: false,
		}, true
	case "Account.Compat_Key__c":
		return CustomFieldDeclaration{
			FullName: fullName, Type: "Text", Length: 80,
			Required: false, ExternalID: true, Unique: true,
			CaseSensitive: OptionalBool{Known: true, Value: false},
		}, true
	default:
		return CustomFieldDeclaration{}, false
	}
}

// DeclaredFieldGrant returns only captured declarations. Unknown permission sets
// or fields return false; callers must not interpret that as a denied grant.
// This function does not evaluate principals, assignments, or effective access.
func DeclaredFieldGrant(permissionSet, field string) (FieldGrantDeclaration, bool) {
	if permissionSet == "Compat_Admin" {
		switch field {
		case "Account.Compat_Text__c", "Account.Compat_Key__c":
			return FieldGrantDeclaration{
				PermissionSet: permissionSet, Field: field, Readable: true, Editable: true,
			}, true
		}
	}
	return FieldGrantDeclaration{}, false
}
