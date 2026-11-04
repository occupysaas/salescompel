package apex

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type scalarSpec struct {
	Type    string  `json:"type"`
	Integer *int32  `json:"integer,omitempty"`
	Boolean *bool   `json:"boolean,omitempty"`
	String  *string `json:"string,omitempty"`
}

func (s scalarSpec) value(t *testing.T) Value {
	t.Helper()
	switch s.Type {
	case "integer":
		if s.Integer != nil && s.Boolean == nil && s.String == nil {
			return Integer(*s.Integer)
		}
	case "boolean":
		if s.Boolean != nil && s.Integer == nil && s.String == nil {
			return Boolean(*s.Boolean)
		}
	case "string":
		if s.String != nil && s.Integer == nil && s.Boolean == nil {
			return String(*s.String)
		}
	case "null":
		if s.String == nil && s.Integer == nil && s.Boolean == nil {
			return Null()
		}
	}
	t.Fatalf("invalid typed scalar: %+v", s)
	return Null()
}

type scenario struct {
	ID               string       `json:"id"`
	SourceFile       string       `json:"sourceFile"`
	Entry            string       `json:"entry"`
	Args             []scalarSpec `json:"args"`
	Identity         Identity     `json:"identity"`
	Policy           Policy       `json:"policy"`
	Accounts         []Account    `json:"accounts"`
	Limits           Limits       `json:"limits"`
	ExpectedCode     string       `json:"expectedCode"`
	ExpectedReturn   *scalarSpec  `json:"expectedReturn"`
	ExpectedAccounts []Account    `json:"expectedAccounts"`
	Basis            string       `json:"basis"`
	SourceRefs       []string     `json:"sourceRefs"`
}

func TestScenarios(t *testing.T) {
	f, e := os.Open("testdata/scenarios.json")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var cases []scenario
	if e = dec.Decode(&cases); e != nil {
		t.Fatal(e)
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		t.Fatal("trailing JSON")
	}
	if len(cases) == 0 {
		t.Fatal("empty scenarios")
	}
	refs := map[string]bool{"classes": true, "methods": true, "primitives": true, "lists": true, "operators": true, "loops": true, "basics": true, "query": true, "binds": true, "dml": true}
	ids := map[string]bool{}
	hashes := map[string][32]byte{}
	for _, tc := range cases {
		if tc.ID == "" || ids[tc.ID] {
			t.Fatal("duplicate/empty scenario")
		}
		ids[tc.ID] = true
		if tc.SourceFile != filepath.Base(tc.SourceFile) || filepath.Ext(tc.SourceFile) != ".cls" {
			t.Fatal("unsafe source path")
		}
		if tc.Basis == "" || len(tc.SourceRefs) == 0 {
			t.Fatal("missing provenance")
		}
		for _, ref := range tc.SourceRefs {
			if !refs[ref] {
				t.Fatalf("unknown source reference %s", ref)
			}
		}
		t.Run(tc.ID, func(t *testing.T) {
			for repeat := 0; repeat < 2; repeat++ {
				src := fixture(t, tc.SourceFile)
				hash := sha256.Sum256([]byte(src))
				if prior, ok := hashes[tc.SourceFile]; ok && prior != hash {
					t.Fatal("source rewritten across scenarios")
				}
				hashes[tc.SourceFile] = hash
				args := make([]Value, len(tc.Args))
				for i, a := range tc.Args {
					args[i] = a.value(t)
				}
				s := makeStore(t, tc.Accounts)
				var events []string
				r, e := run(context.Background(), Request{Source: src, Entry: tc.Entry, Args: args, Identity: tc.Identity, Limits: tc.Limits}, s, tc.Policy, func(s string) { events = append(events, s) })
				wantCode(t, e, tc.ExpectedCode)
				if tc.ExpectedCode == "" {
					if tc.ExpectedReturn == nil || r.Value != tc.ExpectedReturn.value(t) {
						t.Fatalf("unexpected return %+v", r)
					}
					commits := 0
					for _, event := range events {
						if event == "commit" {
							commits++
						}
					}
					if commits != 1 {
						t.Fatalf("commits=%d events=%v", commits, events)
					}
				} else {
					if tc.ExpectedReturn != nil || r != (Result{}) || contains(events, "commit") {
						t.Fatal("error leaked result/commit", events)
					}
					if tc.ExpectedCode == "syntax" || tc.ExpectedCode == "unsupported" {
						if len(events) != 0 {
							t.Fatal("parser touched host", events)
						}
					}
				}
				if tc.ID == "late-batch-denial" || tc.ID == "staged-runtime" || tc.ID == "staged-budget" {
					if !contains(events, "staged") || !contains(events, "discard") {
						t.Fatal("not a staged rollback", events)
					}
				}
				sameStore(t, s, tc.ExpectedAccounts)
				t.Logf("repeat=%d code=%q return=%+v events=%v", repeat, tc.ExpectedCode, r.Value, events)
			}
		})
	}
}
func TestParallelInvocations(t *testing.T) {
	src := fixture(t, "rename.cls")
	beforeHash := sha256.Sum256([]byte(src))
	for i := 0; i < 12; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			tenant := fmt.Sprintf("tenant-%d", i)
			name := fmt.Sprintf("name-%d", i)
			identity := Identity{tenant, fmt.Sprintf("principal-%d", i)}
			input := []Account{{tenant, "same-id", "g", "before"}, {"other", "same-id", "g", "other"}}
			s := makeStore(t, input)
			args := []Value{String("g"), String(name)}
			saved := append([]Value{}, args...)
			req := Request{Source: src, Entry: "Rename.run", Args: args, Identity: identity, Limits: DefaultLimits()}
			req.Limits.Steps = 200 + i
			p := Policy{Identity: identity, QueryAllowed: true, NameWriteAllowed: true}
			r, e := Run(context.Background(), req, s, p)
			wantCode(t, e, "")
			if r.Value != Integer(0) || r.Usage.Rows != 1 || r.Usage.Writes != 1 {
				t.Fatal(r)
			}
			want := []Account{{"other", "same-id", "g", "other"}, {tenant, "same-id", "g", name}}
			sameStore(t, s, want)
			if !reflect.DeepEqual(args, saved) || input[0].Name != "before" || sha256.Sum256([]byte(src)) != beforeHash {
				t.Fatal("mutated inputs")
			}
		})
	}
}
