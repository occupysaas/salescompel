package platform

import "net/url"

// queryColumnsRequested implements the bounded candidate option contract. It
// deliberately rejects ambiguous values instead of silently picking the first.
func queryColumnsRequested(values url.Values) (bool, error) {
	columns, present := values["columns"]
	if !present {
		return false, nil
	}
	if len(columns) == 1 {
		switch columns[0] {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, invalid("INVALID_QUERY_PARAMETER", "columns must be specified once as true or false")
}

// queryColumnMetadata resolves only the existing authorized projection. Callers
// must also complete Select successfully so filter/order policy remains enforced.
// This minimal candidate schema serves the pinned CLI's nonaggregate consumer;
// it is not a claim about undocumented Salesforce metadata properties.
func queryColumnMetadata(p Principal, q Query) ([]map[string]any, error) {
	if _, err := Describe(p, q.Object); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(q.Fields))
	for _, name := range q.Fields {
		if q.Object == "Contact" && name == "Account.Name" {
			if _, err := field(p, "Account", "Name"); err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"columnName": "Account", "joinColumns": []map[string]any{{"columnName": "Name"}}})
			continue
		}
		f, err := field(p, q.Object, name)
		if err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"columnName": f.Name})
	}
	return out, nil
}
