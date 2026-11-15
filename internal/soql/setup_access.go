package soql

import (
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
)

// object resolves an object named in FROM, a parent traversal or a child
// subquery. An object the principal can't read, a setup object included
// (D33-09), behaves as an unknown object: D33-01, INVALID_TYPE. Field read
// is enforced by policy.Allowed in resolveField and resolveRel.
func (b *binder) object(name string) (metadata.Object, error) {
	o, err := b.snap.Object(name)
	if err != nil {
		return o, err
	}
	if !b.p.Perms.Object(o.Name).Read {
		return metadata.Object{}, contract.Invalid("INVALID_TYPE", "Unsupported object")
	}
	return o, nil
}
