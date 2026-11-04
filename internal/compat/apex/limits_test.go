package apex

import (
	"context"
	"math"
	"testing"
)

func TestMeterBoundaries(t *testing.T) {
	l := DefaultLimits()
	l.Steps = 2
	l.Rows = 2
	l.Writes = 2
	l.CallDepth = 2
	m, e := newMeter(l)
	wantCode(t, e, "")
	for i := 0; i < 2; i++ {
		wantCode(t, m.checkpoint(context.Background()), "")
	}
	wantCode(t, m.checkpoint(context.Background()), "budget")
	wantCode(t, m.chargeRows(2), "")
	wantCode(t, m.chargeRows(1), "budget")
	wantCode(t, m.chargeWrites(2), "")
	wantCode(t, m.chargeWrites(math.MaxInt), "budget")
	wantCode(t, m.chargeRows(-1), "invalid-input")
	wantCode(t, m.enter(), "")
	wantCode(t, m.enter(), "")
	wantCode(t, m.enter(), "budget")
	m.leave()
	wantCode(t, m.enter(), "")
	other, e := newMeter(l)
	wantCode(t, e, "")
	wantCode(t, other.checkpoint(context.Background()), "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wantCode(t, other.checkpoint(ctx), "cancelled")
	if other.usage.Steps != 1 {
		t.Fatal("cancel consumed step")
	}
	for _, v := range []int{0, -1} {
		for i := 0; i < 8; i++ {
			bad := DefaultLimits()
			fields := []*int{&bad.SourceBytes, &bad.ParseDepth, &bad.Steps, &bad.CallDepth, &bad.Rows, &bad.Writes, &bad.StringBytes, &bad.CollectionItems}
			*fields[i] = v
			_, e := newMeter(bad)
			wantCode(t, e, "invalid-input")
		}
	}
}
