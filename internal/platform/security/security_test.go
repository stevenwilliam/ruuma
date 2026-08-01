package security

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// docs/12 A02: argon2id, salted, verifiable, and not reversible.
func TestArgon2idHashing(t *testing.T) {
	const password = "kelapa-gading-2026!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash %q is not argon2id", hash)
	}
	if strings.Contains(hash, password) {
		t.Fatal("the hash must not contain the password")
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil || !ok {
		t.Fatalf("VerifyPassword(correct) = %v, %v", ok, err)
	}
	ok, err = VerifyPassword("wrong password entirely", hash)
	if err != nil || ok {
		t.Fatalf("VerifyPassword(wrong) = %v, %v; want false, nil", ok, err)
	}

	// Same password, different salt, different hash.
	other, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if other == hash {
		t.Fatal("two hashes of the same password must differ (salting)")
	}

	if _, err := VerifyPassword(password, "not-a-hash"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("malformed hash: got %v, want ErrInvalidHash", err)
	}
}

// docs/12 A07: length first, breach list, no composition theatre.
func TestPasswordPolicy(t *testing.T) {
	if err := ValidatePassword("short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("short password: got %v", err)
	}
	if err := ValidatePassword("password123"); err == nil {
		t.Fatal("a breached password must be refused")
	}
	if err := ValidatePassword("aaaaaaaaaaaa"); !errors.Is(err, ErrPasswordTrivial) {
		t.Fatalf("repeated character: got %v, want ErrPasswordTrivial", err)
	}
	if err := ValidatePassword("123456789012"); err == nil {
		t.Fatal("a sequential digit run must be refused")
	}
	if err := ValidatePassword("correct horse battery staple"); err != nil {
		t.Fatalf("a long passphrase must be accepted: %v", err)
	}
}

func signer(now func() time.Time) *TokenSigner {
	return NewTokenSigner("current-signing-key-at-least-32-bytes-long", "", "ruuma", 15*time.Minute, now)
}

// docs/12 A02/A07: signature pinning, expiry, issuer, and no alg=none.
func TestTokenIssueAndParse(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	s := signer(func() time.Time { return now })
	subject := uuid.New()

	raw, jti, err := s.Issue(SubjectStaff, subject, RoleFinance)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := s.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.Subject != subject.String() || claims.Role != RoleFinance || claims.ID != jti.String() {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	// The token must not carry a store list — scope is resolved server-side on
	// every request (BR-2.7.9).
	if strings.Contains(raw, "stores") {
		t.Fatal("the access token must not carry store scope")
	}

	// Expired.
	later := signer(func() time.Time { return now.Add(20 * time.Minute) })
	if _, err := later.Parse(raw); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired token: got %v, want ErrTokenExpired", err)
	}

	// Tampered payload.
	parts := strings.Split(raw, ".")
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	if _, err := s.Parse(tampered); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("tampered token: got %v, want ErrTokenInvalid", err)
	}

	// alg=none must be refused.
	none := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "ruuma", Subject: subject.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))},
	})
	unsigned, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build alg=none token: %v", err)
	}
	if _, err := s.Parse(unsigned); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("alg=none: got %v, want ErrTokenInvalid", err)
	}

	// A different issuer is refused.
	foreign := NewTokenSigner("current-signing-key-at-least-32-bytes-long", "", "someone-else", time.Hour, func() time.Time { return now })
	other, _, _ := foreign.Issue(SubjectCustomer, subject, RoleCustomer)
	if _, err := s.Parse(other); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("foreign issuer: got %v, want ErrTokenInvalid", err)
	}
}

// docs/09 §3: a signing key can be rotated without logging everyone out.
func TestTokenKeyRotation(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	old := NewTokenSigner("old-signing-key-at-least-32-bytes-long!!", "", "ruuma", time.Hour, clock)
	raw, _, err := old.Issue(SubjectCustomer, uuid.New(), RoleCustomer)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	rotated := NewTokenSigner("new-signing-key-at-least-32-bytes-long!!",
		"old-signing-key-at-least-32-bytes-long!!", "ruuma", time.Hour, clock)
	if _, err := rotated.Parse(raw); err != nil {
		t.Fatalf("token signed with the previous key must still parse: %v", err)
	}

	// Once the previous key is removed, the old token dies.
	final := NewTokenSigner("new-signing-key-at-least-32-bytes-long!!", "", "ruuma", time.Hour, clock)
	if _, err := final.Parse(raw); err == nil {
		t.Fatal("after rotation completes, tokens on the old key must be rejected")
	}
}

// BR-2.7.12: refresh tokens are stored hashed, never in plaintext.
func TestHashToken(t *testing.T) {
	raw := "a-refresh-token-value"
	h := HashToken(raw)
	if h == raw || len(h) != 64 {
		t.Fatalf("HashToken produced %q", h)
	}
	if HashToken(raw) != h {
		t.Fatal("hashing must be deterministic for lookup")
	}
}
