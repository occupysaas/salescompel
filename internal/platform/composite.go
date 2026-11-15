package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/records"
)

type CompositeRequest struct {
	AllOrNone          bool                  `json:"allOrNone"`
	CollateSubrequests *bool                 `json:"collateSubrequests,omitempty"`
	CompositeRequest   []CompositeSubrequest `json:"compositeRequest"`
}
type CompositeSubrequest struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	ReferenceID string            `json:"referenceId"`
	Body        map[string]any    `json:"body,omitempty"`
	HTTPHeaders map[string]string `json:"httpHeaders,omitempty"`
}
type CompositeResponse struct {
	CompositeResponse []CompositeSubresponse `json:"compositeResponse"`
}
type CompositeSubresponse struct {
	Body           any               `json:"body"`
	HTTPHeaders    map[string]string `json:"httpHeaders"`
	HTTPStatusCode int               `json:"httpStatusCode"`
	ReferenceID    string            `json:"referenceId"`
}

func (s *Store) scopedComposite(ctx context.Context, p Principal, req CompositeRequest) (CompositeResponse, error) {
	return executeComposite(ctx, s, p, req, func(ctx context.Context) (pgx.Tx, error) { return s.begin(ctx) })
}

// errStorage marks a failure that must roll back the whole request.
var errStorage = errors.New("composite storage failure")

func compositeUnavailable() error {
	return problem(500, "SERVER_UNAVAILABLE", "Storage operation failed")
}
func refHalted() *APIError {
	return &APIError{Status: 400, Code: "PROCESSING_HALTED", Message: "Invalid reference specified"}
}

// itemError returns the APIError for an item failure, or errStorage when err
// is a storage failure. spec: cmp-01
func itemError(ctx context.Context, err error) (*APIError, error) {
	var api *APIError
	if ctx.Err() != nil || !errors.As(err, &api) || api.Status >= 500 {
		return nil, errStorage
	}
	return api, nil
}

// executeComposite runs every subrequest in one transaction. spec: cmp-02
func executeComposite(ctx context.Context, s *Store, p Principal, req CompositeRequest, begin func(context.Context) (pgx.Tx, error)) (CompositeResponse, error) {
	empty := CompositeResponse{}
	if p.Catalog == nil {
		return empty, compositeUnavailable()
	}
	plans, err := planComposite(req)
	if err != nil {
		return empty, err
	}
	if err := prevalidateRetrieveFields(p, req, plans); err != nil {
		return empty, err
	}
	tx, err := begin(ctx)
	if err != nil {
		return empty, compositeUnavailable()
	}
	closed := false
	rollback := func() error {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		closed = true
		err := tx.Rollback(cleanup)
		if errors.Is(err, pgx.ErrTxClosed) {
			return nil
		}
		return err
	}
	defer func() {
		if !closed {
			_ = rollback()
		}
	}()
	st := *s
	st.tx = tx
	bodies := map[string]any{}
	out := CompositeResponse{CompositeResponse: make([]CompositeSubresponse, len(req.CompositeRequest))}
	for i, sub := range req.CompositeRequest {
		var status int
		var body any
		var api *APIError
		if req.AllOrNone {
			status, body, api, err = runSubrequest(ctx, &st, p, sub, plans[i], bodies, true)
		} else {
			status, body, api, err = runInSavepoint(ctx, &st, p, sub, plans[i], bodies)
		}
		if err != nil || ctx.Err() != nil {
			_ = rollback()
			return empty, compositeUnavailable()
		}
		if api != nil && req.AllOrNone {
			if rbErr := rollback(); rbErr != nil {
				return empty, compositeUnavailable()
			}
			return haltAll(req, i, status, body, api), nil
		}
		entry := CompositeSubresponse{ReferenceID: sub.ReferenceID, HTTPHeaders: map[string]string{}, HTTPStatusCode: status, Body: body}
		if api != nil {
			entry.Body = []*APIError{api}
		} else if body != nil {
			normalized, ok := normalizeJSON(body)
			if !ok {
				_ = rollback()
				return empty, compositeUnavailable()
			}
			bodies[sub.ReferenceID] = normalized
		} else {
			bodies[sub.ReferenceID] = nil
		}
		out.CompositeResponse[i] = entry
	}
	if err := tx.Commit(ctx); err != nil {
		_ = rollback()
		return empty, compositeUnavailable()
	}
	closed = true
	return out, nil
}

// haltAll builds the all-or-none failure response: the failing entry keeps
// its error (or, for a Collections subrequest, its item array) and every other
// entry becomes PROCESSING_HALTED.
func haltAll(req CompositeRequest, failed, status int, body any, api *APIError) CompositeResponse {
	out := CompositeResponse{CompositeResponse: make([]CompositeSubresponse, len(req.CompositeRequest))}
	for j, original := range req.CompositeRequest {
		entry := CompositeSubresponse{ReferenceID: original.ReferenceID, HTTPHeaders: map[string]string{}, HTTPStatusCode: 400}
		switch {
		case j != failed:
			entry.Body = []*APIError{{Status: 400, Code: "PROCESSING_HALTED", Message: "Composite transaction rolled back"}}
		case body != nil && status == 400:
			entry.Body = body // Collections item array, already rolled back
		default:
			entry.HTTPStatusCode = api.Status
			entry.Body = []*APIError{api}
		}
		out.CompositeResponse[j] = entry
	}
	return out
}

// runInSavepoint runs one subrequest so that an item failure undoes only its
// own writes.
func runInSavepoint(ctx context.Context, st *Store, p Principal, sub CompositeSubrequest, plan subPlan, bodies map[string]any) (int, any, *APIError, error) {
	sp, err := st.tx.Begin(ctx)
	if err != nil {
		return 0, nil, nil, errStorage
	}
	inner := *st
	inner.tx = sp
	status, body, api, err := runSubrequest(ctx, &inner, p, sub, plan, bodies, false)
	if err != nil || api != nil {
		if rbErr := sp.Rollback(ctx); rbErr != nil || err != nil {
			return 0, nil, nil, errStorage
		}
		return status, body, api, nil
	}
	if err := sp.Commit(ctx); err != nil {
		return 0, nil, nil, errStorage
	}
	return status, body, nil, nil
}

// runSubrequest resolves references and dispatches one subrequest. It returns
// the success status and body, or an item error, or a storage error.
func runSubrequest(ctx context.Context, st *Store, p Principal, sub CompositeSubrequest, plan subPlan, bodies map[string]any, outerAllOrNone bool) (int, any, *APIError, error) {
	fail := func(err error) (int, any, *APIError, error) {
		api, storage := itemError(ctx, err)
		if storage != nil {
			return 0, nil, nil, storage
		}
		return api.Status, nil, api, nil
	}
	for _, ref := range plan.refs {
		if _, ok := bodies[ref.id]; !ok {
			return fail(refHalted())
		}
	}
	if plan.headerUnsupported {
		return fail(invalid("INVALID_COMPOSITE_REQUEST", "Unsupported subrequest header"))
	}
	route := plan.route
	if ref, isRef := route.value.single(); isRef {
		v, ok := resolve(ref, bodies)
		if !ok {
			return fail(refHalted())
		}
		if s, isString := v.(string); !isString || s == "" {
			return fail(invalid("INVALID_COMPOSITE_REQUEST", "Reference value cannot be used in the URL"))
		}
	}
	value, ok := renderText(route.value, bodies)
	if !ok {
		return fail(refHalted())
	}
	params := url.Values{}
	for k, t := range route.query {
		v, ok := renderText(t, bodies)
		if !ok {
			return fail(refHalted())
		}
		params.Set(k, v)
	}
	var data map[string]any
	if sub.Body != nil {
		substituted, ok := substituteBody(map[string]any(sub.Body), bodies)
		if !ok {
			return fail(refHalted())
		}
		data = substituted.(map[string]any)
	}
	switch route.kind {
	case routeUnsupported:
		return fail(problem(404, "NOT_FOUND", "Resource not implemented by this candidate"))
	case routeWrongMethod:
		return fail(problem(405, "METHOD_NOT_ALLOWED", "Unsupported method"))
	case routeQuery:
		res, err := st.restQuery(ctx, p, params.Get("q"), params, 2000)
		if err != nil {
			return fail(err)
		}
		return 200, res, nil, nil
	case routeQueryMore:
		res, err := st.restQueryMore(ctx, p, value, 2000)
		if err != nil {
			return fail(err)
		}
		return 200, res, nil, nil
	}
	if route.isCollections() {
		status, body, err := st.collections(ctx, p, route, params, data, outerAllOrNone)
		if errors.Is(err, errCollectionsFailed) {
			return 400, body, &APIError{Status: 400, Code: "PROCESSING_HALTED", Message: "Collections item failed"}, nil
		}
		if err != nil {
			return fail(err)
		}
		return status, body, nil, nil
	}
	definition, lookupErr := p.Catalog.Object(route.object)
	if lookupErr != nil || !records.Readable(p, definition.Name) { // D33-02
		return fail(problem(404, "NOT_FOUND", "Object not found"))
	}
	object := definition.Name
	switch route.kind {
	case routeCreate:
		id, err := st.Create(ctx, p, object, data)
		if err != nil {
			return fail(err)
		}
		return 201, map[string]any{"id": id, "success": true, "errors": []any{}}, nil, nil
	case routeRead:
		row, err := st.Read(ctx, p, object, value, restFields(params.Get("fields"))...)
		if err != nil {
			return fail(err)
		}
		return 200, row, nil, nil
	case routeUpdate:
		if err := st.Update(ctx, p, object, value, data); err != nil {
			return fail(err)
		}
		return 204, nil, nil, nil
	case routeDelete:
		if err := st.Delete(ctx, p, object, value); err != nil {
			return fail(err)
		}
		return 204, nil, nil, nil
	case routeUpsert:
		id, created, err := st.Upsert(ctx, p, value, data)
		if err != nil {
			return fail(err)
		}
		status := 200
		if created {
			status = 201
		}
		return status, map[string]any{"id": id, "created": created, "success": true, "errors": []any{}}, nil, nil
	}
	return fail(problem(404, "NOT_FOUND", "Resource not implemented by this candidate"))
}

// normalizeJSON converts a result body to plain JSON values so references
// resolve against exactly what the client sees.
func normalizeJSON(v any) (any, bool) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var out any
	return out, decodeExact(raw, &out) == nil
}
