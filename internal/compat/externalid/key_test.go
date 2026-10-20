package externalid

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

var referenceVectors = []struct {
	name, a, b                     string
	observedEqual, hypothesisEqual bool
}{
	{"exact", "p-key", "p-key", true, true},
	{"ASCII", "p-AbC", "p-abc", true, true},
	{"accent", "p-é", "p-e\u0301", true, true},
	{"ASCII I", "p-I", "p-i", true, true},
	{"dotless I", "p-I", "p-ı", false, false},
	{"sharp s", "p-ß", "p-ss", true, true},
	{"decoded literal", "p-/percent%é", "p-/percent%é", true, true},
	{"leading space", "p-key", " p-key", true, false},
	{"trailing space", "p-key", "p-key ", true, false},
}

func TestCanonicalKeyReferenceVectors(t *testing.T) {
	if HypothesisVersion != "nfc-full-fold-v1" {
		t.Fatalf("unexpected hypothesis %q", HypothesisVersion)
	}
	var mismatches []string
	for _, v := range referenceVectors {
		t.Run(v.name, func(t *testing.T) {
			a, err := CanonicalKey(v.a)
			if err != nil {
				t.Fatal(err)
			}
			b, err := CanonicalKey(v.b)
			if err != nil {
				t.Fatal(err)
			}
			if (a == b) != v.hypothesisEqual {
				t.Errorf("keys %q / %q: equal=%v, want %v", a, b, a == b, v.hypothesisEqual)
			}
			if (a == b) != v.observedEqual {
				mismatches = append(mismatches, v.name)
			}
		})
	}
	if !reflect.DeepEqual(mismatches, []string{"leading space", "trailing space"}) {
		t.Fatalf("reference disagreements: %v", mismatches)
	}
}

func TestCanonicalKeyRobustness(t *testing.T) {
	// These candidate properties catch trimming, accent stripping, decoding,
	// invalid-byte replacement and unstable normalization; they are unobserved.
	vectors := []struct{ input, want string }{
		{"", ""}, {"\tP-KEY\t", "\tp-key\t"}, {"\u00a0P-KEY\u00a0", "\u00a0p-key\u00a0"},
		{"e", "e"}, {"é", "é"}, {"p-/percent%é", "p-/percent%é"},
		{"p-%2F", "p-%2f"}, {"p-/", "p-/"},
	}
	for _, v := range vectors {
		t.Run(fmt.Sprintf("%q", v.input), func(t *testing.T) {
			got, err := CanonicalKey(v.input)
			if err != nil || got != v.want {
				t.Fatalf("got %q, %v; want %q", got, err, v.want)
			}
			again, err := CanonicalKey(got)
			if err != nil || again != got {
				t.Fatalf("not idempotent: %q, %v", again, err)
			}
		})
	}
	for _, v := range referenceVectors {
		for _, input := range []string{v.a, v.b} {
			got, err := CanonicalKey(input)
			if err != nil {
				t.Fatal(err)
			}
			again, err := CanonicalKey(got)
			if err != nil || again != got {
				t.Fatalf("not idempotent for %q", input)
			}
		}
	}
	got, err := CanonicalKey(string([]byte{0xff}))
	if err == nil || got != "" {
		t.Fatalf("invalid UTF-8: got %q, %v", got, err)
	}
	// Null has no representation in CanonicalKey's string input; Record.Key
	// carries null separately and is covered by collision tests.
}

func TestCanonicalKeyConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	errors := make(chan error, 32)
	for worker := 0; worker < 32; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repeat := 0; repeat < 100; repeat++ {
				for _, v := range referenceVectors {
					a, ea := CanonicalKey(v.a)
					b, eb := CanonicalKey(v.b)
					if ea != nil || eb != nil || (a == b) != v.hypothesisEqual {
						errors <- fmt.Errorf("%s: %q / %q, errors %v / %v", v.name, a, b, ea, eb)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
