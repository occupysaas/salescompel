package query

func ColumnMetadata(p Principal, q Query) ([]map[string]any, error) {
	var err error
	q, err = canonicalQuery(p, q)
	if err != nil {
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
