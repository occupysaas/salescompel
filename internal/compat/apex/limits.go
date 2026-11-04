package apex

import "context"

type Limits struct{ SourceBytes, ParseDepth, Steps, CallDepth, Rows, Writes, StringBytes, CollectionItems int }
type Usage struct{ Steps, Rows, Writes int }

func DefaultLimits() Limits { return Limits{32768, 64, 10000, 2, 100, 100, 4096, 100} }

type meter struct {
	limits   Limits
	usage    Usage
	depth    int
	observer func(string)
}

func newMeter(l Limits) (*meter, error) {
	for _, n := range []int{l.SourceBytes, l.ParseDepth, l.Steps, l.CallDepth, l.Rows, l.Writes, l.StringBytes, l.CollectionItems} {
		if n <= 0 {
			return nil, diagnostic("invalid-input")
		}
	}
	return &meter{limits: l}, nil
}
func (m *meter) event(label string) {
	if m.observer != nil {
		m.observer(label)
	}
}
func (m *meter) checkpoint(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return diagnostic("cancelled")
	}
	return charge(&m.usage.Steps, 1, m.limits.Steps)
}
func charge(used *int, n, limit int) error {
	if n < 0 {
		return diagnostic("invalid-input")
	}
	if n > limit-*used {
		return diagnostic("budget")
	}
	*used += n
	return nil
}
func (m *meter) chargeRows(n int) error   { return charge(&m.usage.Rows, n, m.limits.Rows) }
func (m *meter) chargeWrites(n int) error { return charge(&m.usage.Writes, n, m.limits.Writes) }
func (m *meter) enter() error             { return charge(&m.depth, 1, m.limits.CallDepth) }
func (m *meter) leave()                   { m.depth-- }
