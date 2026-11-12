// Package fieldtype owns the value rules for every field type: parsing,
// rounding, validation and wire formatting. It has no database or metadata
// dependency. Numeric values never pass through binary floating point.
package fieldtype

import (
	"errors"
	"fmt"
)

// Kind is the logical field type stored in metadata.
type Kind string

const (
	Id                  Kind = "Id"
	Text                Kind = "Text"
	TextArea            Kind = "TextArea"
	LongTextArea        Kind = "LongTextArea"
	Number              Kind = "Number"
	Currency            Kind = "Currency"
	Percent             Kind = "Percent"
	Date                Kind = "Date"
	DateTime            Kind = "DateTime"
	Checkbox            Kind = "Checkbox"
	Picklist            Kind = "Picklist"
	MultiselectPicklist Kind = "MultiselectPicklist"
	Email               Kind = "Email"
	Phone               Kind = "Phone"
	Url                 Kind = "Url"
	Lookup              Kind = "Lookup"
	MasterDetail        Kind = "MasterDetail"
	AutoNumber          Kind = "AutoNumber"
	Address             Kind = "Address"
	PersonName          Kind = "PersonName"

	// Stored, not enforced: definitions are retained but never served.
	Formula       Kind = "Formula"
	RollupSummary Kind = "RollupSummary"
	EncryptedText Kind = "EncryptedText"
	Geolocation   Kind = "Geolocation"
	RichTextArea  Kind = "RichTextArea"
)

var (
	ErrSyntax   = errors.New("invalid value")
	ErrRange    = errors.New("value out of range")
	ErrEmail    = errors.New("invalid email address")
	ErrPicklist = errors.New("value not in restricted picklist")
)

type info struct{ describe, soap string }

var active = map[Kind]info{
	Id: {"id", "tns:ID"}, Text: {"string", "xsd:string"}, TextArea: {"textarea", "xsd:string"}, LongTextArea: {"textarea", "xsd:string"},
	Number: {"double", "xsd:double"}, Currency: {"currency", "xsd:double"}, Percent: {"percent", "xsd:double"},
	Date: {"date", "xsd:date"}, DateTime: {"datetime", "xsd:dateTime"}, Checkbox: {"boolean", "xsd:boolean"},
	Picklist: {"picklist", "xsd:string"}, MultiselectPicklist: {"multipicklist", "xsd:string"},
	Email: {"email", "xsd:string"}, Phone: {"phone", "xsd:string"}, Url: {"url", "xsd:string"},
	Lookup: {"reference", "tns:ID"}, MasterDetail: {"reference", "tns:ID"}, AutoNumber: {"string", "xsd:string"},
	Address: {"address", "urn:address"}, PersonName: {"string", "xsd:string"},
}

var storedOnly = map[Kind]bool{Formula: true, RollupSummary: true, EncryptedText: true, Geolocation: true, RichTextArea: true}

// Active reports whether the candidate runs this type.
func (k Kind) Active() bool { _, ok := active[k]; return ok }

// Stored reports whether the type is retained as stored, not enforced.
func (k Kind) Stored() bool { return storedOnly[k] }

func (k Kind) DescribeType() string { return active[k].describe }
func (k Kind) SoapType() string     { return active[k].soap }

// Numeric reports whether values are decimals with precision and scale.
func (k Kind) Numeric() bool { return k == Number || k == Currency || k == Percent }

// Reference reports whether values are record IDs of another object.
func (k Kind) Reference() bool { return k == Lookup || k == MasterDetail }

func (k Kind) Filterable() bool {
	return k.Active() && k != LongTextArea && k != MultiselectPicklist && k != Address
}
func (k Kind) Sortable() bool { return k.Filterable() }

// ParseKind resolves a schema CLI or definition type name. System kinds
// (Id, Address, PersonName) are seeded only and cannot be created.
func ParseKind(name string) (Kind, error) {
	k := Kind(name)
	if (k.Active() && k != Id && k != Address && k != PersonName) || k.Stored() {
		return k, nil
	}
	return "", fmt.Errorf("unsupported field type %q", name)
}
