package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidHash = errors.New("invalid password hash")

// HashPassword returns a bcrypt hash. bcrypt only consumes 72 bytes, so
// callers validate the byte length before hashing.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	if len([]byte(password)) > 72 {
		return "", errors.New("password must be at most 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword accepts bcrypt hashes and legacy argon2id PHC hashes. Legacy
// hashes are upgraded by the caller after a successful login.
func VerifyPassword(password, encoded string) (bool, error) {
	if strings.HasPrefix(encoded, "$argon2id$") {
		return verifyArgon2id(password, encoded)
	}
	if !strings.HasPrefix(encoded, "$2") {
		return false, ErrInvalidHash
	}
	err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, ErrInvalidHash
	}
	return true, nil
}

// NeedsRehash reports whether a stored hash should be replaced after a
// successful login. Legacy argon2id hashes return true.
func NeedsRehash(encoded string) bool {
	return !strings.HasPrefix(encoded, "$2")
}

func verifyArgon2id(password, encoded string) (bool, error) {
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
	// Guard against a malicious or corrupt hash causing pathological allocation.
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
