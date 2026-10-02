package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Passwords hashes passwords with argon2id in the PHC string format.
type Passwords struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
}

// DefaultPasswords are OWASP's argon2id parameters.
func DefaultPasswords() Passwords {
	return Passwords{MemoryKiB: 19 * 1024, Time: 2, Threads: 1}
}

// Hash hashes a password with a fresh salt.
func (p Passwords) Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, 32)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", p.MemoryKiB, p.Time, p.Threads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify reports whether a password matches a hash made by Hash, with the parameters stored in it.
func (Passwords) Verify(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &threads); err != nil {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, threads, uint32(len(want))) //nolint:gosec // a 32-byte key
	return subtle.ConstantTimeCompare(got, want) == 1
}

// newToken makes an opaque token for a link or a cookie, and the hash it is stored as.
func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// HashToken is how a token is stored and looked up.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
