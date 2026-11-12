package soql

import (
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

// SelectExpression returns the SQL that reads one stored field in a form
// WireValue can convert without floating point or time zone loss.
func SelectExpression(alias string, f metadata.Field) string {
	col := alias + "." + pgx.Identifier{f.Column}.Sanitize()
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

// WireValue converts one selected value to its REST JSON form.
func WireValue(f metadata.Field, raw any) any {
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

// AddressKey maps an address component field name to its compound JSON key.
func AddressKey(parent, component string) string {
	prefix := strings.TrimSuffix(parent, "Address")
	key := strings.TrimPrefix(component, prefix)
	return strings.ToLower(key[:1]) + key[1:]
}

// AddressValue builds the compound address object, or nil when every
// component is empty.
func AddressValue(parent metadata.Field, components []metadata.Field, values []any) any {
	out := map[string]any{}
	empty := true
	for i, c := range components {
		out[AddressKey(parent.Name, c.Name)] = values[i]
		if values[i] != nil {
			empty = false
		}
	}
	if empty {
		return nil
	}
	return out
}
