package apex

import (
	"context"
	"sort"
	"unicode/utf8"
)

type Account struct{ Tenant, ID, Cohort, Name string }
type rowKey struct{ tenant, id string }

// Store is a trusted synthetic harness store. Concurrent use of one Store is unsupported.
type Store struct{ accounts []Account }

func NewStore(accounts []Account) (*Store, error) {
	seen := map[rowKey]bool{}
	for _, a := range accounts {
		key := rowKey{a.Tenant, a.ID}
		if a.Tenant == "" || a.ID == "" || seen[key] {
			return nil, diagnostic("invalid-input")
		}
		for _, s := range []string{a.Tenant, a.ID, a.Cohort, a.Name} {
			if !utf8.ValidString(s) {
				return nil, diagnostic("invalid-input")
			}
		}
		seen[key] = true
	}
	s := &Store{accounts: append([]Account{}, accounts...)}
	sort.Slice(s.accounts, func(i, j int) bool {
		a, b := s.accounts[i], s.accounts[j]
		if a.Tenant != b.Tenant {
			return a.Tenant < b.Tenant
		}
		return a.ID < b.ID
	})
	return s, nil
}
func (s *Store) Snapshot() []Account { return append([]Account{}, s.accounts...) }

type recordHandle struct {
	owner *transaction
	key   rowKey
}
type record struct {
	handle  *recordHandle
	account Account
}
type transaction struct {
	store             *Store
	identity          Identity
	policy            Policy
	meter             *meter
	snapshot          []Account
	staged            map[rowKey]string
	issued            map[*recordHandle]rowKey
	finalized, failed bool
}

func validIdentity(id Identity) bool { return id.Tenant != "" && id.Principal != "" }
func (s *Store) begin(id Identity, p Policy, m *meter) (*transaction, error) {
	if s == nil || m == nil || !validIdentity(id) {
		return nil, diagnostic("invalid-input")
	}
	if p.Identity != id {
		return nil, diagnostic("denied")
	}
	p.DeniedWriteIDs = append([]string{}, p.DeniedWriteIDs...)
	tx := &transaction{store: s, identity: id, policy: p, meter: m, snapshot: s.Snapshot(), staged: map[rowKey]string{}, issued: map[*recordHandle]rowKey{}}
	m.event("begin")
	return tx, nil
}
func (tx *transaction) finishError(err error) {
	if err != nil {
		tx.failed = true
	}
}
func (tx *transaction) start(ctx context.Context) error {
	if tx.finalized || tx.failed {
		return diagnostic("runtime")
	}
	if e := tx.meter.checkpoint(ctx); e != nil {
		return e
	}
	return tx.meter.enter()
}
func (tx *transaction) query(ctx context.Context, cohort string) (records []record, err error) {
	defer func() { tx.finishError(err) }()
	if err = tx.start(ctx); err != nil {
		return nil, err
	}
	defer tx.meter.leave()
	tx.meter.event("query")
	if !tx.policy.QueryAllowed {
		return nil, diagnostic("denied")
	}
	if len(cohort) > tx.meter.limits.StringBytes {
		return nil, diagnostic("budget")
	}
	for _, a := range tx.snapshot {
		if err = tx.meter.checkpoint(ctx); err != nil {
			return nil, err
		}
		if a.Tenant != tx.identity.Tenant || a.Cohort != cohort {
			continue
		}
		key := rowKey{a.Tenant, a.ID}
		if name, ok := tx.staged[key]; ok {
			a.Name = name
		}
		for _, v := range []string{a.ID, a.Cohort, a.Name} {
			if len(v) > tx.meter.limits.StringBytes {
				return nil, diagnostic("budget")
			}
		}
		if len(records) >= tx.meter.limits.CollectionItems {
			return nil, diagnostic("budget")
		}
		if err = tx.meter.chargeRows(1); err != nil {
			return nil, err
		}
		h := &recordHandle{owner: tx, key: key}
		tx.issued[h] = key
		records = append(records, record{handle: h, account: a})
	}
	return records, nil
}
func (tx *transaction) stage(ctx context.Context, records []record) (err error) {
	defer func() { tx.finishError(err) }()
	if err = tx.start(ctx); err != nil {
		return err
	}
	defer tx.meter.leave()
	tx.meter.event("stage")
	if !tx.policy.NameWriteAllowed {
		return diagnostic("denied")
	}
	if len(records) > tx.meter.limits.CollectionItems {
		return diagnostic("budget")
	}
	if err = tx.meter.chargeWrites(len(records)); err != nil {
		return err
	}
	seen := map[rowKey]bool{}
	for _, r := range records {
		if err = tx.meter.checkpoint(ctx); err != nil {
			return err
		}
		key, ok := tx.issued[r.handle]
		if !ok || r.handle.owner != tx || r.handle.key != key || key.tenant != tx.identity.Tenant || r.account.Tenant != key.tenant || r.account.ID != key.id || seen[key] {
			return diagnostic("denied")
		}
		seen[key] = true
		for _, id := range tx.policy.DeniedWriteIDs {
			if id == key.id {
				return diagnostic("denied")
			}
		}
		if len(r.account.Name) > tx.meter.limits.StringBytes {
			return diagnostic("budget")
		}
		if !utf8.ValidString(r.account.Name) {
			return diagnostic("type")
		}
	}
	for _, r := range records {
		tx.staged[rowKey{r.account.Tenant, r.account.ID}] = r.account.Name
	}
	tx.meter.event("staged")
	return nil
}
func (tx *transaction) commit(ctx context.Context) (err error) {
	defer func() { tx.finishError(err) }()
	if err = tx.start(ctx); err != nil {
		return err
	}
	defer tx.meter.leave()
	// Snapshot size is a trusted harness allocation, not an evaluated-program heap budget.
	next := append([]Account{}, tx.snapshot...)
	for i, a := range next {
		if err = tx.meter.checkpoint(ctx); err != nil {
			return err
		}
		if name, ok := tx.staged[rowKey{a.Tenant, a.ID}]; ok {
			next[i].Name = name
		}
	}
	tx.meter.event("before-commit")
	if err = tx.meter.checkpoint(ctx); err != nil {
		return err
	}
	tx.store.accounts = next
	tx.finalized = true
	tx.meter.event("commit")
	tx.staged = nil
	tx.issued = nil
	return nil
}
func (tx *transaction) discard() {
	if !tx.finalized {
		tx.meter.event("discard")
	}
	tx.finalized = true
	tx.staged = nil
	tx.issued = nil
}
