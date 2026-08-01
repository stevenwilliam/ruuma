// Package security holds password hashing, token signing and the permission
// model (docs/12, A01/A02/A07).
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (docs/12, A02). 64 MB / t=3 / p=2 is the OWASP-suggested
// balance for a server that also serves traffic.
const (
	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16

	// MinPasswordLength — length beats composition rules (docs/12, A07).
	MinPasswordLength = 12
)

var (
	ErrInvalidHash    = errors.New("security: invalid password hash format")
	ErrPasswordTooShort = fmt.Errorf("security: password must be at least %d characters", MinPasswordLength)
	ErrPasswordBreached = errors.New("security: password appears in a known breach list")
	ErrPasswordTrivial  = errors.New("security: password is too predictable")
)

// HashPassword returns an argon2id PHC-format hash.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("security: salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword compares a plaintext password against a stored hash in
// constant time.
func VerifyPassword(plain, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}
	got := argon2.IDKey([]byte(plain), salt, timeCost, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// commonPasswords is a small local breach list. It is deliberately local — no
// password, hashed or otherwise, is ever sent to a third party (docs/12, A07).
var commonPasswords = map[string]bool{
	"password": true, "password123": true, "123456789012": true, "qwertyuiop12": true,
	"administrator": true, "letmein12345": true, "welcome12345": true, "iloveyou1234": true,
	"ruumaruuma12": true, "restaurant12": true, "jakarta12345": true, "indonesia123": true,
}

// ValidatePassword enforces the password policy: length first, then a breach
// check, then a triviality check. No composition theatre (docs/12, A07).
func ValidatePassword(plain string) error {
	if len([]rune(plain)) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if commonPasswords[strings.ToLower(plain)] {
		return ErrPasswordBreached
	}
	if isSingleClassRepeat(plain) {
		return ErrPasswordTrivial
	}
	return nil
}

// isSingleClassRepeat catches "aaaaaaaaaaaa" and "111111111111".
func isSingleClassRepeat(s string) bool {
	if len(s) == 0 {
		return true
	}
	first := rune(s[0])
	same := true
	for _, r := range s {
		if r != first {
			same = false
			break
		}
	}
	if same {
		return true
	}
	// all digits and strictly ascending or descending, e.g. 123456789012
	digits := true
	for _, r := range s {
		if !unicode.IsDigit(r) {
			digits = false
			break
		}
	}
	return digits && (isSequential(s, 1) || isSequential(s, -1))
}

func isSequential(s string, step int) bool {
	for i := 1; i < len(s); i++ {
		if int(s[i])-int(s[i-1]) != step {
			return false
		}
	}
	return true
}
