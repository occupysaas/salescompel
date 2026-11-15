// Package policytest builds effective permissions for unit tests that run
// without a database. Database-backed code loads them with policy.LoadPermissions.
package policytest

import (
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

// Admin is the System Administrator profile's effective permissions.
func Admin() policy.Permissions {
	return policy.Union(policy.PermissionSet{System: policy.SystemPerms{ModifyAllData: true, ViewAllData: true, APIEnabled: true, ManageUsers: true}})
}

// StandardUser mirrors the seeded Standard User profile over objects: CRUD on
// every non-setup object, Read and Edit on every permissionable field, and no
// grant on Account.Compat_Text__c.
func StandardUser(objects []metadata.Object) policy.Permissions {
	s := policy.PermissionSet{System: policy.SystemPerms{APIEnabled: true}, Objects: map[string]policy.ObjectAccess{}, Fields: map[string]policy.FieldAccess{}}
	for _, o := range objects {
		if metadata.IsSetupObject(o.Name) {
			continue
		}
		s.Objects[o.Name] = policy.ObjectAccess{Read: true, Create: true, Edit: true, Delete: true}
		for _, f := range o.Fields {
			if metadata.Permissionable(f) && !(o.Name == "Account" && f.Name == "Compat_Text__c") {
				s.Fields[o.Name+"."+f.Name] = policy.FieldAccess{Read: true, Edit: f.Createable || f.Updateable}
			}
		}
	}
	return policy.Union(s)
}
