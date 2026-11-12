package records

import (
	"context"
	"salescompel/internal/fieldtype"
	"time"
)

type autoValue struct{ column, value string }

// allocateAutoNumbers takes the next number for each auto-number field of the
// object inside the caller's transaction. Rolled-back numbers leave gaps.
func allocateAutoNumbers(ctx context.Context, db database, p Principal, object string, now time.Time) ([]autoValue, error) {
	o, err := p.Catalog.Object(object)
	if err != nil {
		return nil, err
	}
	out := []autoValue{}
	for _, f := range o.Fields {
		if f.Kind() != fieldtype.AutoNumber {
			continue
		}
		var n int64
		if err := db.QueryRow(ctx, "UPDATE sc_catalog.auto_numbers SET next_value=next_value+1 WHERE tenant_id=$1 AND field_id=$2 RETURNING next_value-1", p.Tenant, f.ID).Scan(&n); err != nil {
			return nil, dbError(err)
		}
		v, err := fieldtype.FormatAutoNumber(f.Options.DisplayFormat, n, now.UTC())
		if err != nil {
			return nil, invalid("NUMBER_OUTSIDE_VALID_RANGE", "Auto-number value exceeds 30 characters")
		}
		out = append(out, autoValue{f.Column, v})
	}
	return out, nil
}
