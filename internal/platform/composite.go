package platform

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type CompositeRequest struct {
	AllOrNone          bool                  `json:"allOrNone"`
	CollateSubrequests *bool                 `json:"collateSubrequests,omitempty"`
	CompositeRequest   []CompositeSubrequest `json:"compositeRequest"`
}
type CompositeSubrequest struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	ReferenceID string         `json:"referenceId"`
	Body        map[string]any `json:"body,omitempty"`
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

func (s *Store) Composite(ctx context.Context, p Principal, req CompositeRequest) (CompositeResponse, error) {
	return executeComposite(ctx, p, req, func(ctx context.Context) (pgx.Tx, error) { return s.pool.Begin(ctx) })
}
func executeComposite(ctx context.Context, p Principal, req CompositeRequest, begin func(context.Context) (pgx.Tx, error)) (CompositeResponse, error) {
	empty := CompositeResponse{}
	unavailable := func() (CompositeResponse, error) {
		return empty, problem(500, "SERVER_UNAVAILABLE", "Storage operation failed")
	}
	if err := validateComposite(req); err != nil {
		return empty, err
	}
	tx, err := begin(ctx)
	if err != nil {
		return unavailable()
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
	ids := map[string]string{}
	out := CompositeResponse{CompositeResponse: make([]CompositeSubresponse, len(req.CompositeRequest))}
	for i, sub := range req.CompositeRequest {
		route := compositeRoute.FindStringSubmatch(sub.URL)
		object, id := route[1], route[2]
		if ref := compositeReference.FindStringSubmatch(id); ref != nil {
			id = ids[ref[1]]
		}
		data := make(map[string]any, len(sub.Body))
		for k, v := range sub.Body {
			data[k] = v
		}
		for k, value := range data {
			if v, ok := value.(string); ok && strings.EqualFold(k, "AccountId") {
				if ref := compositeReference.FindStringSubmatch(v); ref != nil {
					data[k] = ids[ref[1]]
				}
			}
		}
		result := CompositeSubresponse{ReferenceID: sub.ReferenceID, HTTPHeaders: map[string]string{}, HTTPStatusCode: 204}
		switch sub.Method {
		case "POST":
			id, err = create(ctx, tx, p, object, data)
			if err == nil {
				ids[sub.ReferenceID] = id
				result.HTTPStatusCode = 201
				result.Body = map[string]any{"id": id, "success": true, "errors": []any{}}
			}
		case "PATCH":
			err = update(ctx, tx, p, object, id, data)
		case "DELETE":
			err = deleteRecord(ctx, tx, p, object, id)
		}
		if err != nil || ctx.Err() != nil {
			rbErr := rollback()
			var api *APIError
			if rbErr != nil || ctx.Err() != nil || !errors.As(err, &api) || api.Status >= 500 {
				return unavailable()
			}
			for j, original := range req.CompositeRequest {
				failure := &APIError{Status: 400, Code: "PROCESSING_HALTED", Message: "Composite transaction rolled back"}
				if j == i {
					failure = api
				}
				out.CompositeResponse[j] = CompositeSubresponse{ReferenceID: original.ReferenceID, HTTPHeaders: map[string]string{}, HTTPStatusCode: failure.Status, Body: []*APIError{failure}}
			}
			return out, nil
		}
		out.CompositeResponse[i] = result
	}
	if err := tx.Commit(ctx); err != nil {
		_ = rollback()
		return unavailable()
	}
	closed = true
	return out, nil
}
