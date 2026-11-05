package query

import (
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/recordid"
	"strings"
)

type NullPlacement uint8

const (
	NullDefault NullPlacement = iota
	NullFirst
	NullLast
)

func canonicalQuery(p Principal, q Query) (Query, error) {
	object, ok := canonicalObject(p, q.Object)
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
			value, err := normalizeID(p, target, *q.WhereValue)
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

type Principal = policy.Principal
type Field = metadata.Field

var invalid = contract.Invalid
var fieldProblem = contract.FieldProblem
var field = policy.Field

func canonicalObject(p Principal, name string) (string, bool) {
	if p.Catalog == nil {
		return "", false
	}
	o, e := p.Catalog.Object(name)
	return o.Name, e == nil
}
func normalizeID(p Principal, object, input string) (string, error) {
	o, e := p.Catalog.Object(object)
	if e != nil {
		return "", e
	}
	return recordid.Normalize(o.Prefix, input)
}

func Normalize(p Principal, q Query) (Query, error) { return canonicalQuery(p, q) }
