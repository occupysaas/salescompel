package metadata

import (
	"github.com/jackc/pgx/v5"
	"salescompel/internal/contract"
	"salescompel/internal/fieldtype"
	"strings"
)

// PicklistValue is a describe picklist entry. API values are tenant data.
type PicklistValue struct {
	Active       bool    `json:"active"`
	DefaultValue bool    `json:"defaultValue"`
	Label        string  `json:"label"`
	Value        string  `json:"value"`
	ValidFor     *string `json:"validFor"`
}

// Options are candidate-only field parameters, persisted with the definition
// but never part of describe output.
type Options struct {
	Kind           fieldtype.Kind `json:"kind"`
	VisibleLines   int            `json:"visibleLines,omitempty"`
	DeleteBehavior string         `json:"deleteBehavior,omitempty"` // SetNull, Restrict or Cascade
	Reparentable   bool           `json:"reparentable,omitempty"`
	DisplayFormat  string         `json:"displayFormat,omitempty"`
	StartingNumber int64          `json:"startingNumber,omitempty"`
	Components     []string       `json:"components,omitempty"` // compound field parts
	Generated      bool           `json:"generated,omitempty"`  // database-computed column
}

// Field is one field definition. JSON tags are the describe wire form.
type Field struct {
	Name               string          `json:"name"`
	Label              string          `json:"label"`
	Type               string          `json:"type"`
	SoapType           string          `json:"soapType"`
	Nillable           bool            `json:"nillable"`
	Createable         bool            `json:"createable"`
	Updateable         bool            `json:"updateable"`
	ExternalID         bool            `json:"externalId"`
	Length             int             `json:"length"`
	Precision          int             `json:"precision"`
	Scale              int             `json:"scale"`
	Digits             int             `json:"digits"`
	ReferenceTo        []string        `json:"referenceTo"`
	RelationshipName   *string         `json:"relationshipName"`
	RelationshipOrder  *int            `json:"relationshipOrder"`
	CascadeDelete      bool            `json:"cascadeDelete"`
	RestrictedDelete   bool            `json:"restrictedDelete"`
	PicklistValues     []PicklistValue `json:"picklistValues"`
	RestrictedPicklist bool            `json:"restrictedPicklist"`
	DependentPicklist  bool            `json:"dependentPicklist"`
	AutoNumber         bool            `json:"autoNumber"`
	Calculated         bool            `json:"calculated"`
	DefaultValue       any             `json:"defaultValue"`
	DefaultedOnCreate  bool            `json:"defaultedOnCreate"`
	NameField          bool            `json:"nameField"`
	CompoundFieldName  *string         `json:"compoundFieldName"`
	HTMLFormatted      bool            `json:"htmlFormatted"`
	Filterable         bool            `json:"filterable"`
	Sortable           bool            `json:"sortable"`
	Groupable          bool            `json:"groupable"`
	Unique             bool            `json:"unique"`
	Options            Options         `json:"-"`
	Column             string          `json:"-"`
	ID                 string          `json:"-"`
}

// Kind is the field's logical type.
func (f Field) Kind() fieldtype.Kind { return f.Options.Kind }

// NumericBounds returns precision and scale for numeric fields; int-described
// numbers use digits with scale zero.
func (f Field) NumericBounds() (int, int) {
	if f.Type == "int" {
		return f.Digits, 0
	}
	return f.Precision, f.Scale
}

// Compound reports a field with no physical column (computed from components).
func (f Field) Compound() bool { return f.Options.Kind == fieldtype.Address }

// StoredField is a stored-not-enforced definition (never described or served).
type StoredField struct {
	Object string `json:"object"`
	Name   string `json:"name"`
	Label  string `json:"label"`
	Kind   string `json:"type"`
}

type Object struct {
	ID, Tenant, Schema, Table, Prefix string  `json:"-"`
	PluralLabel                       string  `json:"labelPlural"`
	Standard                          bool    `json:"-"`
	Name                              string  `json:"name"`
	Label                             string  `json:"label"`
	Fields                            []Field `json:"fields"`
	Createable                        bool    `json:"createable"`
	Updateable                        bool    `json:"updateable"`
	Deletable                         bool    `json:"deletable"`
	Queryable                         bool    `json:"queryable"`
}

type Snapshot struct {
	Tenant   string
	Revision int64
	Currency string
	Objects  []Object
	Stored   []StoredField
}

func (o Object) TableSQL() string { return pgx.Identifier{o.Schema, o.Table}.Sanitize() }
func (s Snapshot) Object(name string) (Object, error) {
	for _, o := range s.Objects {
		if strings.EqualFold(o.Name, name) {
			return o, nil
		}
	}
	return Object{}, contract.Invalid("INVALID_TYPE", "Unsupported object")
}
func (s Snapshot) Field(object, name string) (Field, error) {
	o, e := s.Object(object)
	if e != nil {
		return Field{}, e
	}
	for _, f := range o.Fields {
		if strings.EqualFold(f.Name, name) {
			return f, nil
		}
	}
	return Field{}, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", name)
}
