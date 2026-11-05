package platform

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"time"
)

type JWTClaims struct{ Issuer, Subject, ID string }
type parsedJWT struct {
	signingInput string
	signature    []byte
	audience     string
	expires      int64
	issuedAt     int64
	hasIssuedAt  bool
}

var errJWT = errors.New("invalid JWT assertion")

func parseJWT(assertion string) (JWTClaims, parsedJWT, error) {
	var claims JWTClaims
	var parsed parsedJWT
	if len(assertion) > 16384 {
		return claims, parsed, errJWT
	}
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		return claims, parsed, errJWT
	}
	decoded := make([][]byte, 3)
	for i, part := range parts {
		b, e := base64.RawURLEncoding.Strict().DecodeString(part)
		if e != nil {
			return claims, parsed, errJWT
		}
		decoded[i] = b
	}
	var header struct {
		Algorithm string `json:"alg"`
	}
	if json.Unmarshal(decoded[0], &header) != nil || header.Algorithm != "RS256" {
		return claims, parsed, errJWT
	}
	var body struct {
		Issuer   string `json:"iss"`
		Subject  string `json:"sub"`
		ID       string `json:"jti"`
		Audience string `json:"aud"`
		Expires  *int64 `json:"exp"`
		IssuedAt *int64 `json:"iat"`
	}
	if json.Unmarshal(decoded[1], &body) != nil || body.Expires == nil {
		return claims, parsed, errJWT
	}
	claims = JWTClaims{Issuer: body.Issuer, Subject: body.Subject, ID: body.ID}
	parsed = parsedJWT{signingInput: parts[0] + "." + parts[1], signature: decoded[2], audience: body.Audience, expires: *body.Expires}
	if body.IssuedAt != nil {
		parsed.issuedAt = *body.IssuedAt
		parsed.hasIssuedAt = true
	}
	return claims, parsed, nil
}
func validJWTCertificate(cert *x509.Certificate, now time.Time) bool {
	if cert == nil || now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return false
	}
	key, ok := cert.PublicKey.(*rsa.PublicKey)
	return ok && key.N.BitLen() >= 2048
}
func verifyJWT(p parsedJWT, cert *x509.Certificate, audiences []string, now time.Time) error {
	if !validJWTCertificate(cert, now) {
		return errJWT
	}
	accepted := false
	for _, a := range audiences {
		if a == p.audience {
			accepted = true
			break
		}
	}
	if !accepted {
		return errJWT
	}
	if p.expires < 100000000000 {
		expiry := time.Unix(p.expires, 0)
		if !expiry.After(now) || expiry.After(now.Add(5*time.Minute)) {
			return errJWT
		}
	} else {
		if !p.hasIssuedAt {
			return errJWT
		}
		issued := time.Unix(p.issuedAt, 0)
		if issued.Before(now.Add(-5*time.Minute)) || issued.After(now.Add(5*time.Minute)) {
			return errJWT
		}
	}
	digest := sha256.Sum256([]byte(p.signingInput))
	if rsa.VerifyPKCS1v15(cert.PublicKey.(*rsa.PublicKey), crypto.SHA256, digest[:], p.signature) != nil {
		return errJWT
	}
	return nil
}
func parseJWTCertificate(value string, now time.Time) (*x509.Certificate, error) {
	block, rest := pem.Decode([]byte(value))
	if block == nil || block.Type != "CERTIFICATE" || strings.TrimSpace(string(rest)) != "" {
		return nil, errJWT
	}
	cert, e := x509.ParseCertificate(block.Bytes)
	if e != nil || !validJWTCertificate(cert, now) {
		return nil, errJWT
	}
	return cert, nil
}
