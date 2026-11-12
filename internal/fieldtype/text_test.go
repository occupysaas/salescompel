package fieldtype

import (
	"strings"
	"testing"
)

func TestEmail(t *testing.T) {
	for _, ok := range []string{"a@b.co", "first.last+tag@sub.example.org"} {
		if !ValidEmail(ok) {
			t.Errorf("Email(%q) rejected", ok)
		}
	}
	long := strings.Repeat("a", 76) + "@b.co"
	for _, bad := range []string{"a@b", "a b@c.d", "a@@b.c", "a@b..c", "@b.co", "a@.co", "a@b.co.", "a\x00@b.co", long} {
		if ValidEmail(bad) {
			t.Errorf("Email(%q) accepted", bad)
		}
	}
	if len(long) != 81 {
		t.Fatalf("fixture length %d", len(long))
	}
	if !ValidEmail(strings.Repeat("a", 75) + "@b.co") {
		t.Error("80-character address rejected")
	}
}

func TestText(t *testing.T) {
	if !ValidText("héllo", 5) || ValidText("héllo!", 5) || ValidText("a\x00", 5) || ValidText(string([]byte{0xff}), 5) {
		t.Fatal("Text bounds wrong")
	}
}
