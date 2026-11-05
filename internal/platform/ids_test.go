package platform

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeRecordID(t *testing.T) {
	const id = "001AbCdEfGhIjKlIVK"
	for _, input := range []string{id[:15], id, id[:15] + "ivk", id[:15] + "iVk"} {
		got, err := NormalizeRecordID("Account", input)
		if err != nil || got != id {
			t.Errorf("%q: %q %v", input, got, err)
		}
	}
	for _, input := range []string{id[:14], id[:15] + "A", id + "A", id[:15] + "AAA", "001AbCdEfGhIjK!IVK"} {
		if _, err := NormalizeRecordID("Account", input); !errors.Is(err, errMalformedID) {
			t.Errorf("accepted %q: %v", input, err)
		}
	}
	if _, err := NormalizeRecordID("Contact", id); !errors.Is(err, errMalformedID) {
		t.Fatal(err)
	}
	generated, err := newID("Account")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := NormalizeRecordID("Account", generated[:15]); err != nil || got != generated {
		t.Fatal(got, err)
	}
}
func TestNormalizeRecordIDCaseSensitiveBase(t *testing.T) {
	const id = "001AbCdEfGhIjKlIVK"
	if got, err := NormalizeRecordID("Account", strings.ToLower(id[:15])); err != nil || got == id {
		t.Fatal(got, err)
	}
	if _, err := NormalizeRecordID("Account", strings.ToLower(id[:15])+id[15:]); !errors.Is(err, errMalformedID) {
		t.Fatal(err)
	}
}
