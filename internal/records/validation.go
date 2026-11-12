package records

import (
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"sort"
	"strings"
	"unicode/utf8"
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
		if v == nil {
			if !f.Nillable {
				return fieldProblem("REQUIRED_FIELD_MISSING", "Required value is missing", f.Name)
			}
			continue
		}
		s, ok := v.(string)
		if !ok {
			return fieldProblem("JSON_PARSER_ERROR", "Expected a string or null", f.Name)
		}
		if !f.Nillable && strings.TrimSpace(s) == "" {
			return fieldProblem("REQUIRED_FIELD_MISSING", "Required value is missing", f.Name)
		}
		if !utf8.ValidString(s) || strings.ContainsRune(s, 0) || utf8.RuneCountInString(s) > f.Length {
			return fieldProblem("STRING_TOO_LONG", "Invalid or oversized field value", f.Name)
		}
	}
	if create {
		for _, f := range fieldsFor(p, object) {
			if f.Name != "Id" && !f.Nillable && f.Createable {
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
