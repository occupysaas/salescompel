package query

import (
	"salescompel/internal/fieldtype"
	"salescompel/internal/soql"
)

// selectExpression returns the SQL that reads one stored field in a form
// wireValue can convert without floating point or time zone loss.
func selectExpression(f Field) string {
	return soql.SelectExpression("r", f)
}

// wireValue converts one selected value to its REST JSON form.
func wireValue(f Field, raw any) any {
	return soql.WireValue(f, raw)
}

// addressKey maps an address component field name to its compound JSON key.
func addressKey(parent, component string) string {
	return soql.AddressKey(parent, component)
}

// addressValue builds the compound address object, or nil when every
// component is empty.
func addressValue(parent Field, components []Field, values []any) any {
	return soql.AddressValue(parent, components, values)
}

// caseInsensitive reports kinds compared with case-insensitive equality.
func caseInsensitive(k fieldtype.Kind) bool {
	switch k {
	case fieldtype.Text, fieldtype.TextArea, fieldtype.Email, fieldtype.Phone, fieldtype.Url, fieldtype.PersonName:
		return true
	}
	return false
}

// literalFilterPending reports kinds whose equality needs typed SOQL literals
// (Plan 29); quoted-string filters on them are refused rather than coerced.
func literalFilterPending(k fieldtype.Kind) bool {
	return k.Numeric() || k == fieldtype.Date || k == fieldtype.DateTime || k == fieldtype.Checkbox
}
