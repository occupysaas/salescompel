package policy

import "testing"

func TestExecutionContextFailsClosed(t *testing.T) {
	base := ExecutionContext{Principal: Principal{Tenant: "A", Role: "admin"}, APIVersion: "65.0", PrivilegeMode: "user", SharingMode: "tenant"}
	if e := base.Validate(); e != nil {
		t.Fatal(e)
	}
	// Role no longer decides anything: an empty role is valid (OAuth users).
	if e := (ExecutionContext{Principal: Principal{Tenant: "A"}, APIVersion: "65.0", PrivilegeMode: "user", SharingMode: "tenant"}).Validate(); e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"tenant", "version", "privilege", "sharing"} {
		bad := base
		switch mode {
		case "tenant":
			bad.Principal.Tenant = ""
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
