package platform

import (
	"salescompel/internal/metadata"
	"salescompel/internal/query"
)

func canonicalQuery(p Principal, q Query) (Query, error) {
	if p.Catalog == nil {
		snapshot := metadata.Snapshot{Tenant: p.Tenant, Objects: metadata.StandardObjects()}
		p.Catalog = &snapshot
	}
	return query.Normalize(p, q)
}

func queryColumnMetadata(p Principal, q Query) ([]map[string]any, error) {
	if p.Catalog == nil {
		snapshot := metadata.Snapshot{Tenant: p.Tenant, Objects: metadata.StandardObjects()}
		p.Catalog = &snapshot
	}
	return query.ColumnMetadata(p, q)
}
