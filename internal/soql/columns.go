package soql

// ColumnMetadata computes the REST columnMetadata array for a bound query.
// Plain root fields report their name; entries nested inside joinColumns report
// nested joinColumns for parent paths and child subqueries, and aggregates
// report their result key with aggregate=true. Every nested entry, including a
// leaf, carries an explicit joinColumns array because the pinned CLI reads its
// length while walking the chain. spec: soql-08
func ColumnMetadata(b *Bound) []map[string]any {
	return columnMetadata(b, false)
}

func columnMetadata(b *Bound, nested bool) []map[string]any {
	if b == nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(b.Columns)+len(b.Children))
	for _, col := range b.Columns {
		out = append(out, columnMetadataFor(col, nested))
	}
	for _, child := range b.Children {
		out = append(out, map[string]any{
			"columnName":  child.Key,
			"joinColumns": columnMetadata(child.Query, true),
		})
	}
	return out
}

func columnMetadataFor(col BoundColumn, nested bool) map[string]any {
	if col.Agg != "" {
		return map[string]any{"columnName": col.Key, "aggregate": true}
	}
	name := col.Key
	if name == "" {
		name = col.Field.Name
	}
	return parentColumnMetadata(col.Path, name, nested)
}

// parentColumnMetadata nests one joinColumns object per parent-relationship
// hop. The leaf carries the selected field or function result key, plus an
// empty joinColumns array when it sits inside a joinColumns chain.
func parentColumnMetadata(path []PathStep, leaf string, nested bool) map[string]any {
	if len(path) == 0 {
		out := map[string]any{"columnName": leaf}
		if nested {
			out["joinColumns"] = []map[string]any{}
		}
		return out
	}
	rel := "relationship"
	if path[0].Via.RelationshipName != nil {
		rel = *path[0].Via.RelationshipName
	}
	return map[string]any{
		"columnName":  rel,
		"joinColumns": []map[string]any{parentColumnMetadata(path[1:], leaf, true)},
	}
}
