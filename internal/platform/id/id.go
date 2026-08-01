// Package id generates identifiers.
//
//   - Primary keys are UUIDv7 — time-ordered for index locality, not sequential
//     in a way that leaks volume (BR-1.2.1).
//   - Human-facing order codes are Crockford base32 over CSPRNG bytes: unique,
//     non-guessable, carrying no identity and no sequence (BR-1.2.2).
package id

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

// crockford is Crockford base32: no I, L, O or U, so a human reading a printed
// ticket cannot confuse a code.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// OrderCodeLength is the length of a human-facing order code (BR-1.2.2).
const OrderCodeLength = 8

// New returns a UUIDv7 (BR-1.2.1).
func New() uuid.UUID {
	v, err := uuid.NewV7()
	if err != nil {
		// NewV7 only fails if the system CSPRNG fails, in which case nothing
		// this service does is safe to continue.
		panic(fmt.Sprintf("uuidv7: %v", err))
	}
	return v
}

// NewString returns a UUIDv7 as a string.
func NewString() string { return New().String() }

// OrderCode returns an 8-character Crockford base32 code from the system
// CSPRNG (BR-1.2.2). Uniqueness is additionally guaranteed by the unique index
// on orders.order_code; callers retry on collision.
func OrderCode() (string, error) {
	return randomCrockford(OrderCodeLength)
}

// UniqueCode returns the Indonesian "kode unik" — an integer in 1..999 added to
// an order total so finance can match one incoming transfer to one order
// (BR-2.6.2). Uniqueness among open orders is enforced by the caller inside the
// order transaction.
func UniqueCode() (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(999))
	if err != nil {
		return 0, fmt.Errorf("unique code: %w", err)
	}
	return int(n.Int64()) + 1, nil
}

// Token returns a URL-safe random token of n Crockford characters, used for
// email verification and password reset (stored hashed).
func Token(n int) (string, error) { return randomCrockford(n) }

func randomCrockford(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("csprng: %w", err)
	}
	var sb strings.Builder
	sb.Grow(n)
	for _, v := range b {
		sb.WriteByte(crockford[int(v)%len(crockford)])
	}
	return sb.String(), nil
}

// NormalizeOrderCode uppercases a code and maps the Crockford aliases a human
// might type (I/L → 1, O → 0) so a mistyped ticket still resolves.
func NormalizeOrderCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	r := strings.NewReplacer("I", "1", "L", "1", "O", "0", "-", "", " ", "")
	return r.Replace(s)
}
