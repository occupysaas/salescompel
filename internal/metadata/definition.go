package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"salescompel/internal/fieldtype"
)

// definitionVersion is the persisted shape of sc_catalog.fields.definition.
const definitionVersion = 2

type storedDefinition struct {
	V       int     `json:"v"`
	Field   Field   `json:"field"`
	Options Options `json:"options"`
}

var defaultLength = map[fieldtype.Kind]int{
	fieldtype.TextArea: 255, fieldtype.Picklist: 255, fieldtype.MultiselectPicklist: 4099, fieldtype.Email: 80,
	fieldtype.Phone: 40, fieldtype.Url: 255, fieldtype.Lookup: 18, fieldtype.MasterDetail: 18, fieldtype.AutoNumber: 30,
}

// Complete derives describe properties that follow from the field's kind,
// so seeds, schema commands and migrations agree.
func Complete(f Field) Field {
	k := f.Options.Kind
	if !(k == fieldtype.Number && f.Type == "int") {
		f.Type = k.DescribeType()
		f.SoapType = k.SoapType()
	} else {
		f.SoapType = "xsd:int"
	}
	if f.Label == "" {
		f.Label = f.Name
	}
	if f.Length == 0 {
		f.Length = defaultLength[k]
	}
	if f.ReferenceTo == nil {
		f.ReferenceTo = []string{}
	}
	if f.PicklistValues == nil {
		f.PicklistValues = []PicklistValue{}
	}
	f.AutoNumber = k == fieldtype.AutoNumber
	f.Filterable = k.Filterable()
	f.Sortable = k.Sortable()
	f.Groupable = k.Filterable() && !k.Numeric() && k != fieldtype.DateTime && k != fieldtype.TextArea
	f.CascadeDelete = k.Reference() && f.Options.DeleteBehavior == "Cascade"
	f.RestrictedDelete = k.Reference() && f.Options.DeleteBehavior == "Restrict"
	f.DefaultedOnCreate = k == fieldtype.Checkbox || k == fieldtype.AutoNumber || f.DefaultValue != nil
	return f
}

// EncodeDefinition writes the persisted definition shape.
func EncodeDefinition(f Field) ([]byte, error) {
	if !f.Options.Kind.Active() {
		return nil, fmt.Errorf("field %s: inactive kind %q", f.Name, f.Options.Kind)
	}
	return json.Marshal(storedDefinition{V: definitionVersion, Field: f, Options: f.Options})
}

// DecodeDefinition strictly reads a persisted definition. Unknown keys,
// other versions, inactive kinds and kind/type mismatches are rejected.
func DecodeDefinition(raw []byte) (Field, error) {
	var d storedDefinition
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&d); e != nil {
		return Field{}, e
	}
	if d.V != definitionVersion {
		return Field{}, errors.New("unsupported definition version")
	}
	k := d.Options.Kind
	if !k.Active() {
		return Field{}, errors.New("unsupported active field type")
	}
	want := k.DescribeType()
	if k == fieldtype.Number && d.Field.Type == "int" {
		want = "int"
	}
	if d.Field.Type != want {
		return Field{}, errors.New("field type does not match its kind")
	}
	f := d.Field
	f.Options = d.Options
	return f, nil
}
