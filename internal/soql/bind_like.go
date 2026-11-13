package soql

import (
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

// typeLike types a LIKE operand from the literal's pattern encoding, keeping
// \% and \_ escaped while a doubled literal backslash stays distinguishable.
// Id patterns keep the same normalization as every other Id string operand.
// spec: soql-03
func (b *binder) typeLike(owner metadata.Object, f metadata.Field, lit Literal) (Param, error) {
	if lit.Kind != LitString {
		return Param{}, wrongType(f)
	}
	pattern := lit.Pattern
	if pattern == "" {
		pattern = lit.Text
	}
	if f.Kind() == fieldtype.Id {
		return b.idParam(owner, f, pattern)
	}
	return Param{Value: pattern, Cast: "text"}, nil
}
