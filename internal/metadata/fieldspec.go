package metadata

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"regexp"
	"salescompel/internal/fieldtype"
	"strings"
)

// FieldInput is an operator's request to create one active field.
type FieldInput struct {
	Object, Name, Label, Type string
	Length                    int
	Precision, Scale          int
	VisibleLines              int
	Values                    []PicklistValue
	Restricted, Required      bool
	Reparentable              bool
	Default                   *string
	ReferenceTo               string
	DeleteBehavior            string
	DisplayFormat             string
	RelationshipName          string
	ChildRelationshipName     string
	StartingNumber            int64
}

// ColumnDDL is the physical definition for one new field column.
type ColumnDDL struct {
	Type       string // column type with NOT NULL/DEFAULT where applicable
	Check      string // optional CHECK constraint expression
	ForeignKey string // optional complete FOREIGN KEY constraint clause
}

// childRelNameMax caps parent-side relationship names, matching the stem
// limit on relationshipName below. childRelNamePattern must agree.
const childRelNameMax = 40

var (
	relationshipName    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}__r$`)
	childRelNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}$`)
)

func fail(format string, args ...any) error { return fmt.Errorf("invalid field: "+format, args...) }

// BuildField validates an operator request against the catalog and returns
// the completed definition (with generated ID and column) and its DDL.
// hasRecords reports whether the target object already holds records.
func BuildField(in FieldInput, target Object, snapshot Snapshot, hasRecords bool) (Field, ColumnDDL, error) {
	ddl := ColumnDDL{}
	k, err := fieldtype.ParseKind(in.Type)
	if err != nil || !k.Active() {
		return Field{}, ddl, fail("type %q is not supported", in.Type)
	}
	if !ValidAPIName(in.Name) || !validLabel(in.Label) {
		return Field{}, ddl, fail("invalid name or label")
	}
	for _, old := range target.Fields {
		if strings.EqualFold(old.Name, in.Name) {
			return Field{}, ddl, errors.New("field already exists")
		}
	}
	numeric, picklist, reference := k.Numeric(), k == fieldtype.Picklist || k == fieldtype.MultiselectPicklist, k.Reference()
	irrelevant := map[string]bool{
		"length":                  in.Length != 0 && k != fieldtype.Text && k != fieldtype.LongTextArea,
		"precision/scale":         (in.Precision != 0 || in.Scale != 0) && !numeric,
		"visible lines":           in.VisibleLines != 0 && k != fieldtype.LongTextArea && k != fieldtype.MultiselectPicklist,
		"values":                  (len(in.Values) != 0 || in.Restricted) && !picklist,
		"default":                 in.Default != nil && k != fieldtype.Checkbox && !picklist,
		"reference":               (in.ReferenceTo != "" || in.RelationshipName != "") && !reference,
		"child relationship name": in.ChildRelationshipName != "" && !reference,
		"delete behavior":         in.DeleteBehavior != "" && k != fieldtype.Lookup,
		"reparentable":            in.Reparentable && k != fieldtype.MasterDetail,
		"display format":          (in.DisplayFormat != "" || in.StartingNumber != 0) && k != fieldtype.AutoNumber,
		"required":                in.Required && (k == fieldtype.Checkbox || k == fieldtype.AutoNumber || k == fieldtype.MasterDetail || k == fieldtype.TextArea || k == fieldtype.LongTextArea || k == fieldtype.MultiselectPicklist),
		"negative starting":       in.StartingNumber < 0,
	}
	for name, bad := range irrelevant {
		if bad {
			return Field{}, ddl, fail("%s does not apply to %s", name, k)
		}
	}
	// Standard objects share one physical table across tenants, so a NOT NULL
	// or auto-numbered column would affect every tenant's records.
	if target.Standard && (in.Required || k == fieldtype.AutoNumber) {
		return Field{}, ddl, fail("required and auto-number fields cannot be added to standard objects, whose storage is shared")
	}
	if in.Required && hasRecords {
		return Field{}, ddl, fail("required fields can only be added to objects without records")
	}
	f := Field{ID: randomID(), Name: in.Name, Label: in.Label, Nillable: !in.Required, Createable: true, Updateable: true, Options: Options{Kind: k}}
	f.Column = "f_" + f.ID
	notNull := ""
	if in.Required {
		notNull = " NOT NULL"
	}
	col := pgx.Identifier{f.Column}.Sanitize()
	switch k {
	case fieldtype.Text:
		if in.Length < 1 || in.Length > 255 {
			return Field{}, ddl, fail("Text length must be 1-255")
		}
		f.Length = in.Length
		ddl.Type = fmt.Sprintf("varchar(%d)", in.Length)
	case fieldtype.TextArea:
		f.Length, ddl.Type = 255, "varchar(255)"
	case fieldtype.LongTextArea:
		f.Length = in.Length
		if f.Length == 0 {
			f.Length = 32768
		}
		if f.Length < 256 || f.Length > 131072 {
			return Field{}, ddl, fail("LongTextArea length must be 256-131072")
		}
		f.Options.VisibleLines = in.VisibleLines
		ddl.Type, ddl.Check = "text", fmt.Sprintf("char_length(%s)<=%d", col, f.Length)
	case fieldtype.Number, fieldtype.Currency, fieldtype.Percent:
		p, s := in.Precision, in.Scale
		if p == 0 && s == 0 {
			p = 18
			if k == fieldtype.Currency {
				s = 2
			}
		}
		if p < 1 || p > 18 || s < 0 || s > p {
			return Field{}, ddl, fail("precision must be 1-18 and scale 0-precision")
		}
		f.Precision, f.Scale = p, s
		ddl.Type = fmt.Sprintf("numeric(%d,%d)", p, s)
	case fieldtype.Date:
		ddl.Type = "date"
	case fieldtype.DateTime:
		ddl.Type = "timestamptz"
	case fieldtype.Checkbox:
		def := false
		if in.Default != nil {
			switch *in.Default {
			case "true":
				def = true
			case "false":
			default:
				return Field{}, ddl, fail("Checkbox default must be true or false")
			}
		}
		f.Nillable, f.DefaultValue = false, def
		ddl.Type = fmt.Sprintf("boolean NOT NULL DEFAULT %t", def)
	case fieldtype.Picklist, fieldtype.MultiselectPicklist:
		if err := checkValues(in.Values, k == fieldtype.MultiselectPicklist); err != nil {
			return Field{}, ddl, err
		}
		f.PicklistValues = make([]PicklistValue, len(in.Values))
		copy(f.PicklistValues, in.Values)
		f.RestrictedPicklist = in.Restricted
		if in.Default != nil {
			found := false
			for i := range f.PicklistValues {
				if f.PicklistValues[i].Value == *in.Default && f.PicklistValues[i].Active {
					f.PicklistValues[i].DefaultValue, found = true, true
				}
			}
			if !found {
				return Field{}, ddl, fail("default must be an active picklist value")
			}
		}
		if k == fieldtype.Picklist {
			ddl.Type = "varchar(255)"
		} else {
			f.Options.VisibleLines = in.VisibleLines
			ddl.Type = "varchar(4099)"
		}
	case fieldtype.Email:
		ddl.Type = "varchar(80)"
	case fieldtype.Phone:
		ddl.Type = "varchar(40)"
	case fieldtype.Url:
		ddl.Type = "varchar(255)"
	case fieldtype.Lookup, fieldtype.MasterDetail:
		parent, err := snapshot.Object(in.ReferenceTo)
		if err != nil {
			return Field{}, ddl, fail("reference target %q does not exist", in.ReferenceTo)
		}
		rel := in.RelationshipName
		if rel == "" {
			rel = strings.TrimSuffix(in.Name, "__c") + "__r"
		}
		if !relationshipName.MatchString(rel) {
			return Field{}, ddl, fail("invalid relationship name %q", rel)
		}
		masters := 0
		for _, old := range target.Fields {
			if old.RelationshipName != nil && strings.EqualFold(*old.RelationshipName, rel) {
				return Field{}, ddl, fail("relationship name %q already used", rel)
			}
			if old.Options.Kind == fieldtype.MasterDetail {
				masters++
			}
		}
		f.ReferenceTo, f.RelationshipName, f.Length = []string{parent.Name}, &rel, 18
		// spec: child-01
		childName := in.ChildRelationshipName
		if childName == "" {
			childName = DefaultChildRelationshipName(target)
		}
		if !childRelNamePattern.MatchString(childName) {
			if in.ChildRelationshipName == "" {
				return Field{}, ddl, fail("invalid child relationship name %q; pass --child-relationship-name to choose another", childName)
			}
			return Field{}, ddl, fail("invalid child relationship name %q", childName)
		}
		for _, o := range snapshot.Objects {
			for _, old := range o.Fields {
				if !old.Options.Kind.Reference() || !fieldReferences(old, parent.Name) {
					continue
				}
				if strings.EqualFold(childRelationshipName(o, old), childName) {
					return Field{}, ddl, fail("child relationship name %q already used on %s; pass --child-relationship-name to choose another", childName, parent.Name)
				}
			}
		}
		f.Options.ChildRelationshipName = childName
		action := ""
		if k == fieldtype.Lookup {
			behavior := in.DeleteBehavior
			if in.Required {
				// A required lookup cannot be cleared when its parent is deleted.
				if behavior == "SetNull" {
					return Field{}, ddl, fail("a required lookup must use the Restrict delete behavior")
				}
				behavior = "Restrict"
			}
			switch behavior {
			case "", "SetNull":
				f.Options.DeleteBehavior, action = "SetNull", fmt.Sprintf("SET NULL (%s)", col)
			case "Restrict":
				f.Options.DeleteBehavior, action = "Restrict", "RESTRICT"
			default:
				return Field{}, ddl, fail("delete behavior must be SetNull or Restrict")
			}
		} else {
			switch {
			case target.Standard:
				return Field{}, ddl, fail("a standard object cannot be the detail of a master-detail relationship")
			case strings.EqualFold(parent.Name, target.Name):
				return Field{}, ddl, fail("an object cannot be its own master")
			case masters >= 2:
				return Field{}, ddl, fail("an object can have at most two master-detail relationships")
			case hasRecords:
				return Field{}, ddl, fail("master-detail fields can only be added to objects without records")
			}
			f.Nillable, f.Updateable = false, in.Reparentable
			f.Options.DeleteBehavior, f.Options.Reparentable, action = "Cascade", in.Reparentable, "CASCADE"
			f.RelationshipOrder = ptr(masters)
			notNull = " NOT NULL"
		}
		ddl.Type = "varchar(18)"
		ddl.ForeignKey = fmt.Sprintf("CONSTRAINT %s FOREIGN KEY(tenant_id,%s) REFERENCES %s(tenant_id,id) ON DELETE %s",
			pgx.Identifier{"fk_" + f.ID}.Sanitize(), col, parent.TableSQL(), action)
	case fieldtype.AutoNumber:
		if !fieldtype.ValidAutoNumberFormat(in.DisplayFormat) {
			return Field{}, ddl, fail("invalid auto-number display format")
		}
		f.Nillable, f.Createable, f.Updateable, f.Length = false, false, false, 30
		f.Options.DisplayFormat, f.Options.StartingNumber = in.DisplayFormat, in.StartingNumber
		ddl.Type = "varchar(30)" // NOT NULL is applied after backfill
	default:
		return Field{}, ddl, fail("type %q is not creatable", in.Type)
	}
	ddl.Type += notNull
	return Complete(f), ddl, nil
}

func checkValues(values []PicklistValue, multi bool) error {
	if len(values) < 1 || len(values) > 1000 {
		return fail("picklists need 1-1000 values")
	}
	seen := map[string]bool{}
	for _, v := range values {
		if strings.TrimSpace(v.Value) == "" || !fieldtype.ValidText(v.Value, 255) || strings.TrimSpace(v.Label) == "" || !fieldtype.ValidText(v.Label, 255) {
			return fail("invalid picklist value %q", v.Value)
		}
		if multi && strings.Contains(v.Value, ";") {
			return fail("multi-select values cannot contain ';'")
		}
		key := strings.ToLower(v.Value)
		if seen[key] {
			return fail("duplicate picklist value %q", v.Value)
		}
		seen[key] = true
	}
	return nil
}
