package apex

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func request(src, entry string, args ...Value) Request {
	return Request{Source: src, Entry: entry, Args: args, Identity: hostPolicy().Identity, Limits: DefaultLimits()}
}
func runScalar(t *testing.T, body string) (Result, error) {
	t.Helper()
	return Run(context.Background(), request(source(body), "Probe.run"), makeStore(t, nil), hostPolicy())
}
func TestRunCompute(t *testing.T) {
	s := fixture(t, "compute.cls")
	for _, tc := range []struct{ n, w int32 }{{0, 0}, {1, 0}, {4, 6}} {
		r, e := Run(context.Background(), request(s, "Compute.run", Integer(tc.n)), makeStore(t, nil), hostPolicy())
		wantCode(t, e, "")
		if r.Value != Integer(tc.w) {
			t.Fatalf("n=%d: %+v", tc.n, r)
		}
	}
}
func TestRunTypes(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		want             int32
	}{
		{"short-and", "if (false && (1 / 0 == 0)) {return 2;} return 1;", "", 1},
		{"short-or", "if (true || (1 / 0 == 0)) {return 3;} return 0;", "", 3},
		{"scope", "Integer x = 1; {Integer x = 7; x = 8;} return x;", "", 1},
		{"outer-assignment", "Integer x = 1; {x = 8;} return x;", "", 8},
		{"null-equality", "String s = null; if (s == null) {return 4;} return 0;", "", 4},
		{"null-deref", "Account a = null; String s = a.Name; return 0;", "runtime", 0},
		{"null-arithmetic", "String s = null; String x = s + 'a'; return 0;", "runtime", 0},
		{"null-integer", "Integer n = null; return 0;", "type", 0},
		{"null-boolean", "Boolean n = null; return 0;", "type", 0},
		{"overflow", "return 2147483647 + 1;", "runtime", 0},
		{"min-negate", "Integer n = -2147483648; return -n;", "runtime", 0},
		{"min-divide", "return -2147483648 / -1;", "runtime", 0},
		{"multiply-overflow", "return 50000 * 50000;", "runtime", 0},
		{"subtract-overflow", "return -2147483648 - 1;", "runtime", 0},
		{"division-zero", "return 4 / 0;", "runtime", 0},
		{"division-truncation", "return -7 / 2;", "", -3},
		{"precedence", "return 2 + 3 * 4;", "", 14},
		{"boolean", "if (!(3 <= 2) && 4 >= 4 && 2 != 3) {return 9;} return 0;", "", 9},
		{"exact-string", "if ('a' == 'A') {return 1;} return 0;", "", 0},
		{"missing-return", "Integer x = 1;", "runtime", 0},
		{"wrong-return", "return true;", "type", 0},
		{"unknown-variable", "return missing;", "type", 0},
		{"escaped-variable", "{Integer x = 1;} return x;", "type", 0},
		{"no-conversion", "String s = 'x' + 1; return 0;", "type", 0},
		{"null-condition", "if (null) {return 1;} return 0;", "runtime", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, e := runScalar(t, tc.body)
			wantCode(t, e, tc.code)
			if tc.code == "" && r.Value != Integer(tc.want) {
				t.Fatalf("got %+v", r)
			}
			if tc.code != "" && r != (Result{}) {
				t.Fatal("error leaked result")
			}
		})
	}
	for _, tc := range []struct {
		args  []Value
		entry string
		code  string
	}{{nil, "Compute.run", "type"}, {[]Value{String("x")}, "Compute.run", "type"}, {[]Value{Integer(1), Integer(2)}, "Compute.run", "type"}, {[]Value{Integer(1)}, "Other.run", "invalid-input"}} {
		var events []string
		_, e := run(context.Background(), request(fixture(t, "compute.cls"), tc.entry, tc.args...), makeStore(t, nil), hostPolicy(), func(s string) { events = append(events, s) })
		wantCode(t, e, tc.code)
		if len(events) != 0 {
			t.Fatal("host began before input validation", events)
		}
	}
}
func TestRunStringBounds(t *testing.T) {
	for _, n := range []int{4, 3} {
		req := request(source("String s = 'ab' + 'cd'; if (s == 'abcd') { return 1; } return 0;"), "Probe.run")
		req.Limits.StringBytes = n
		r, e := Run(context.Background(), req, makeStore(t, nil), hostPolicy())
		if n == 4 {
			wantCode(t, e, "")
			if r.Value != Integer(1) {
				t.Fatal(r)
			}
		} else {
			wantCode(t, e, "budget")
		}
	}
	// No oversized literal masks the concatenation check.
	req := request(source("String s = 'ab' + 'cd'; return 0;"), "Probe.run")
	req.Limits.StringBytes = 3
	_, e := Run(context.Background(), req, makeStore(t, nil), hostPolicy())
	wantCode(t, e, "budget")
	req = request(fixture(t, "rename.cls"), "Rename.run", String("g"), String(strings.Repeat("x", 4097)))
	var events []string
	_, e = run(context.Background(), req, makeStore(t, nil), hostPolicy(), func(s string) { events = append(events, s) })
	wantCode(t, e, "budget")
	if len(events) != 0 {
		t.Fatal(events)
	}
}
func TestRunRecords(t *testing.T) {
	for _, name := range []string{"rename.cls", "fail-after-write.cls"} {
		t.Run(name, func(t *testing.T) {
			s := makeStore(t, hostRows())
			before := s.Snapshot()
			entry := "Rename.run"
			code := ""
			if name != "rename.cls" {
				entry = "FailAfterWrite.run"
				code = "runtime"
			}
			var events []string
			r, e := run(context.Background(), request(fixture(t, name), entry, String("g"), String("new")), s, hostPolicy(), func(s string) { events = append(events, s) })
			wantCode(t, e, code)
			if code == "" {
				before[0].Name = "new"
				before[1].Name = "new"
				if r.Usage.Rows != 2 || r.Usage.Writes != 2 {
					t.Fatal(r)
				}
			}
			sameStore(t, s, before)
			if !contains(events, "staged") {
				t.Fatal("never staged", events)
			}
			if code != "" && contains(events, "commit") {
				t.Fatal("committed on error", events)
			}
		})
	}
}
func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
func TestRunUnsupportedBeforeHost(t *testing.T) {
	src := strings.Replace(fixture(t, "rename.cls"), "return 0;", "return 0; if (false) { System.debug('no'); }", 1)
	s := makeStore(t, hostRows())
	before := s.Snapshot()
	var events []string
	_, e := run(context.Background(), request(src, "Rename.run", String("g"), String("new")), s, hostPolicy(), func(s string) { events = append(events, s) })
	wantCode(t, e, "unsupported")
	if len(events) != 0 {
		t.Fatal("host activity", events)
	}
	sameStore(t, s, before)
}
func TestRunBudgets(t *testing.T) {
	for _, kind := range []string{"steps", "depth", "rows", "writes", "collection", "after-staging"} {
		t.Run(kind, func(t *testing.T) {
			s := makeStore(t, hostRows())
			before := s.Snapshot()
			req := request(fixture(t, "rename.cls"), "Rename.run", String("g"), String("new"))
			var events []string
			switch kind {
			case "steps":
				req = request(fixture(t, "infinite.cls"), "Infinite.run")
				req.Limits.Steps = 20
			case "depth":
				req.Limits.CallDepth = 1
			case "rows":
				req.Limits.Rows = 1
			case "writes":
				req.Limits.Writes = 1
			case "collection":
				req.Limits.CollectionItems = 1
			case "after-staging":
				req.Source = strings.Replace(req.Source, "return 0;", "while (true) {} return 0;", 1)
				req.Limits.Steps = 100
			}
			_, e := run(context.Background(), req, s, hostPolicy(), func(s string) { events = append(events, s) })
			wantCode(t, e, "budget")
			sameStore(t, s, before)
			if kind == "after-staging" && !contains(events, "staged") {
				t.Fatal("budget before stage", events)
			}
			if contains(events, "commit") {
				t.Fatal(events)
			}
		})
	}
}
func TestRunCancellation(t *testing.T) {
	for _, label := range []string{"before", "staged", "before-commit"} {
		t.Run(label, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := makeStore(t, hostRows())
			before := s.Snapshot()
			var events []string
			if label == "before" {
				cancel()
			}
			_, e := run(ctx, request(fixture(t, "rename.cls"), "Rename.run", String("g"), String("new")), s, hostPolicy(), func(s string) {
				events = append(events, s)
				if s == label {
					cancel()
				}
			})
			wantCode(t, e, "cancelled")
			sameStore(t, s, before)
			if label != "before" && !contains(events, label) {
				t.Fatal("missing synchronization", events)
			}
			if contains(events, "commit") {
				t.Fatal(events)
			}
		})
	}
}
func TestRunInputs(t *testing.T) {
	req := request(source("return 0;"), "Probe.run")
	s := makeStore(t, nil)
	for _, tc := range []struct {
		ctx    context.Context
		store  *Store
		id     Identity
		policy Policy
		code   string
	}{{nil, s, req.Identity, hostPolicy(), "invalid-input"}, {context.Background(), nil, req.Identity, hostPolicy(), "invalid-input"}, {context.Background(), s, Identity{}, hostPolicy(), "invalid-input"}, {context.Background(), s, req.Identity, Policy{}, "denied"}} {
		r := req
		r.Identity = tc.id
		_, e := Run(tc.ctx, r, tc.store, tc.policy)
		wantCode(t, e, tc.code)
	}
	if !reflect.DeepEqual(req.Args, []Value(nil)) {
		t.Fatal("inputs changed")
	}
}
