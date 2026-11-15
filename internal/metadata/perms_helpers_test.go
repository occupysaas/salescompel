package metadata_test

import (
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/policy/policytest"
)

// adminPerms and standardPerms stand in for the seeded profiles' loaded
// permissions in database-free describe tests.
func adminPerms() policy.Permissions { return policytest.Admin() }
func standardPerms(objects []metadata.Object) policy.Permissions {
	return policytest.StandardUser(objects)
}
