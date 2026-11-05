package policy

import (
	"errors"
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
	"strings"
)

type Principal struct {
	Tenant, User, Role, Username, Scope string
	Catalog                             *metadata.Snapshot `json:"-"`
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
	if e.Principal.Tenant == "" || (e.Principal.Role != "admin" && e.Principal.Role != "restricted") || e.APIVersion != "65.0" || e.PrivilegeMode != "user" || e.SharingMode != "tenant" {
		return errors.New("unsupported execution context")
	}
	return nil
}
func Allowed(p Principal, object, name string) bool {
	return !(p.Role != "admin" && object == "Account" && name == "Compat_Text__c")
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
		if strings.EqualFold(f.Name, name) && Allowed(p, o.Name, f.Name) {
			return f, nil
		}
	}
	return metadata.Field{}, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", name)
}
