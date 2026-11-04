package platform

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func compSub(method, object, id, ref string, data map[string]any) CompositeSubrequest {
	u := apiBase + "sobjects/" + object
	if id != "" {
		u += "/" + id
	}
	return CompositeSubrequest{Method: method, URL: u, ReferenceID: ref, Body: data}
}
func compReq(subs ...CompositeSubrequest) CompositeRequest {
	return CompositeRequest{AllOrNone: true, CompositeRequest: subs}
}
func TestCompositeAtomic(t *testing.T) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()
	count := func() int {
		var n int
		if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE tenant_id=$1", a.Tenant).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	t.Run("linked-commit", func(t *testing.T) {
		r, e := s.Composite(ctx, a, compReq(compSub("POST", "Account", "", "a", map[string]any{"Name": "Parent"}), compSub("POST", "Contact", "", "c", map[string]any{"LastName": "Child", "AccountId": "@{a.id}"})))
		if e != nil {
			t.Fatal(e)
		}
		aid := r.CompositeResponse[0].Body.(map[string]any)["id"].(string)
		cid := r.CompositeResponse[1].Body.(map[string]any)["id"].(string)
		row, e := s.Read(ctx, a, "Contact", cid)
		if e != nil || row["AccountId"] != aid {
			t.Fatal("relationship not committed")
		}
	})
	t.Run("create-invalid-rollback", func(t *testing.T) {
		before := count()
		r, e := s.Composite(ctx, a, compReq(compSub("POST", "Account", "", "a", map[string]any{"Name": "Rollback"}), compSub("PATCH", "Account", "@{a.id}", "bad", map[string]any{"Bogus": true}), compSub("POST", "Account", "", "later", map[string]any{"Name": "Never"})))
		if e != nil || count() != before {
			t.Fatal("partial effects", e)
		}
		for i, x := range r.CompositeResponse {
			expected := "PROCESSING_HALTED"
			if i == 1 {
				expected = "INVALID_FIELD"
			}
			if x.HTTPStatusCode != 400 || x.Body.([]*APIError)[0].Code != expected {
				t.Fatal("wrong rollback result")
			}
		}
	})
	t.Run("update-delete-rollback", func(t *testing.T) {
		id, e := s.Create(ctx, a, "Account", map[string]any{"Name": "Original"})
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.Composite(ctx, a, compReq(compSub("PATCH", "Account", id, "p", map[string]any{"Name": "Changed"}), compSub("DELETE", "Account", id, "d", nil), compSub("POST", "Contact", "", "bad", map[string]any{})))
		row, re := s.Read(ctx, a, "Account", id)
		if e != nil || re != nil || row["Name"] != "Original" || len(r.CompositeResponse) != 3 {
			t.Fatal("rollback did not restore", e, re)
		}
	})
	t.Run("sql-failure", func(t *testing.T) {
		before := count()
		r, e := s.Composite(ctx, a, compReq(compSub("POST", "Account", "", "a", map[string]any{"Name": "SQL rollback"}), compSub("POST", "Contact", "", "c", map[string]any{"LastName": "BadFK", "AccountId": "001000000000000AAA"})))
		if e != nil || count() != before || r.CompositeResponse[1].Body.([]*APIError)[0].Code != "INVALID_CROSS_REFERENCE_KEY" {
			t.Fatal("SQL rollback failed", e)
		}
	})
	t.Run("restricted", func(t *testing.T) {
		p := a
		p.Role = "restricted"
		before := count()
		r, e := s.Composite(ctx, p, compReq(compSub("POST", "Account", "", "a", map[string]any{"Name": "Allowed"}), compSub("PATCH", "Account", "@{a.id}", "p", map[string]any{"Compat_Text__c": "Denied"})))
		if e != nil || count() != before || r.CompositeResponse[1].HTTPStatusCode != 400 {
			t.Fatal("policy rollback failed")
		}
	})
	t.Run("cross-tenant", func(t *testing.T) {
		id, e := s.Create(ctx, b, "Account", map[string]any{"Name": "Other"})
		if e != nil {
			t.Fatal(e)
		}
		for _, method := range []string{"PATCH", "DELETE"} {
			var data map[string]any
			if method == "PATCH" {
				data = map[string]any{"Name": "Stolen"}
			}
			before := count()
			r, e := s.Composite(ctx, a, compReq(compSub("POST", "Account", "", "a", map[string]any{"Name": "Rollback"}), compSub(method, "Account", id, "bad", data)))
			if e != nil || count() != before || r.CompositeResponse[1].HTTPStatusCode != 404 {
				t.Fatal("tenant isolation failed")
			}
		}
	})
	t.Run("ordered-delete", func(t *testing.T) {
		r, e := s.Composite(ctx, a, compReq(compSub("POST", "Account", "", "a", map[string]any{"Name": "Transient"}), compSub("PATCH", "Account", "@{a.id}", "p", map[string]any{"Name": "Changed"}), compSub("DELETE", "Account", "@{a.id}", "d", nil)))
		if e != nil {
			t.Fatal(e)
		}
		if r.CompositeResponse[1].HTTPStatusCode != 204 || r.CompositeResponse[1].Body != nil || r.CompositeResponse[2].Body != nil {
			t.Fatal("204 bodies")
		}
		id := r.CompositeResponse[0].Body.(map[string]any)["id"].(string)
		if _, e := s.Read(ctx, a, "Account", id); e == nil {
			t.Fatal("delete not committed")
		}
	})
	t.Run("prevalidation", func(t *testing.T) {
		before := count()
		r := compReq(compSub("POST", "Account", "", "a", map[string]any{"Name": "Never"}), compSub("GET", "Account", "", "bad", nil))
		if _, e := s.Composite(ctx, a, r); e == nil || count() != before {
			t.Fatal("prevalidation writes")
		}
	})
	t.Run("25-commits", func(t *testing.T) {
		r := CompositeRequest{AllOrNone: true}
		for i := 0; i < 25; i++ {
			r.CompositeRequest = append(r.CompositeRequest, compSub("POST", "Account", "", string(rune('a'+i)), map[string]any{"Name": "Batch"}))
		}
		out, e := s.Composite(ctx, a, r)
		if e != nil || len(out.CompositeResponse) != 25 {
			t.Fatal(e)
		}
		for _, v := range out.CompositeResponse {
			if _, e := s.Read(ctx, a, "Account", v.Body.(map[string]any)["id"].(string)); e != nil {
				t.Fatal(e)
			}
		}
	})
}

// Wrap real transactions: deterministic failure timing without global hooks or races.
type compositeTestTx struct {
	pgx.Tx
	cancel                   context.CancelFunc
	commitFail, rollbackFail bool
	clean                    bool
}

func (t *compositeTestTx) Commit(ctx context.Context) error {
	if t.commitFail {
		return errors.New("private_marker")
	}
	return t.Tx.Commit(ctx)
}
func (t *compositeTestTx) Rollback(ctx context.Context) error {
	t.clean = ctx.Err() == nil
	err := t.Tx.Rollback(ctx)
	if t.rollbackFail {
		return errors.New("private_marker")
	}
	return err
}
func (t *compositeTestTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, e := t.Tx.Exec(ctx, sql, args...)
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	return tag, e
}
func TestCompositeInfrastructure(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	for _, mode := range []string{"begin", "commit", "rollback", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wrapper *compositeTestTx
			begin := func(ctx context.Context) (pgx.Tx, error) {
				if mode == "begin" {
					return nil, errors.New("private_marker")
				}
				tx, e := s.pool.Begin(ctx)
				wrapper = &compositeTestTx{Tx: tx, commitFail: mode == "commit", rollbackFail: mode == "rollback"}
				if mode == "cancel" {
					wrapper.cancel = cancel
				}
				return wrapper, e
			}
			req := compReq(compSub("POST", "Account", "", "a", map[string]any{"Name": "Not committed"}), compSub("POST", "Contact", "", "bad", map[string]any{}))
			if mode == "commit" {
				req.CompositeRequest = req.CompositeRequest[:1]
			}
			out, e := executeComposite(ctx, a, req, begin)
			if e == nil || len(out.CompositeResponse) != 0 {
				t.Fatal("leaked successes")
			}
			if wrapper != nil && !wrapper.clean {
				t.Fatal("canceled rollback context")
			}
			var n int
			if e := s.pool.QueryRow(context.Background(), "SELECT count(*) FROM accounts WHERE tenant_id=$1 AND name='Not committed'", a.Tenant).Scan(&n); e != nil || n != 0 {
				t.Fatal("partial effects", e)
			}
		})
	}
}
