package identity

import (
	"errors"
	"testing"
	"time"
)

var now = time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

func ptr(t time.Time) *time.Time { return &t }

// BR-2.7.4 / D12: no guest checkout, and no ordering without a verified phone —
// for every one of the four sign-in methods.
func TestVerifiedPhoneGatesOrdering_BR_2_7_4(t *testing.T) {
	if err := CanPlaceOrder(nil); !errors.Is(err, ErrPhoneVerificationRequired) {
		t.Fatalf("unverified: got %v, want ErrPhoneVerificationRequired", err)
	}
	if err := CanPlaceOrder(ptr(time.Time{})); !errors.Is(err, ErrPhoneVerificationRequired) {
		t.Fatalf("zero time: got %v, want ErrPhoneVerificationRequired", err)
	}
	if err := CanPlaceOrder(ptr(now)); err != nil {
		t.Fatalf("verified phone should be allowed to order: %v", err)
	}
	if len(AllProviders()) != 4 {
		t.Fatalf("expected four sign-in methods (D24), got %d", len(AllProviders()))
	}
}

// BR-2.7.3: linking requires a verified match on both sides. This is the
// account-takeover rule.
func TestIdentityLinking_BR_2_7_3(t *testing.T) {
	existing := &ExistingAccount{
		CustomerID:      "cust-1",
		Email:           "budi@example.com",
		EmailVerifiedAt: ptr(now),
		Phone:           "+628123456789",
		PhoneVerifiedAt: ptr(now),
	}

	cases := []struct {
		name     string
		in       IncomingIdentity
		existing *ExistingAccount
		want     LinkDecision
	}{
		{
			"google with verified matching email links",
			IncomingIdentity{Provider: Google, Email: "budi@example.com", EmailVerified: true},
			existing, LinkToExisting,
		},
		{
			"case-insensitive email still links",
			IncomingIdentity{Provider: Google, Email: "Budi@Example.com", EmailVerified: true},
			existing, LinkToExisting,
		},
		{
			"unverified matching email is refused",
			IncomingIdentity{Provider: Instagram, Email: "budi@example.com", EmailVerified: false},
			existing, RefuseLink,
		},
		{
			"verified provider email but unverified local account is refused",
			IncomingIdentity{Provider: Google, Email: "budi@example.com", EmailVerified: true},
			&ExistingAccount{CustomerID: "cust-2", Email: "budi@example.com"},
			RefuseLink,
		},
		{
			"verified matching phone links",
			IncomingIdentity{Provider: Phone, Phone: "+628123456789", PhoneVerified: true},
			existing, LinkToExisting,
		},
		{
			"unverified matching phone is refused",
			IncomingIdentity{Provider: Phone, Phone: "+628123456789"},
			existing, RefuseLink,
		},
		{
			"no existing account creates a new one",
			IncomingIdentity{Provider: Google, Email: "new@example.com", EmailVerified: true},
			nil, CreateNew,
		},
		{
			"different email creates a new account",
			IncomingIdentity{Provider: Google, Email: "other@example.com", EmailVerified: true},
			existing, CreateNew,
		},
		{
			"instagram with no email or phone creates a new account",
			IncomingIdentity{Provider: Instagram, ProviderUserID: "ig-123"},
			existing, CreateNew,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DecideLink(tc.in, tc.existing); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// BR-2.7.5: single-use, expiring, attempt-capped codes.
func TestOTPLifecycle_BR_2_7_5(t *testing.T) {
	base := OTP{Purpose: PurposeVerifyPhone, MaxAttempts: 5, ExpiresAt: now.Add(5 * time.Minute)}

	if err := CheckOTP(base, "hash", "hash", now); err != nil {
		t.Fatalf("valid code: %v", err)
	}

	cases := []struct {
		name string
		otp  OTP
		hash string
		at   time.Time
		want error
	}{
		{"wrong code", base, "other", now, ErrOTPMismatch},
		{"expired", OTP{MaxAttempts: 5, ExpiresAt: now.Add(-time.Second)}, "hash", now, ErrOTPExpired},
		{"already used", OTP{MaxAttempts: 5, ExpiresAt: now.Add(time.Minute), ConsumedAt: ptr(now)}, "hash", now, ErrOTPConsumed},
		{"attempts exhausted", OTP{MaxAttempts: 5, Attempts: 5, ExpiresAt: now.Add(time.Minute)}, "hash", now, ErrOTPAttemptsExceeded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckOTP(tc.otp, "hash", tc.hash, tc.at); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}

	// Expiry is exact: the code dies at ExpiresAt, not after it.
	exact := OTP{MaxAttempts: 5, ExpiresAt: now}
	if err := CheckOTP(exact, "hash", "hash", now); !errors.Is(err, ErrOTPExpired) {
		t.Fatalf("at exactly ExpiresAt: got %v, want ErrOTPExpired", err)
	}
}

// The phone is the key for reaching a customer, so every shape Indonesians type
// must normalise to one stored form.
func TestNormalizePhone(t *testing.T) {
	valid := map[string]string{
		"081234567890":      "+6281234567890",
		"0812 3456 7890":    "+6281234567890",
		"0812-3456-7890":    "+6281234567890",
		"6281234567890":     "+6281234567890",
		"+6281234567890":    "+6281234567890",
		"+62 812 3456 7890": "+6281234567890",
	}
	for in, want := range valid {
		got, err := NormalizePhone(in)
		if err != nil {
			t.Fatalf("NormalizePhone(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("NormalizePhone(%q) = %q, want %q", in, got, want)
		}
	}

	for _, bad := range []string{"", "12345", "+1 555 0100", "0212345678", "+62012345678", "abc"} {
		if _, err := NormalizePhone(bad); !errors.Is(err, ErrInvalidPhone) {
			t.Fatalf("NormalizePhone(%q) should be rejected, got %v", bad, err)
		}
	}
}

func TestMaskPhone(t *testing.T) {
	got := MaskPhone("+6281234567890")
	if got == "+6281234567890" {
		t.Fatal("masking must not return the full number")
	}
	if len(got) != len("+6281234567890") {
		t.Fatalf("mask %q should keep the length", got)
	}
}
