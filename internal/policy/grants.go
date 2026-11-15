package policy

import (
	"strings"

	"salescompel/internal/contract"
)

const integrityCode = "FIELD_INTEGRITY_EXCEPTION" // D33-10, D33-11

func dependencyError(dependent, field string, missing []string) error {
	return contract.FieldProblem(integrityCode,
		"Permission "+dependent+" depends on permission(s): "+strings.Join(missing, ", "), field)
}

// ValidateObjectGrant enforces the ObjectPermissions dependencies (D33-10).
func ValidateObjectGrant(a ObjectAccess) error {
	check := func(on bool, label, field string, deps ...struct {
		ok    bool
		label string
	}) error {
		if !on {
			return nil
		}
		var missing []string
		for _, d := range deps {
			if !d.ok {
				missing = append(missing, d.label)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		return dependencyError(label, field, missing)
	}
	type dep = struct {
		ok    bool
		label string
	}
	read, edit, del, all := dep{a.Read, "Read"}, dep{a.Edit, "Edit"}, dep{a.Delete, "Delete"}, dep{a.ViewAll, "View All Records"}
	for _, err := range []error{
		check(a.Create, "Create", "PermissionsCreate", read),
		check(a.Edit, "Edit", "PermissionsEdit", read),
		check(a.Delete, "Delete", "PermissionsDelete", read, edit),
		check(a.ViewAll, "View All Records", "PermissionsViewAllRecords", read),
		check(a.ModifyAll, "Modify All Records", "PermissionsModifyAllRecords", read, edit, del, all),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// ValidateFieldGrant enforces the FieldPermissions dependency (D33-11).
func ValidateFieldGrant(a FieldAccess) error {
	if a.Edit && !a.Read {
		return dependencyError("Edit", "PermissionsEdit", []string{"Read"})
	}
	return nil
}
