package platform

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	a, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	b, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "$argon2id$v=19$m=19456,t=2,p=1$") || a == b || !verifyPassword(a, "correct horse battery") || verifyPassword(a, "wrong") {
		t.Fatal("hash format, salt or verification failed")
	}
	for _, bad := range []string{"malformed", strings.Replace(a, "m=19456", "m=262145", 1), strings.Replace(a, "t=2", "t=999999", 1), strings.Replace(a, "p=1", "p=0", 1)} {
		if verifyPassword(bad, "correct horse battery") {
			t.Fatal("accepted hostile hash")
		}
	}
	verifyDummyPassword("unknown")
}
func TestNewSecret(t *testing.T) {
	a, e := newSecret()
	if e != nil {
		t.Fatal(e)
	}
	b, e := newSecret()
	if e != nil {
		t.Fatal(e)
	}
	if a == b || !regexp.MustCompile("^[0-9a-f]{64}$").MatchString(a) {
		t.Fatal("invalid secret")
	}
}
func TestOrgID(t *testing.T) {
	a := orgID("tenant")
	if len(a) != 18 || !strings.HasPrefix(a, "00D") || a != orgID("tenant") || a == orgID("other") || a[15:] != idChecksum(a[:15]) {
		t.Fatal("invalid org ID")
	}
}
func TestAccessTokenValue(t *testing.T) {
	a, e := accessTokenValue("tenant")
	if e != nil {
		t.Fatal(e)
	}
	if !regexp.MustCompile("^00D[A-Za-z0-9]{12}![0-9a-f]{64}$").MatchString(a) {
		t.Fatal("invalid token")
	}
}
func TestVerifyPKCE(t *testing.T) {
	if !verifyPKCE("E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk") {
		t.Fatal("RFC pair rejected")
	}
	for _, n := range []int{42, 43, 171, 256, 257} {
		v := strings.Repeat("a", n)
		h := sha256.Sum256([]byte(v))
		if verifyPKCE(base64.RawURLEncoding.EncodeToString(h[:]), v) != (n >= 43 && n <= 256) {
			t.Fatalf("length %d", n)
		}
	}
	v := strings.Repeat("a", 42) + "!"
	h := sha256.Sum256([]byte(v))
	if verifyPKCE(base64.RawURLEncoding.EncodeToString(h[:]), v) {
		t.Fatal("invalid alphabet accepted")
	}
}
