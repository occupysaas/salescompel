package soql

import (
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
)

// object resolves an object named in FROM, a parent traversal or a child
// subquery. A setup object the principal can't read behaves as an unknown
// object (D33-09, answered as D33-01). Object and field read enforcement for
// every other object arrives with Task 5.
func (b *binder) object(name string) (metadata.Object, error) {
	o, err := b.snap.Object(name)
	if err != nil {
		return o, err
	}
	if metadata.IsSetupObject(o.Name) && !b.p.Perms.Object(o.Name).Read {
		return metadata.Object{}, contract.Invalid("INVALID_TYPE", "Unsupported object")
	}
	return o, nil
}
