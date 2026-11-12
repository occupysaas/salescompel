package records

import (
	"errors"
	"salescompel/internal/fieldtype"
	"strings"
)

func parseError(f Field, message string) error {
	return fieldProblem("JSON_PARSER_ERROR", message, f.Name)
}

func activeValue(f Field, v string) bool {
	for _, p := range f.PicklistValues {
		if p.Active && p.Value == v {
			return true
		}
	}
	return false
}

// coerce validates one non-nil field value and returns the SQL parameter and
// the explicit cast its placeholder needs. A nil parameter stores NULL.
func coerce(f Field, v any) (any, string, error) {
	k := f.Kind()
	if v == nil {
		if k == fieldtype.Checkbox {
			return false, "", nil
		}
		return nil, "", nil
	}
	switch k {
	case fieldtype.Checkbox:
		b, ok := v.(bool)
		if !ok {
			return nil, "", parseError(f, "Expected true, false or null")
		}
		return b, "", nil
	case fieldtype.Number, fieldtype.Currency, fieldtype.Percent:
		if s, ok := v.(string); ok && s == "" && f.Nillable {
			return nil, "", nil
		}
		p, s := f.NumericBounds()
		d, err := fieldtype.Decimal(v, p, s)
		if errors.Is(err, fieldtype.ErrRange) {
			return nil, "", fieldProblem("NUMBER_OUTSIDE_VALID_RANGE", "Number is outside the field's valid range", f.Name)
		}
		if err != nil {
			return nil, "", parseError(f, "Expected a decimal number or null")
		}
		return d, "::numeric", nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, "", parseError(f, "Expected a string or null")
	}
	if s == "" && f.Nillable {
		return nil, "", nil
	}
	switch k {
	case fieldtype.Date:
		d, err := fieldtype.ParseDate(s)
		if err != nil {
			return nil, "", parseError(f, "Expected a date as YYYY-MM-DD")
		}
		return d, "::date", nil
	case fieldtype.DateTime:
		t, err := fieldtype.ParseDateTime(s)
		if err != nil {
			return nil, "", parseError(f, "Expected an ISO 8601 date-time")
		}
		return t, "", nil
	case fieldtype.MultiselectPicklist:
		defined := make([]string, 0, len(f.PicklistValues))
		for _, p := range f.PicklistValues {
			defined = append(defined, p.Value)
		}
		canonical, selected := fieldtype.MultiPicklist(s, defined)
		if len(selected) > 100 {
			return nil, "", fieldProblem("FIELD_INTEGRITY_EXCEPTION", "Too many values selected", f.Name)
		}
		for _, value := range selected {
			if !fieldtype.ValidText(value, 255) || f.RestrictedPicklist && !activeValue(f, value) {
				return nil, "", fieldProblem("INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST", "Value is not in the picklist", f.Name)
			}
		}
		if canonical == "" {
			if !f.Nillable {
				return nil, "", fieldProblem("REQUIRED_FIELD_MISSING", "Required value is missing", f.Name)
			}
			return nil, "", nil
		}
		if !fieldtype.ValidText(canonical, f.Length) {
			return nil, "", fieldProblem("STRING_TOO_LONG", "Invalid or oversized field value", f.Name)
		}
		return canonical, "", nil
	}
	if !f.Nillable && strings.TrimSpace(s) == "" {
		return nil, "", fieldProblem("REQUIRED_FIELD_MISSING", "Required value is missing", f.Name)
	}
	if !fieldtype.ValidText(s, f.Length) {
		return nil, "", fieldProblem("STRING_TOO_LONG", "Invalid or oversized field value", f.Name)
	}
	switch k {
	case fieldtype.Email:
		if !fieldtype.ValidEmail(s) {
			return nil, "", fieldProblem("INVALID_EMAIL_ADDRESS", "Invalid email address", f.Name)
		}
	case fieldtype.Picklist:
		if f.RestrictedPicklist && !activeValue(f, s) {
			return nil, "", fieldProblem("INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST", "Value is not in the restricted picklist", f.Name)
		}
	}
	return s, "", nil
}

// applyDefaults fills omitted checkbox and picklist values on create.
func applyDefaults(o Object, data map[string]any) {
	for _, f := range o.Fields {
		if _, given := data[f.Name]; given {
			continue
		}
		switch f.Kind() {
		case fieldtype.Checkbox:
			if b, ok := f.DefaultValue.(bool); ok {
				data[f.Name] = b
			}
		case fieldtype.Picklist, fieldtype.MultiselectPicklist:
			for _, p := range f.PicklistValues {
				if p.DefaultValue && p.Active {
					data[f.Name] = p.Value
				}
			}
		}
	}
}
