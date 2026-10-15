// Package platform implements the explicitly bounded M2 candidate contract.
package platform

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"
)

type Principal struct{ Tenant, User, Role string }
type APIError struct {
	Status  int      `json:"-"`
	Code    string   `json:"errorCode"`
	Message string   `json:"message"`
	Fields  []string `json:"fields,omitempty"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }
func problem(status int, code, message string) error {
	return &APIError{Status: status, Code: code, Message: message}
}
func invalid(code, message string) error { return problem(http.StatusBadRequest, code, message) }

type Field struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Nillable    bool     `json:"nillable"`
	Createable  bool     `json:"createable"`
	Updateable  bool     `json:"updateable"`
	ExternalID  bool     `json:"externalId"`
	Length      int      `json:"length"`
	ReferenceTo []string `json:"referenceTo"`
	Column      string   `json:"-"`
}
type Object struct {
	Name       string  `json:"name"`
	Label      string  `json:"label"`
	Fields     []Field `json:"fields"`
	Createable bool    `json:"createable"`
	Updateable bool    `json:"updateable"`
	Deletable  bool    `json:"deletable"`
	Queryable  bool    `json:"queryable"`
}

var catalog = map[string][]Field{
	"Account": {{Name: "Id", Type: "id", Column: "id"}, {Name: "Name", Type: "string", Column: "name", Length: 255}, {Name: "Compat_Text__c", Type: "string", Column: "compat_text", Nillable: true, Length: 255}, {Name: "Compat_Key__c", Type: "string", Column: "compat_key", Nillable: true, ExternalID: true, Length: 255}},
	"Contact": {{Name: "Id", Type: "id", Column: "id"}, {Name: "FirstName", Type: "string", Column: "first_name", Nillable: true, Length: 255}, {Name: "LastName", Type: "string", Column: "last_name", Length: 255}, {Name: "AccountId", Type: "reference", Column: "account_id", Nillable: true, Length: 18, ReferenceTo: []string{"Account"}}},
}

func allowed(p Principal, object, name string) bool {
	return !(p.Role != "admin" && object == "Account" && name == "Compat_Text__c")
}
func field(p Principal, object, name string) (Field, error) {
	fields, ok := catalog[object]
	if !ok {
		return Field{}, invalid("INVALID_TYPE", "Unsupported object")
	}
	for _, f := range fields {
		if f.Name == name && allowed(p, object, name) {
			return f, nil
		}
	}
	return Field{}, invalid("INVALID_FIELD", fmt.Sprintf("Field %s is unavailable", name))
}
func Describe(p Principal, object string) (Object, error) {
	fields, ok := catalog[object]
	if !ok {
		return Object{}, invalid("INVALID_TYPE", "Unsupported object")
	}
	d := Object{Name: object, Label: object, Fields: []Field{}, Createable: true, Updateable: true, Deletable: true, Queryable: true}
	for _, f := range fields {
		if allowed(p, object, f.Name) {
			f.Label = f.Name
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
	if _, ok := catalog[object]; !ok {
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
		if k == "Id" {
			return invalid("INVALID_FIELD_FOR_INSERT_UPDATE", "Id is read only")
		}
		if v == nil {
			if !f.Nillable {
				return invalid("REQUIRED_FIELD_MISSING", k+" is required")
			}
			continue
		}
		s, ok := v.(string)
		if !ok {
			return invalid("JSON_PARSER_ERROR", k+" requires a string or null")
		}
		if !f.Nillable && strings.TrimSpace(s) == "" {
			return invalid("REQUIRED_FIELD_MISSING", k+" is required")
		}
		if !utf8.ValidString(s) || strings.ContainsRune(s, 0) || utf8.RuneCountInString(s) > f.Length {
			return invalid("STRING_TOO_LONG", "Invalid or oversized field value")
		}
	}
	if create {
		for _, f := range catalog[object] {
			if f.Name != "Id" && !f.Nillable {
				if v, ok := data[f.Name]; !ok || v == nil {
					return invalid("REQUIRED_FIELD_MISSING", f.Name+" is required")
				}
			}
		}
	}
	return nil
}
