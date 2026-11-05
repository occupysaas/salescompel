package platform

import "testing"

func TestCanonicalExternalIDVectors(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"key", "key"}, {"AbC", "abc"}, {"é", "é"}, {"e\u0301", "é"}, {"I", "i"}, {"i", "i"}, {"ı", "ı"}, {"ß", "ss"}, {"ss", "ss"}, {"/percent%é", "/percent%é"}, {" key", "key"}, {"key ", "key"}, {"key\t", "key\t"}, {"key\u00a0", "key\u00a0"}, {"a b", "a b"}, {"ab", "ab"}, {"%2F", "%2f"}, {"/", "/"}, {"   ", ""}} {
		got, e := canonicalExternalID(tc.input)
		if e != nil || got != tc.want {
			t.Errorf("%q: %q %v", tc.input, got, e)
		}
		twice, e := canonicalExternalID(got)
		if e != nil || twice != got {
			t.Fatal("not idempotent", tc.input)
		}
	}
	if _, e := canonicalExternalID("\xff"); e == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}
