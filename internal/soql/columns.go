package soql

// ColumnMetadata computes the REST columnMetadata array for a bound query.
// Plain fields report their name, parent paths report nested joinColumns,
// child subqueries report the relationship name and their inner columns, and
// aggregates report their result key with aggregate=true. spec: soql-08
func ColumnMetadata(b *Bound) []map[string]any {
	if b == nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(b.Columns)+len(b.Children))
	for _, col := range b.Columns {
		out = append(out, columnMetadataFor(col))
	}
	for _, child := range b.Children {
		out = append(out, map[string]any{
			"columnName":  child.Key,
			"joinColumns": ColumnMetadata(child.Query),
		})
	}
	return out
}

func columnMetadataFor(col BoundColumn) map[string]any {
	if col.Agg != "" {
		return map[string]any{"columnName": col.Key, "aggregate": true}
	}
	name := col.Key
	if name == "" {
		name = col.Field.Name
	}
	return parentColumnMetadata(col.Path, name)
}

// parentColumnMetadata nests one joinColumns object per parent-relationship
// hop. The leaf carries the selected field or function result key.
func parentColumnMetadata(path []PathStep, leaf string) map[string]any {
	if len(path) == 0 {
		return map[string]any{"columnName": leaf}
	}
	rel := "relationship"
	if path[0].Via.RelationshipName != nil {
		rel = *path[0].Via.RelationshipName
	}
	return map[string]any{
		"columnName":  rel,
		"joinColumns": []map[string]any{parentColumnMetadata(path[1:], leaf)},
	}
}
