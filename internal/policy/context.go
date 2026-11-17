package policy

import (
	"errors"
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
	"strings"
	"time"
)

// Principal is the caller of one request. Perms, Location and ProfileID are
// loaded from the setup tables when the token is checked. Role is set only
// for provisioned static tokens, whose role maps to a seeded profile; the
// runtime decides nothing from it directly.
type Principal struct {
	Tenant, User, Role, Username, Scope string
	ProfileID                           string
	Perms                               Permissions        `json:"-"`
	Location                            *time.Location     `json:"-"`
	Catalog                             *metadata.Snapshot `json:"-"`
	// Sharing holds the org-wide defaults (Plan 34); nil means defaults.
	Sharing *Sharing `json:"-"`
}
type ExecutionContext struct {
	Principal                              Principal
	APIVersion, PrivilegeMode, SharingMode string
}

func ForAPI(p Principal, version string) (ExecutionContext, error) {
	e := ExecutionContext{p, version, "user", "tenant"}
	return e, e.Validate()
}
func (e ExecutionContext) Validate() error {
	if e.Principal.Tenant == "" || e.APIVersion != "65.0" || e.PrivilegeMode != "user" || e.SharingMode != "tenant" {
		return errors.New("unsupported execution context")
	}
	return nil
}

// Allowed reports whether p can read field f of object.
func Allowed(p Principal, object string, f metadata.Field) bool {
	return p.Perms.FieldRead(object, f)
}
func Field(p Principal, object, name string) (metadata.Field, error) {
	if p.Catalog == nil {
		return metadata.Field{}, contract.Invalid("INVALID_TYPE", "Tenant catalog is unavailable")
	}
	o, e := p.Catalog.Object(object)
	if e != nil {
		return metadata.Field{}, e
	}
	for _, f := range o.Fields {
		if strings.EqualFold(f.Name, name) && Allowed(p, o.Name, f) {
			return f, nil
		}
	}
	return metadata.Field{}, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", name)
}
