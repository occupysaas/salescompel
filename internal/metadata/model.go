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
	Kind                  fieldtype.Kind `json:"kind"`
	VisibleLines          int            `json:"visibleLines,omitempty"`
	DeleteBehavior        string         `json:"deleteBehavior,omitempty"` // SetNull, Restrict or Cascade
	Reparentable          bool           `json:"reparentable,omitempty"`
	DisplayFormat         string         `json:"displayFormat,omitempty"`
	StartingNumber        int64          `json:"startingNumber,omitempty"`
	Components            []string       `json:"components,omitempty"`            // compound field parts
	Generated             bool           `json:"generated,omitempty"`             // database-computed column
	ChildRelationshipName string         `json:"childRelationshipName,omitempty"` // parent-side name for this reference
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

// ChildRelationship is one parent-to-child relationship, surfaced in describe.
type ChildRelationship struct {
	ChildSObject        string `json:"childSObject"`
	Field               string `json:"field"`
	RelationshipName    string `json:"relationshipName"`
	CascadeDelete       bool   `json:"cascadeDelete"`
	RestrictedDelete    bool   `json:"restrictedDelete"`
	DeprecatedAndHidden bool   `json:"deprecatedAndHidden"`
}

type Object struct {
	ID, Tenant, Schema, Table, Prefix string              `json:"-"`
	PluralLabel                       string              `json:"labelPlural"`
	Standard                          bool                `json:"-"`
	Name                              string              `json:"name"`
	Label                             string              `json:"label"`
	Fields                            []Field             `json:"fields"`
	ChildRelationships                []ChildRelationship `json:"childRelationships"`
	Createable                        bool                `json:"createable"`
	Updateable                        bool                `json:"updateable"`
	Deletable                         bool                `json:"deletable"`
	Queryable                         bool                `json:"queryable"`
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

// asciiAlnum keeps only ASCII letters and digits, matching the PL/pgSQL
// strip in migration 0005 (non-ASCII removed first so collation-sensitive
// classes and case-folded ranges cannot keep accented or look-alike letters,
// then every other character).
func asciiAlnum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// DefaultChildRelationshipName derives the parent-side name for a child
// object: its plural label with non-alphanumeric characters removed, plus
// "__r" on custom children. The result can be empty, start with a digit or
// exceed childRelNameMax; minting callers must fall back to
// FallbackChildRelationshipName when it is not a valid identifier.
// spec: child-01
func DefaultChildRelationshipName(child Object) string {
	b := asciiAlnum(child.PluralLabel)
	if strings.HasSuffix(child.Name, "__c") {
		b += "__r"
	}
	return b
}

// FallbackChildRelationshipName is the stable name migration 0005 mints when
// DefaultChildRelationshipName is not a valid identifier: "Rel" plus the
// reference field's api_name stripped to ASCII alphanumerics, truncated so
// the whole name fits childRelNameMax and always matches childRelNamePattern.
// spec: child-01
func FallbackChildRelationshipName(fieldAPIName string) string {
	stem := asciiAlnum(fieldAPIName)
	if len(stem) > childRelNameMax-3 {
		stem = stem[:childRelNameMax-3]
	}
	return "Rel" + stem
}

// MintChildRelationshipName is the name migration 0005 writes for a reference
// field that carries no explicit childRelationshipName: the derived default
// when it is a valid identifier, otherwise FallbackChildRelationshipName.
// The create path does not fall back; it rejects an invalid derived default
// and asks for --child-relationship-name.
// spec: child-01
func MintChildRelationshipName(child Object, fieldAPIName string) string {
	if nm := DefaultChildRelationshipName(child); childRelNamePattern.MatchString(nm) {
		return nm
	}
	return FallbackChildRelationshipName(fieldAPIName)
}

func childRelationshipName(o Object, f Field) string {
	if f.Options.ChildRelationshipName != "" {
		return f.Options.ChildRelationshipName
	}
	return DefaultChildRelationshipName(o)
}

// ChildRelationships lists every lookup and master-detail field that
// references object, in catalog order.
// spec: child-01
func (s Snapshot) ChildRelationships(object string) []ChildRelationship {
	if _, e := s.Object(object); e != nil {
		return nil
	}
	out := []ChildRelationship{}
	for _, o := range s.Objects {
		for _, f := range o.Fields {
			if !f.Options.Kind.Reference() || !fieldReferences(f, object) {
				continue
			}
			out = append(out, ChildRelationship{
				ChildSObject:     o.Name,
				Field:            f.Name,
				RelationshipName: childRelationshipName(o, f),
				CascadeDelete:    f.CascadeDelete,
				RestrictedDelete: f.RestrictedDelete,
			})
		}
	}
	return out
}

// ChildRelationship resolves a child relationship name against a parent
// object, case-insensitively.
// spec: child-01
func (s Snapshot) ChildRelationship(parent, name string) (Object, Field, error) {
	if _, e := s.Object(parent); e != nil {
		return Object{}, Field{}, e
	}
	for _, o := range s.Objects {
		for _, f := range o.Fields {
			if !f.Options.Kind.Reference() || !fieldReferences(f, parent) {
				continue
			}
			if strings.EqualFold(childRelationshipName(o, f), name) {
				return o, f, nil
			}
		}
	}
	return Object{}, Field{}, contract.FieldProblem("INVALID_FIELD", "Child relationship is unavailable", name)
}

func fieldReferences(f Field, parent string) bool {
	for _, r := range f.ReferenceTo {
		if strings.EqualFold(r, parent) {
			return true
		}
	}
	return false
}
