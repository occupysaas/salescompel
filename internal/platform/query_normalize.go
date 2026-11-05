package platform

import "strings"

type NullPlacement uint8

const (
	NullDefault NullPlacement = iota
	NullFirst
	NullLast
)

func canonicalQuery(p Principal, q Query) (Query, error) {
	object, ok := CanonicalObject(q.Object)
	if !ok {
		return q, invalid("INVALID_TYPE", "Unsupported object")
	}
	q.Object = object
	names := make([]string, len(q.Fields))
	for i, name := range q.Fields {
		if object == "Contact" && strings.EqualFold(name, "Account.Name") {
			names[i] = "Account.Name"
			continue
		}
		f, err := field(p, object, name)
		if err != nil {
			return q, err
		}
		names[i] = f.Name
	}
	q.Fields = names
	if q.HasWhere {
		f, err := field(p, object, q.WhereField)
		if err != nil {
			return q, err
		}
		q.WhereField = f.Name
		if q.WhereValue != nil && (f.Type == "id" || f.Type == "reference") {
			target := object
			if f.Type == "reference" {
				target = f.ReferenceTo[0]
			}
			value, err := NormalizeRecordID(target, *q.WhereValue)
			if err != nil {
				return q, fieldProblem("INVALID_QUERY_FILTER_OPERATOR", "Invalid ID filter", f.Name)
			}
			q.WhereValue = &value
		}
	}
	if q.Order != "" {
		f, err := field(p, object, q.Order)
		if err != nil {
			return q, err
		}
		q.Order = f.Name
	}
	return q, nil
}
