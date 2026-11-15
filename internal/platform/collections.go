package platform

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"salescompel/internal/records"
	soqleng "salescompel/internal/soql"
)

// sObject Collections executor. Runs on a Store bound to a transaction.
// spec: col-01, col-02, col-03

// errCollectionsFailed reports that an item failed while all-or-none was
// forced by an outer Composite request; the returned body is the rolled-back
// item array.
var errCollectionsFailed = errors.New("collections item failed")

// retrieveChunk keeps each WHERE clause inside the SOQL 4,000-character limit
// (150 quoted 18-character IDs is about 3,200 characters).
const retrieveChunk = 150

type collectionsItem struct {
	object string
	id     string // update, delete
	key    string // upsert
	data   map[string]any
	err    *APIError // decided before execution
}

func (s *Store) collections(ctx context.Context, p Principal, route subRoute, params url.Values, data map[string]any, force bool) (int, any, error) {
	switch route.kind {
	case routeCollRetrieveGet, routeCollRetrievePost:
		var ids, fields []string
		if route.kind == routeCollRetrieveGet {
			if err := checkRetrieveGetParams(params.Get("ids"), params.Get("fields"), params.Has("ids"), params.Has("fields")); err != nil {
				return 0, nil, err
			}
			ids, fields = splitIDs(params.Get("ids")), splitFields(params.Get("fields"))
		} else {
			if err := validateCollectionsRequest(route, data, nil); err != nil {
				return 0, nil, err
			}
			ids, fields = stringList(data["ids"]), stringList(data["fields"])
		}
		out, err := s.collectionsRetrieve(ctx, p, route.object, ids, fields)
		return 200, out, err
	case routeCollDelete:
		if err := checkDeleteParams(params.Get("ids"), params.Get("allOrNone"), params.Has("ids"), params.Has("allOrNone")); err != nil {
			return 0, nil, err
		}
		var items []collectionsItem
		for _, id := range splitIDs(params.Get("ids")) {
			items = append(items, deleteItem(p, id))
		}
		return s.collectionsWrite(ctx, p, routeCollDelete, items, params.Get("allOrNone") == "true" || force, force)
	}
	if err := validateCollectionsWrite(route.kind, data); err != nil {
		return 0, nil, err
	}
	allOrNone, _ := data["allOrNone"].(bool)
	var items []collectionsItem
	for _, raw := range data["records"].([]any) {
		items = append(items, writeItem(route.kind, raw.(map[string]any)))
	}
	if route.kind == routeCollUpsert {
		var keys []string
		for _, it := range items {
			if it.err == nil {
				keys = append(keys, it.key)
			}
		}
		if err := s.recordTx(p).LockAccountKeys(ctx, keys); err != nil {
			return 0, nil, err
		}
	}
	return s.collectionsWrite(ctx, p, route.kind, items, allOrNone || force, force)
}

func stringList(v any) []string {
	var out []string
	list, _ := v.([]any)
	for _, x := range list {
		if s, ok := x.(string); ok {
			out = append(out, s)
		} else {
			out = append(out, "")
		}
	}
	return out
}

// writeItem turns one record into an operation, stripping attributes and
// taking the ID (update) or external key (upsert) out of the field data.
func writeItem(kind routeKind, record map[string]any) collectionsItem {
	typ, _ := recordType(record)
	it := collectionsItem{object: typ, data: map[string]any{}}
	for k, v := range record {
		switch {
		case k == "attributes":
		case kind == routeCollUpdate && strings.EqualFold(k, "id"):
			id, ok := v.(string)
			if !ok {
				it.err = &APIError{Status: 400, Code: "MALFORMED_ID", Message: "Malformed record ID", Fields: []string{"Id"}}
			}
			it.id = id
		case kind == routeCollUpsert && strings.EqualFold(k, "Compat_Key__c"):
			key, ok := v.(string)
			if !ok || strings.TrimSpace(key) == "" {
				it.err = &APIError{Status: 400, Code: "REQUIRED_FIELD_MISSING", Message: "External ID value is required", Fields: []string{"Compat_Key__c"}}
			}
			it.key = key
		default:
			it.data[k] = v
		}
	}
	if kind == routeCollUpsert && it.key == "" && it.err == nil {
		it.err = &APIError{Status: 400, Code: "REQUIRED_FIELD_MISSING", Message: "External ID value is required", Fields: []string{"Compat_Key__c"}}
	}
	return it
}

// deleteItem finds the record's object from its key prefix.
func deleteItem(p Principal, id string) collectionsItem {
	malformed := collectionsItem{err: &APIError{Status: 400, Code: "MALFORMED_ID", Message: "Malformed record ID"}}
	if len(id) != 15 && len(id) != 18 {
		return malformed
	}
	object, ok := p.Catalog.ObjectByPrefix(id[:3])
	if !ok || !records.Readable(p, object.Name) { // D33-02: hidden reads as unknown
		return malformed
	}
	normalized, err := records.NormalizeID(p, object.Name, id)
	if err != nil {
		return malformed
	}
	return collectionsItem{object: object.Name, id: normalized}
}

func (s *Store) runItem(ctx context.Context, p Principal, kind routeKind, it collectionsItem) (map[string]any, error) {
	if it.err != nil {
		return nil, it.err
	}
	switch kind {
	case routeCollCreate:
		id, err := s.Create(ctx, p, it.object, it.data)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "success": true, "errors": []any{}}, nil
	case routeCollUpdate:
		if err := s.Update(ctx, p, it.object, it.id, it.data); err != nil {
			return nil, err
		}
		id, _ := records.NormalizeID(p, it.object, it.id)
		return map[string]any{"id": id, "success": true, "errors": []any{}}, nil
	case routeCollUpsert:
		id, created, err := s.Upsert(ctx, p, it.key, it.data)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "success": true, "errors": []any{}, "created": created}, nil
	case routeCollDelete:
		if err := s.Delete(ctx, p, it.object, it.id); err != nil {
			return nil, err
		}
		return map[string]any{"id": it.id, "success": true, "errors": []any{}}, nil
	}
	return nil, problem(404, "NOT_FOUND", "Resource not implemented by this candidate")
}

func itemFailure(api *APIError) map[string]any {
	fields := api.Fields
	if fields == nil {
		fields = []string{}
	}
	return map[string]any{"success": false, "errors": []any{map[string]any{"statusCode": api.Code, "message": api.Message, "fields": fields}}}
}

func rolledBack(id string) map[string]any {
	out := map[string]any{"success": false, "errors": []any{map[string]any{
		"statusCode": "ALL_OR_NONE_OPERATION_ROLLED_BACK",
		"message":    "Record rolled back because not all records were valid",
		"fields":     []string{},
	}}}
	if id != "" {
		out["id"] = id
	}
	return out
}

// collectionsWrite runs every item. All-or-none uses one savepoint for the
// batch; otherwise each item has its own.
func (s *Store) collectionsWrite(ctx context.Context, p Principal, kind routeKind, items []collectionsItem, allOrNone, force bool) (int, any, error) {
	results := make([]any, len(items))
	if !allOrNone {
		for i, it := range items {
			res, api, err := s.itemInSavepoint(ctx, p, kind, it)
			if err != nil {
				return 0, nil, err
			}
			if api != nil {
				results[i] = itemFailure(api)
			} else {
				results[i] = res
			}
		}
		return 200, results, nil
	}
	sp, err := s.tx.Begin(ctx)
	if err != nil {
		return 0, nil, errStorage
	}
	inner := *s
	inner.tx = sp
	written := make([]string, len(items))
	for i, it := range items {
		res, runErr := inner.runItem(ctx, p, kind, it)
		if runErr == nil {
			written[i], _ = res["id"].(string)
			results[i] = res
			continue
		}
		api, storage := itemError(ctx, runErr)
		if rbErr := sp.Rollback(ctx); rbErr != nil || storage != nil {
			return 0, nil, errStorage
		}
		for j := range items {
			if j == i {
				results[j] = itemFailure(api)
			} else {
				results[j] = rolledBack(written[j])
			}
		}
		if force {
			return 400, results, errCollectionsFailed
		}
		return 200, results, nil
	}
	if err := sp.Commit(ctx); err != nil {
		return 0, nil, errStorage
	}
	return 200, results, nil
}

func (s *Store) itemInSavepoint(ctx context.Context, p Principal, kind routeKind, it collectionsItem) (map[string]any, *APIError, error) {
	if it.err != nil {
		return nil, it.err, nil
	}
	sp, err := s.tx.Begin(ctx)
	if err != nil {
		return nil, nil, errStorage
	}
	inner := *s
	inner.tx = sp
	res, runErr := inner.runItem(ctx, p, kind, it)
	if runErr != nil {
		api, storage := itemError(ctx, runErr)
		if rbErr := sp.Rollback(ctx); rbErr != nil || storage != nil {
			return nil, nil, errStorage
		}
		return nil, api, nil
	}
	if err := sp.Commit(ctx); err != nil {
		return nil, nil, errStorage
	}
	return res, nil, nil
}

// collectionsRetrieve returns one entry per requested ID, null where the
// record is malformed, of another object, missing or unreadable.
func (s *Store) collectionsRetrieve(ctx context.Context, p Principal, object string, ids, fields []string) ([]any, error) {
	definition, err := p.Catalog.Object(object)
	if err != nil || !records.Readable(p, definition.Name) { // D33-02
		return nil, problem(404, "NOT_FOUND", "Object not found")
	}
	object = definition.Name
	names, err := records.SelectFields(p, object, fields)
	if err != nil {
		return nil, err
	}
	normalized := make([]string, len(ids))
	var unique []string
	seen := map[string]bool{}
	for i, id := range ids {
		n, err := records.NormalizeID(p, object, id)
		if err != nil {
			continue
		}
		normalized[i] = n
		if !seen[n] {
			seen[n] = true
			unique = append(unique, n)
		}
	}
	found := map[string]any{}
	for start := 0; start < len(unique); start += retrieveChunk {
		end := min(start+retrieveChunk, len(unique))
		quoted := make([]string, 0, end-start)
		for _, id := range unique[start:end] {
			quoted = append(quoted, "'"+id+"'")
		}
		text := "SELECT " + strings.Join(names, ",") + " FROM " + object + " WHERE Id IN (" + strings.Join(quoted, ",") + ")"
		res, err := s.recordTx(p).Query(ctx, text, soqleng.Options{BatchSize: collectionsDefaultPage})
		if err != nil {
			return nil, err
		}
		for _, m := range res.Records {
			if id, ok := m["Id"].(string); ok {
				found[id] = m
			}
		}
	}
	out := make([]any, len(ids))
	for i, n := range normalized {
		if n != "" {
			out[i] = found[n]
		}
	}
	return out, nil
}
