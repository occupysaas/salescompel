package platform

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
func verifyPassword(stored, password string) bool {
	if len(stored) > 1024 {
		return false
	}
	parts := strings.Split(stored, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false
	}
	values := make([]uint64, 3)
	for i, prefix := range []string{"m=", "t=", "p="} {
		if !strings.HasPrefix(params[i], prefix) {
			return false
		}
		v, e := strconv.ParseUint(strings.TrimPrefix(params[i], prefix), 10, 32)
		if e != nil {
			return false
		}
		values[i] = v
	}
	m, iterations, parallelism := values[0], values[1], values[2]
	if parallelism < 1 || parallelism > 16 || iterations < 1 || iterations > 10 || m < 8*parallelism || m > 262144 {
		return false
	}
	salt, e := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if e != nil || len(salt) != 16 {
		return false
	}
	key, e := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if e != nil || len(key) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(m), uint8(parallelism), 32)
	return subtle.ConstantTimeCompare(actual, key) == 1
}

var dummyPasswordHash = func() string {
	secret, e := newSecret()
	if e != nil {
		panic("initialize password verifier")
	}
	hash, e := hashPassword(secret)
	if e != nil {
		panic("initialize password verifier")
	}
	return hash
}()

func verifyDummyPassword(password string) { verifyPassword(dummyPasswordHash, password) }
func newSecret() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func orgID(tenant string) string {
	digest := sha256.Sum256([]byte(tenant))
	base := "00D" + hex.EncodeToString(digest[:6])
	return base + idChecksum(base)
}
func accessTokenValue(tenant string) (string, error) {
	s, e := newSecret()
	if e != nil {
		return "", e
	}
	return orgID(tenant)[:15] + "!" + s, nil
}
func verifyPKCE(challenge, verifier string) bool {
	if len(verifier) < 43 || len(verifier) > 256 {
		return false
	}
	for _, c := range verifier {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._~-", c)) {
			return false
		}
	}
	digest := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(digest[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(challenge)) == 1
}
