package apex

import (
	"context"
	"reflect"
	"testing"
)

func hostRows() []Account {
	return []Account{{"b", "1", "g", "other"}, {"a", "2", "g", "two"}, {"a", "1", "g", "one"}, {"a", "3", "G", "case"}}
}
func hostPolicy() Policy {
	return Policy{Identity: Identity{"a", "writer"}, QueryAllowed: true, NameWriteAllowed: true}
}
func makeStore(t *testing.T, rows []Account) *Store {
	t.Helper()
	s, e := NewStore(rows)
	wantCode(t, e, "")
	return s
}
func beginTest(t *testing.T, s *Store, p Policy, l Limits) *transaction {
	t.Helper()
	m, e := newMeter(l)
	wantCode(t, e, "")
	tx, e := s.begin(p.Identity, p, m)
	wantCode(t, e, "")
	return tx
}
func sameStore(t *testing.T, s *Store, want []Account) {
	t.Helper()
	if !reflect.DeepEqual(s.Snapshot(), want) {
		t.Fatalf("store changed: %+v want %+v", s.Snapshot(), want)
	}
}
func TestHostTenantAndPolicy(t *testing.T) {
	ctx := context.Background()
	s := makeStore(t, hostRows())
	p := hostPolicy()
	tx := beginTest(t, s, p, DefaultLimits())
	r, e := tx.query(ctx, "g")
	wantCode(t, e, "")
	if len(r) != 2 || r[0].account.ID != "1" || r[1].account.ID != "2" || r[0].account.Tenant != "a" {
		t.Fatal("tenant/order/cohort failure", r)
	}
	p.QueryAllowed = false
	tx = beginTest(t, s, p, DefaultLimits())
	_, e = tx.query(ctx, "g")
	wantCode(t, e, "denied")
	p = hostPolicy()
	p.NameWriteAllowed = false
	tx = beginTest(t, s, p, DefaultLimits())
	r, e = tx.query(ctx, "g")
	wantCode(t, e, "")
	wantCode(t, tx.stage(ctx, r), "denied")
	m, _ := newMeter(DefaultLimits())
	_, e = s.begin(Identity{"a", "different"}, hostPolicy(), m)
	wantCode(t, e, "denied")
	_, e = s.begin(Identity{}, hostPolicy(), m)
	wantCode(t, e, "invalid-input")
	for _, rows := range [][]Account{{{"", "x", "", ""}}, {{"a", "", "", ""}}, {{"a", "x", "", ""}, {"a", "x", "", ""}}} {
		_, e := NewStore(rows)
		wantCode(t, e, "invalid-input")
	}
}
func TestHostHandles(t *testing.T) {
	ctx := context.Background()
	s := makeStore(t, hostRows())
	for _, kind := range []string{"unknown", "cross-tenant", "previous"} {
		t.Run(kind, func(t *testing.T) {
			tx := beginTest(t, s, hostPolicy(), DefaultLimits())
			r, e := tx.query(ctx, "g")
			wantCode(t, e, "")
			bad := r[0]
			switch kind {
			case "unknown":
				bad.handle = nil
			case "cross-tenant":
				bad.account.Tenant = "b"
			case "previous":
				tx.discard()
				tx = beginTest(t, s, hostPolicy(), DefaultLimits())
			}
			wantCode(t, tx.stage(ctx, []record{bad}), "denied")
			wantCode(t, tx.commit(ctx), "runtime")
		})
	}
}
func TestHostSnapshotIsolation(t *testing.T) {
	ctx := context.Background()
	input := hostRows()
	s := makeStore(t, input)
	before := s.Snapshot()
	input[0].Name = "input"
	snap := s.Snapshot()
	snap[0].Name = "snapshot"
	tx := beginTest(t, s, hostPolicy(), DefaultLimits())
	r, e := tx.query(ctx, "g")
	wantCode(t, e, "")
	r[0].account.Name = "detached"
	sameStore(t, s, before)
	tx.discard()
}
func TestHostCommitDiscard(t *testing.T) {
	ctx := context.Background()
	s := makeStore(t, hostRows())
	before := s.Snapshot()
	tx := beginTest(t, s, hostPolicy(), DefaultLimits())
	r, e := tx.query(ctx, "g")
	wantCode(t, e, "")
	r[0].account.Name = "changed"
	wantCode(t, tx.stage(ctx, r[:1]), "")
	again, e := tx.query(ctx, "g")
	wantCode(t, e, "")
	if again[0].account.Name != "changed" {
		t.Fatal("no read-your-writes")
	}
	sameStore(t, s, before)
	tx.discard()
	tx.discard()
	sameStore(t, s, before)
	wantCode(t, tx.commit(ctx), "runtime")
	_, e = tx.query(ctx, "g")
	wantCode(t, e, "runtime")
	tx = beginTest(t, s, hostPolicy(), DefaultLimits())
	r, e = tx.query(ctx, "g")
	wantCode(t, e, "")
	r[0].account.Name = "changed"
	wantCode(t, tx.stage(ctx, r[:1]), "")
	wantCode(t, tx.commit(ctx), "")
	before[0].Name = "changed"
	sameStore(t, s, before)
	wantCode(t, tx.stage(ctx, r), "runtime")
	wantCode(t, tx.commit(ctx), "runtime")
}
func TestHostBulkFailure(t *testing.T) {
	ctx := context.Background()
	s := makeStore(t, hostRows())
	before := s.Snapshot()
	p := hostPolicy()
	p.DeniedWriteIDs = []string{"2"}
	tx := beginTest(t, s, p, DefaultLimits())
	p.DeniedWriteIDs[0] = "1"
	r, e := tx.query(ctx, "g")
	wantCode(t, e, "")
	r[0].account.Name = "first"
	wantCode(t, tx.stage(ctx, r[:1]), "")
	r[0].account.Name = "second"
	r[1].account.Name = "denied"
	wantCode(t, tx.stage(ctx, r), "denied")
	wantCode(t, tx.commit(ctx), "runtime")
	tx.discard()
	sameStore(t, s, before)
}
func TestHostBudgets(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"rows", "writes", "collection", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			s := makeStore(t, hostRows())
			before := s.Snapshot()
			l := DefaultLimits()
			if kind == "rows" {
				l.Rows = 2
			}
			if kind == "writes" {
				l.Writes = 2
			}
			if kind == "collection" {
				l.CollectionItems = 1
			}
			tx := beginTest(t, s, hostPolicy(), l)
			r, e := tx.query(ctx, "g")
			if kind == "collection" {
				wantCode(t, e, "budget")
			} else {
				wantCode(t, e, "")
				switch kind {
				case "rows":
					_, e = tx.query(ctx, "g")
					wantCode(t, e, "budget")
				case "writes":
					wantCode(t, tx.stage(ctx, r), "")
					wantCode(t, tx.stage(ctx, r[:1]), "budget")
				case "duplicate":
					wantCode(t, tx.stage(ctx, []record{r[0], r[0]}), "denied")
				}
			}
			tx.discard()
			sameStore(t, s, before)
		})
	}
	l := DefaultLimits()
	l.Rows = 2
	l.Writes = 2
	l.CollectionItems = 2
	s := makeStore(t, hostRows())
	tx := beginTest(t, s, hostPolicy(), l)
	r, e := tx.query(ctx, "g")
	wantCode(t, e, "")
	wantCode(t, tx.stage(ctx, r), "")
	wantCode(t, tx.commit(ctx), "")
}
