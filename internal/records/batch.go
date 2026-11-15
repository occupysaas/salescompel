package records

import (
	"context"
	"sort"
)

// LockAccountKeys takes the per-key upsert locks for every valid key in
// sorted canonical order, so concurrent batches that share keys cannot
// deadlock. UpsertAccount takes the same locks again later in the same
// transaction, which PostgreSQL allows. Invalid keys are skipped; their
// upserts fail on their own. spec: col-01
func (s *Tx) LockAccountKeys(ctx context.Context, keys []string) error {
	canonical := make([]string, 0, len(keys))
	seen := map[string]bool{}
	for _, key := range keys {
		c, err := canonicalExternalID(key)
		if err != nil || c == "" || seen[c] {
			continue
		}
		seen[c] = true
		canonical = append(canonical, c)
	}
	sort.Strings(canonical)
	for _, c := range canonical {
		if _, err := s.db.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", s.principal.Tenant+"\x1f"+c); err != nil {
			return dbError(err)
		}
	}
	return nil
}

// NormalizeID checks id against object's key prefix and returns its 18
// character form.
func NormalizeID(p Principal, object, id string) (string, error) {
	return normalizeID(p, object, id)
}
