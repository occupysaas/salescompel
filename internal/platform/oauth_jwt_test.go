package platform

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func jwtTestCert(t *testing.T, key crypto.Signer, now time.Time) *x509.Certificate {
	t.Helper()
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, tpl, tpl, key.Public(), key)
	if e != nil {
		t.Fatal(e)
	}
	cert, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	return cert
}
func jwtTestSign(t *testing.T, key *rsa.PrivateKey, alg string, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]any{"alg": alg, "jku": "https://untrusted.invalid/key"})
	p, _ := json.Marshal(claims)
	s := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(p)
	d := sha256.Sum256([]byte(s))
	sig, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, d[:])
	if e != nil {
		t.Fatal(e)
	}
	return s + "." + base64.RawURLEncoding.EncodeToString(sig)
}
func TestJWTAccepts(t *testing.T) {
	now := time.Unix(1793845797, 0)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert := jwtTestCert(t, key, now)
	for _, tc := range []struct {
		name     string
		exp, iat int64
		aud      string
	}{{"seconds", now.Unix() + 60, 0, "https://h"}, {"five minutes", now.Unix() + 300, 0, "https://h"}, {"milliseconds", now.UnixMilli(), now.Unix(), "https://h"}, {"old iat", now.UnixMilli(), now.Unix() - 299, "https://h"}, {"slash", now.Unix() + 60, 0, "https://h/"}} {
		t.Run(tc.name, func(t *testing.T) {
			s := jwtTestSign(t, key, "RS256", map[string]any{"iss": "app", "sub": "user", "jti": "id", "aud": tc.aud, "exp": tc.exp, "iat": tc.iat})
			c, p, e := parseJWT(s)
			if e != nil {
				t.Fatal(e)
			}
			if c.Issuer != "app" || c.Subject != "user" || c.ID != "id" {
				t.Fatal(c)
			}
			if e = verifyJWT(p, cert, []string{"https://h", "https://h/"}, now); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestJWTRejects(t *testing.T) {
	now := time.Unix(1793845797, 0)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert := jwtTestCert(t, key, now)
	cases := map[string]string{}
	for _, tc := range []struct {
		name, alg, aud string
		exp, iat       int64
	}{{"none", "none", "https://h", now.Unix() + 60, 0}, {"RS384", "RS384", "https://h", now.Unix() + 60, 0}, {"expired", "RS256", "https://h", now.Unix(), 0}, {"301 seconds", "RS256", "https://h", now.Unix() + 301, 0}, {"far future", "RS256", "https://h", now.Unix() + 31536000, 0}, {"zero iat", "RS256", "https://h", now.UnixMilli(), 0}, {"old iat", "RS256", "https://h", now.UnixMilli(), now.Unix() - 301}, {"future iat", "RS256", "https://h", now.UnixMilli(), now.Unix() + 301}, {"audience", "RS256", "https://wrong", now.Unix() + 60, 0}} {
		cases[tc.name] = jwtTestSign(t, key, tc.alg, map[string]any{"exp": tc.exp, "iat": tc.iat, "aud": tc.aud})
	}
	cases["missing iat"] = jwtTestSign(t, key, "RS256", map[string]any{"exp": now.UnixMilli(), "aud": "https://h"})
	claims := map[string]any{"exp": now.Unix() + 60, "aud": "https://h"}
	valid := jwtTestSign(t, key, "RS256", claims)
	parts := strings.Split(valid, ".")
	payload, _ := json.Marshal(map[string]any{"exp": now.Unix() + 61, "aud": "https://h"})
	cases["tampered"] = parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + parts[2]
	cases["different key"] = jwtTestSign(t, other, "RS256", claims)
	h := base64.RawURLEncoding.EncodeToString([]byte("{\"alg\":\"HS256\"}"))
	body := h + "." + parts[1]
	mac := hmac.New(sha256.New, cert.RawSubjectPublicKeyInfo)
	mac.Write([]byte(body))
	cases["HMAC"] = body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	cases["none"] = base64.RawURLEncoding.EncodeToString([]byte("{\"alg\":\"none\"}")) + "." + parts[1] + "."
	cases["bad encoding"] = "!.!.!"
	cases["two segments"] = "a.b"
	cases["oversized"] = strings.Repeat("a", 16385)
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			_, p, e := parseJWT(s)
			if e == nil {
				e = verifyJWT(p, cert, []string{"https://h", "https://h/"}, now)
			}
			if e == nil {
				t.Fatal("accepted invalid assertion")
			}
		})
	}
}
func TestJWTCertificate(t *testing.T) {
	now := time.Now()
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	small, _ := rsa.GenerateKey(rand.Reader, 1024)
	ec, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	for _, tc := range []struct {
		name string
		key  crypto.Signer
		at   time.Time
		ok   bool
	}{{"valid", rsaKey, now, true}, {"expired", rsaKey, now.Add(-2 * time.Hour), false}, {"small", small, now, false}, {"ECDSA", ec, now, false}} {
		t.Run(tc.name, func(t *testing.T) {
			cert := jwtTestCert(t, tc.key, tc.at)
			s := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
			_, e := parseJWTCertificate(s, now)
			if (e == nil) != tc.ok {
				t.Fatalf("error %v", e)
			}
		})
	}
	if _, e := parseJWTCertificate("bad PEM", now); e == nil {
		t.Fatal("accepted malformed PEM")
	}
}
