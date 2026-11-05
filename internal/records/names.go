package records

import (
	"sort"
)

// canonicalFields rejects ambiguous decoded maps, whose source order is lost.
func canonicalFields(p Principal, object string, data map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(data))
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		f, err := field(p, object, k)
		if err != nil {
			return nil, err
		}
		if _, exists := out[f.Name]; exists {
			return nil, fieldProblem("INVALID_FIELD", "Ambiguous field names", f.Name)
		}
		out[f.Name] = data[k]
	}
	return out, nil
}
