package auth

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func legacyArgon2Hash(password string) string {
	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte(password), salt, 1, 8*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, 8*1024, 1, 1,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))
}

func TestHashPasswordUsesBcrypt(t *testing.T) {
	hash, err := HashPassword("correct horse 42")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash = %q, want a bcrypt prefix", hash)
	}
	if NeedsRehash(hash) {
		t.Fatal("bcrypt hash should not need a rehash")
	}
	ok, err := VerifyPassword("correct horse 42", hash)
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v; want true, nil", ok, err)
	}
	ok, err = VerifyPassword("wrong password", hash)
	if err != nil || ok {
		t.Fatalf("wrong verify = %v, %v; want false, nil", ok, err)
	}
}

func TestVerifyPasswordAcceptsLegacyArgon2id(t *testing.T) {
	hash := legacyArgon2Hash("legacy password 7")
	if !NeedsRehash(hash) {
		t.Fatal("legacy hash should need a rehash")
	}
	ok, err := VerifyPassword("legacy password 7", hash)
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v; want true, nil", ok, err)
	}
	ok, err = VerifyPassword("wrong password", hash)
	if err != nil || ok {
		t.Fatalf("wrong verify = %v, %v; want false, nil", ok, err)
	}
}

func TestVerifyPasswordRejectsUnknownHashes(t *testing.T) {
	if _, err := VerifyPassword("password", "plain text"); err != ErrInvalidHash {
		t.Fatalf("error = %v, want ErrInvalidHash", err)
	}
	if _, err := VerifyPassword("password", "$argon2id$broken"); err != ErrInvalidHash {
		t.Fatalf("error = %v, want ErrInvalidHash", err)
	}
}

func TestHashPasswordRejectsBcryptOverflow(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", 73)); err == nil {
		t.Fatal("expected an error for a password over 72 bytes")
	}
}
