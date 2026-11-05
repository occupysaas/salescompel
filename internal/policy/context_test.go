package policy

import "testing"

func TestExecutionContextFailsClosed(t *testing.T) {
	base := ExecutionContext{Principal: Principal{Tenant: "A", Role: "admin"}, APIVersion: "65.0", PrivilegeMode: "user", SharingMode: "tenant"}
	if e := base.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"tenant", "role", "version", "privilege", "sharing"} {
		bad := base
		switch mode {
		case "tenant":
			bad.Principal.Tenant = ""
		case "role":
			bad.Principal.Role = "unknown"
		case "version":
			bad.APIVersion = "99.0"
		case "privilege":
			bad.PrivilegeMode = "system"
		case "sharing":
			bad.SharingMode = "without sharing"
		}
		if bad.Validate() == nil {
			t.Fatal("accepted", mode)
		}
	}
}
