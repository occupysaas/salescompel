package records

import (
	"salescompel/internal/contract"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"sort"
)

type Principal = policy.Principal
type Field = metadata.Field
type Object = metadata.Object
type APIError = contract.APIError

var invalid = contract.Invalid
var problem = contract.Problem
var fieldProblem = contract.FieldProblem

func allowed(p Principal, object, name string) bool {
	return !(p.Role != "admin" && object == "Account" && name == "Compat_Text__c")
}
func field(p Principal, object, name string) (Field, error) { return policy.Field(p, object, name) }
func Describe(p Principal, object string) (Object, error) {
	fields, ok := catalogFields(p, object)
	if !ok {
		return Object{}, invalid("INVALID_TYPE", "Unsupported object")
	}
	d, _ := p.Catalog.Object(object)
	d.Fields = []Field{}
	for _, f := range fields {
		if allowed(p, d.Name, f.Name) {
			if f.Label == "" {
				f.Label = f.Name
			}
			f.Createable = f.Name != "Id"
			f.Updateable = f.Createable
			if f.ReferenceTo == nil {
				f.ReferenceTo = []string{}
			}
			d.Fields = append(d.Fields, f)
		}
	}
	return d, nil
}
func Validate(p Principal, object string, data map[string]any, create bool) error {
	if _, ok := catalogFields(p, object); !ok {
		return invalid("INVALID_TYPE", "Unsupported object")
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := data[k]
		f, err := field(p, object, k)
		if err != nil {
			return err
		}
		if f.Name == "Id" {
			return fieldProblem("INVALID_FIELD_FOR_INSERT_UPDATE", "Id is read only", "Id")
		}
		if (create && !f.Createable) || (!create && !f.Updateable) {
			return fieldProblem("INVALID_FIELD_FOR_INSERT_UPDATE", "Field is not writable", f.Name)
		}
		if v == nil {
			if !f.Nillable && f.Kind() != fieldtype.Checkbox {
				return fieldProblem("REQUIRED_FIELD_MISSING", "Required value is missing", f.Name)
			}
			continue
		}
		if _, _, err := coerce(f, v); err != nil {
			return err
		}
	}
	if create {
		for _, f := range fieldsFor(p, object) {
			if f.Name != "Id" && !f.Nillable && f.Createable && f.Kind() != fieldtype.Checkbox {
				if v, ok := data[f.Name]; !ok || v == nil {
					return fieldProblem("REQUIRED_FIELD_MISSING", "Required value is missing", f.Name)
				}
			}
		}
	}
	return nil
}

func catalogFields(p Principal, object string) ([]Field, bool) {
	if p.Catalog != nil {
		o, e := p.Catalog.Object(object)
		return o.Fields, e == nil
	}
	return nil, false
}
func fieldsFor(p Principal, object string) []Field { f, _ := catalogFields(p, object); return f }
func canonicalObject(p Principal, input string) (string, bool) {
	if p.Catalog != nil {
		o, e := p.Catalog.Object(input)
		return o.Name, e == nil
	}
	return "", false
}
