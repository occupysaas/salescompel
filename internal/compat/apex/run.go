package apex

import (
	"context"
	"unicode/utf8"
)

type Request struct {
	Source, Entry string
	Args          []Value
	Identity      Identity
	Limits        Limits
}
type Result struct {
	Value Value
	Usage Usage
}

// Run executes the complete source and publishes effects only after success.
func Run(ctx context.Context, req Request, store *Store, policy Policy) (Result, error) {
	return run(ctx, req, store, policy, nil)
}
func run(ctx context.Context, req Request, store *Store, policy Policy, observer func(string)) (Result, error) {
	if ctx == nil || store == nil || !validIdentity(req.Identity) {
		return Result{}, diagnostic("invalid-input")
	}
	if policy.Identity != req.Identity {
		return Result{}, diagnostic("denied")
	}
	m, err := newMeter(req.Limits)
	if err != nil {
		return Result{}, err
	}
	m.observer = observer
	if ctx.Err() != nil {
		return Result{}, diagnostic("cancelled")
	}
	p, err := compile(req.Source, req.Limits)
	if err != nil {
		return Result{}, err
	}
	if req.Entry != p.class+".run" {
		return Result{}, diagnostic("invalid-input")
	}
	if len(req.Args) != len(p.params) {
		return Result{}, diagnostic("type")
	}
	for i, a := range req.Args {
		if a.kind == stringKind {
			if len(a.s) > req.Limits.StringBytes {
				return Result{}, diagnostic("budget")
			}
			if !utf8.ValidString(a.s) {
				return Result{}, diagnostic("invalid-input")
			}
		}
		if !assignable(p.params[i].typ, scalar(a)) {
			return Result{}, diagnostic("type")
		}
	}
	if ctx.Err() != nil {
		return Result{}, diagnostic("cancelled")
	}
	if err = m.enter(); err != nil {
		return Result{}, err
	}
	defer m.leave()
	tx, err := store.begin(req.Identity, policy, m)
	if err != nil {
		return Result{}, err
	}
	defer tx.discard()
	v, err := evaluate(ctx, p, req.Args, tx, m)
	if err != nil {
		return Result{}, err
	}
	if err = tx.commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{Value: v, Usage: m.usage}, nil
}
