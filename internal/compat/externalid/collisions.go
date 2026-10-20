package externalid

import (
	"fmt"
	"sort"
)

// Record is a synthetic diagnostic input. A nil Key is null, distinct from empty.
type Record struct {
	Tenant string
	ID     string
	Key    *string
}

// Collision reports identities sharing a tenant and canonical key.
// It does not select a winner or prescribe any record mutation.
type Collision struct {
	Tenant    string
	Canonical string
	IDs       []string
}

// FindCollisions groups non-null keys under HypothesisVersion. Results and IDs
// are sorted lexicographically by tenant, canonical key and ID, respectively.
// Invalid records return a nil result; input records and keys are never mutated.
func FindCollisions(records []Record) ([]Collision, error) {
	type identity struct{ tenant, id string }
	type group struct{ tenant, canonical string }
	seen := make(map[identity]struct{}, len(records))
	groups := make(map[group][]string)
	for i, record := range records {
		if record.Tenant == "" || record.ID == "" {
			return nil, fmt.Errorf("record %d: tenant and ID must be nonempty", i)
		}
		id := identity{record.Tenant, record.ID}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("record %d: duplicate tenant/ID", i)
		}
		seen[id] = struct{}{}
		if record.Key == nil {
			continue
		}
		canonical, err := CanonicalKey(*record.Key)
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", i, err)
		}
		key := group{record.Tenant, canonical}
		groups[key] = append(groups[key], record.ID)
	}
	collisions := make([]Collision, 0)
	for key, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		ownedIDs := append([]string(nil), ids...)
		sort.Strings(ownedIDs)
		collisions = append(collisions, Collision{Tenant: key.tenant, Canonical: key.canonical, IDs: ownedIDs})
	}
	sort.Slice(collisions, func(i, j int) bool {
		if collisions[i].Tenant != collisions[j].Tenant {
			return collisions[i].Tenant < collisions[j].Tenant
		}
		return collisions[i].Canonical < collisions[j].Canonical
	})
	return collisions, nil
}
