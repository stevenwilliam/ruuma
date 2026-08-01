// Package identity holds the customer sign-in rules: OTP lifecycle, identity
// linking across providers, and the phone-verification gate on ordering
// (BR-2.7.1–5, D24).
package identity

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Provider is a sign-in method (D24).
type Provider string

const (
	Password  Provider = "password"
	Google    Provider = "google"
	Instagram Provider = "instagram"
	Phone     Provider = "phone"
)

// AllProviders lists the four supported methods.
func AllProviders() []Provider { return []Provider{Password, Google, Instagram, Phone} }

var (
	ErrPhoneVerificationRequired = errors.New("identity: a verified phone is required before ordering")
	ErrOTPExpired                = errors.New("identity: otp expired")
	ErrOTPConsumed               = errors.New("identity: otp already used")
	ErrOTPAttemptsExceeded       = errors.New("identity: too many attempts")
	ErrOTPMismatch               = errors.New("identity: otp does not match")
	ErrInvalidPhone              = errors.New("identity: phone number is not a valid Indonesian mobile number")
	ErrUnverifiedLink            = errors.New("identity: cannot link an unverified identity to an existing account")
)

// CanPlaceOrder enforces BR-2.7.4: whatever the sign-in method, the counter
// must be able to reach the customer, so a verified phone gates ordering.
//
// This is the rule Instagram sign-in runs into — Instagram Login returns no
// email and no phone, so those customers must add and verify one first
// (docs/00 Q8).
func CanPlaceOrder(phoneVerifiedAt *time.Time) error {
	if phoneVerifiedAt == nil || phoneVerifiedAt.IsZero() {
		return ErrPhoneVerificationRequired
	}
	return nil
}

// ── Identity linking ─────────────────────────────────────────────────────────

// ExistingAccount is what the app layer found when looking up a sign-in
// attempt's email or phone.
type ExistingAccount struct {
	CustomerID      string
	Email           string
	EmailVerifiedAt *time.Time
	Phone           string
	PhoneVerifiedAt *time.Time
}

// IncomingIdentity is the identity being presented by a provider.
type IncomingIdentity struct {
	Provider       Provider
	ProviderUserID string
	Email          string
	EmailVerified  bool // as asserted by the provider (Google sets this; Instagram cannot)
	Phone          string
	PhoneVerified  bool
}

// LinkDecision is what the app layer should do.
type LinkDecision string

const (
	LinkToExisting LinkDecision = "LINK_TO_EXISTING"
	CreateNew      LinkDecision = "CREATE_NEW"
	RefuseLink     LinkDecision = "REFUSE_LINK"
)

// DecideLink implements BR-2.7.3: an identity joins an existing customer only
// on a verified matching email or a verified matching phone. An unverified
// claim never links — otherwise anyone could register "someone@else.com" with a
// provider that does not verify, and inherit their order history.
func DecideLink(in IncomingIdentity, existing *ExistingAccount) LinkDecision {
	if existing == nil {
		return CreateNew
	}

	if in.Email != "" && strings.EqualFold(in.Email, existing.Email) {
		if in.EmailVerified && existing.EmailVerifiedAt != nil {
			return LinkToExisting
		}
		return RefuseLink
	}

	if in.Phone != "" && in.Phone == existing.Phone {
		if in.PhoneVerified && existing.PhoneVerifiedAt != nil {
			return LinkToExisting
		}
		return RefuseLink
	}

	return CreateNew
}

// ── OTP ──────────────────────────────────────────────────────────────────────

// OTPPurpose is why a code was issued.
type OTPPurpose string

const (
	PurposeSignup      OTPPurpose = "signup"
	PurposeLogin       OTPPurpose = "login"
	PurposeVerifyPhone OTPPurpose = "verify_phone"
)

// OTP is a stored one-time code. The code itself is never held here — only its
// hash lives in the database (BR-2.7.5).
type OTP struct {
	Purpose     OTPPurpose
	Attempts    int
	MaxAttempts int
	ConsumedAt  *time.Time
	ExpiresAt   time.Time
}

// CheckOTP validates a code's usability and whether the supplied hash matches.
//
// The caller maps every failure to one identical client response — a wrong,
// expired, used or over-attempted code must be indistinguishable, or the
// endpoint becomes an oracle (BR-2.7.5, docs/12 A07).
func CheckOTP(o OTP, storedHash, presentedHash string, now time.Time) error {
	if o.ConsumedAt != nil {
		return ErrOTPConsumed
	}
	if !now.Before(o.ExpiresAt) {
		return ErrOTPExpired
	}
	if o.MaxAttempts > 0 && o.Attempts >= o.MaxAttempts {
		return ErrOTPAttemptsExceeded
	}
	if storedHash == "" || presentedHash != storedHash {
		return ErrOTPMismatch
	}
	return nil
}

// ── Phone numbers ────────────────────────────────────────────────────────────

var indonesianMobile = regexp.MustCompile(`^\+628[1-9][0-9]{6,11}$`)

// NormalizePhone converts the shapes Indonesians actually type — 08…, 62…,
// +62…, with spaces or dashes — into E.164. The phone is the identity key for
// reaching a customer, so it must be stored one way only.
func NormalizePhone(raw string) (string, error) {
	s := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if r == '+' {
			return r
		}
		return -1
	}, raw)

	switch {
	case strings.HasPrefix(s, "+62"):
		// already E.164
	case strings.HasPrefix(s, "62"):
		s = "+" + s
	case strings.HasPrefix(s, "0"):
		s = "+62" + strings.TrimPrefix(s, "0")
	default:
		return "", ErrInvalidPhone
	}

	if !indonesianMobile.MatchString(s) {
		return "", ErrInvalidPhone
	}
	return s, nil
}

// MaskPhone renders a phone for display without exposing it in full — used in
// admin lists and never in logs, where it is redacted entirely.
func MaskPhone(e164 string) string {
	if len(e164) < 6 {
		return "***"
	}
	return e164[:5] + strings.Repeat("*", len(e164)-8) + e164[len(e164)-3:]
}
