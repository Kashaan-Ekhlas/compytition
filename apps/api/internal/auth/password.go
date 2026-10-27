package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. 64 MiB / 3 passes / 2 lanes is comfortably above the
// OWASP minimum and keeps login under ~100ms on a small container.
const (
	argonMemoryKiB  = 64 * 1024
	argonIterations = 3
	argonParallel   = 2
	argonSaltLength = 16
	argonKeyLength  = 32
)

var ErrInvalidHash = errors.New("invalid password hash")

// HashPassword returns a PHC-formatted argon2id hash:
// $argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallel, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonIterations, argonParallel,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword performs a constant-time comparison of the supplied password
// against a stored PHC hash.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrInvalidHash
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, ErrInvalidHash
	}
	// Guard against a malicious/corrupt hash causing pathological allocation.
	if memory < 8*1024 || memory > 256*1024 || iterations < 1 || iterations > 10 || parallelism < 1 || parallelism > 16 {
		return false, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, ErrInvalidHash
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func passwordParamsForTest(encoded string) (m, t uint32, p uint8, saltLen int, keyLen int, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return 0, 0, 0, 0, 0, ErrInvalidHash
	}
	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[4])
	if e != nil {
		return 0, 0, 0, 0, 0, e
	}
	key, e := base64.RawStdEncoding.DecodeString(parts[5])
	if e != nil {
		return 0, 0, 0, 0, 0, e
	}
	saltLen, keyLen = len(salt), len(key)
	return
}
