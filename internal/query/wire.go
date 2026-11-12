package query

import (
	"github.com/jackc/pgx/v5"
	"salescompel/internal/fieldtype"
	"strings"
)

// selectExpression returns the SQL that reads one stored field in a form
// wireValue can convert without floating point or time zone loss.
func selectExpression(f Field) string {
	col := "r." + pgx.Identifier{f.Column}.Sanitize()
	switch k := f.Kind(); {
	case k.Numeric():
		return col + "::text"
	case k == fieldtype.Date:
		return "to_char(" + col + ",'YYYY-MM-DD')"
	case k == fieldtype.DateTime:
		return "to_char(" + col + ` AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS".000+0000"')`
	}
	return col
}

// wireValue converts one selected value to its REST JSON form.
func wireValue(f Field, raw any) any {
	if raw == nil {
		return nil
	}
	if f.Kind().Numeric() {
		if s, ok := raw.(string); ok {
			return fieldtype.DecimalJSON(s)
		}
	}
	return raw
}

// addressKey maps an address component field name to its compound JSON key.
func addressKey(parent, component string) string {
	prefix := strings.TrimSuffix(parent, "Address")
	key := strings.TrimPrefix(component, prefix)
	return strings.ToLower(key[:1]) + key[1:]
}

// addressValue builds the compound address object, or nil when every
// component is empty.
func addressValue(parent Field, components []Field, values []any) any {
	out := map[string]any{}
	empty := true
	for i, c := range components {
		out[addressKey(parent.Name, c.Name)] = values[i]
		if values[i] != nil {
			empty = false
		}
	}
	if empty {
		return nil
	}
	return out
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
