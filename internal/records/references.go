package records

import (
	"salescompel/internal/recordid"
)

func normalizeReferences(p Principal, object string, data map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(data))
	for k, v := range data {
		f, err := field(p, object, k)
		if err != nil {
			return nil, err
		}
		if s, ok := v.(string); ok && s != "" && f.Type == "reference" {
			normalized, err := normalizeID(p, f.ReferenceTo[0], s)
			if err != nil {
				return nil, &APIError{Status: 400, Code: "MALFORMED_ID", Message: "Invalid reference ID", Fields: []string{f.Name}}
			}
			v = normalized
		}
		out[k] = v
	}
	return out, nil
}

func normalizeID(p Principal, object, input string) (string, error) {
	if p.Catalog == nil {
		return "", recordid.ErrMalformedID
	}
	o, e := p.Catalog.Object(object)
	if e != nil {
		return "", e
	}
	return recordid.Normalize(o.Prefix, input)
}
