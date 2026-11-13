package soql

import (
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

// restDateTimeLayout is the Plan 28 REST date-time wire form (UTC).
const restDateTimeLayout = `2006-01-02T15:04:05.000+0000`

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

// numericAggregate reports an aggregate whose exact value must be carried as
// numeric text: SUM and AVG always, MIN and MAX over numeric fields.
func numericAggregate(v BoundValue) bool {
	switch v.Agg {
	case "SUM", "AVG":
		return true
	case "MIN", "MAX":
		return v.Field.Kind().Numeric()
	}
	return false
}

// wireColumnValue converts one bound SELECT value, accounting for result
// kinds that differ from the argument field: DAY_ONLY yields a calendar date,
// and MIN/MAX over temporal fields return that temporal value. spec: soql-05
func wireColumnValue(col BoundColumn, raw any) any {
	if raw == nil {
		return nil
	}
	t, ok := raw.(time.Time)
	if !ok {
		return WireValue(col.Field, raw)
	}
	if col.Func == "DAY_ONLY" {
		return t.Format("2006-01-02")
	}
	if col.Agg == "MIN" || col.Agg == "MAX" {
		switch col.Field.Kind() {
		case fieldtype.Date:
			return t.Format("2006-01-02")
		case fieldtype.DateTime:
			return t.UTC().Format(restDateTimeLayout)
		}
	}
	return raw
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

// ColumnMetadata computes REST columnMetadata for bound query columns.
// spec: soql-01
func ColumnMetadata(cols []BoundColumn) []map[string]any {
	out := make([]map[string]any, 0, len(cols))
	for _, col := range cols {
		if len(col.Path) > 0 {
			rel := "relationship"
			if col.Path[0].Via.RelationshipName != nil {
				rel = *col.Path[0].Via.RelationshipName
			}
			out = append(out, map[string]any{
				"columnName":  rel,
				"joinColumns": []map[string]any{{"columnName": col.Field.Name}},
			})
			continue
		}
		out = append(out, map[string]any{"columnName": col.Field.Name})
	}
	return out
}
