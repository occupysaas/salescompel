package platform

import (
	"context"
	"strings"

	soqleng "salescompel/internal/soql"
)

func (s *Store) Create(ctx context.Context, p Principal, object string, data map[string]any) (string, error) {
	var id string
	e := s.within(ctx, p, func(s *Store, p Principal) error {
		var e error
		id, e = s.recordTx(p).Create(ctx, object, data)
		return e
	})
	return id, e
}
func (s *Store) Update(ctx context.Context, p Principal, object, id string, data map[string]any) error {
	return s.within(ctx, p, func(s *Store, p Principal) error { return s.recordTx(p).Update(ctx, object, id, data) })
}
func (s *Store) Delete(ctx context.Context, p Principal, object, id string) error {
	return s.within(ctx, p, func(s *Store, p Principal) error { return s.recordTx(p).Delete(ctx, object, id) })
}
func (s *Store) Read(ctx context.Context, p Principal, object, id string) (map[string]any, error) {
	var out map[string]any
	e := s.within(ctx, p, func(s *Store, p Principal) error { var e error; out, e = s.recordTx(p).Read(ctx, object, id); return e })
	return out, e
}
func (s *Store) Select(ctx context.Context, p Principal, q Query) ([]map[string]any, error) {
	var out []map[string]any
	e := s.within(ctx, p, func(s *Store, p Principal) error { var e error; out, e = s.recordTx(p).Select(ctx, q); return e })
	return out, e
}

// Query executes SOQL text through the engine and returns the REST result shape.
func (s *Store) Query(ctx context.Context, p Principal, text string, batch int) (soqleng.Result, error) {
	var out soqleng.Result
	e := s.within(ctx, p, func(s *Store, p Principal) error {
		var e error
		out, e = s.recordTx(p).Query(ctx, text, soqleng.Options{BatchSize: batch})
		return e
	})
	return out, e
}
func (s *Store) Upsert(ctx context.Context, p Principal, key string, data map[string]any) (string, bool, error) {
	for name, v := range data {
		if strings.EqualFold(name, "Compat_Key__c") && v != nil {
			switch v.(type) {
			case map[string]any, []any:
				return "", false, invalid("JSON_PARSER_ERROR", "External ID requires a scalar value or null")
			default:
				return "", false, invalid("INVALID_FIELD", "External ID value must be supplied in the upsert URL")
			}
		}
	}

	var id string
	var created bool
	e := s.within(ctx, p, func(s *Store, p Principal) error {
		var e error
		id, created, e = s.recordTx(p).UpsertAccount(ctx, key, data)
		return e
	})
	return id, created, e
}
func (s *Store) Composite(ctx context.Context, p Principal, r CompositeRequest) (CompositeResponse, error) {
	var out CompositeResponse
	e := s.within(ctx, p, func(s *Store, p Principal) error { var e error; out, e = s.scopedComposite(ctx, p, r); return e })
	return out, e
}
