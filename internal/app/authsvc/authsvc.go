// Package authsvc implements the four customer sign-in methods and staff login
// (D24, BR-2.7.x).
//
// Every failure path returns the same shape, so the endpoints cannot be used to
// discover which accounts exist (docs/12, A07).
package authsvc

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/identity"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/id"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// A customer may sign in with email+password, Google, Instagram or phone+OTP
// (BR-2.7.2, D24); every method converges on one customer record through
// customer_identities.

// OAuthProfile is what a provider tells us about a person.
type OAuthProfile struct {
	ProviderUserID string
	Email          string
	EmailVerified  bool
	FullName       string
}

// OAuthClient exchanges an authorization code for a profile. Google and
// Instagram implementations live in the adapter layer; both stay disabled until
// credentials exist (docs/00 Q8).
type OAuthClient interface {
	AuthorizeURL(state string) (string, error)
	Exchange(ctx context.Context, code string) (*OAuthProfile, error)
	Configured() bool
}

type Service struct {
	customers ports.Customers
	staff     ports.Staff
	tokens    ports.Tokens
	notifier  ports.Notifier
	mailer    ports.Mailer
	params    ports.Params
	audit     ports.Auditor
	clock     ports.Clock
	signer    *security.TokenSigner
	baseURL   string
	oauth     map[identity.Provider]OAuthClient
}

func New(customers ports.Customers, staff ports.Staff, tokens ports.Tokens, notifier ports.Notifier,
	mailer ports.Mailer, params ports.Params, audit ports.Auditor, clk ports.Clock,
	signer *security.TokenSigner, baseURL string, oauth map[identity.Provider]OAuthClient) *Service {
	return &Service{
		customers: customers, staff: staff, tokens: tokens, notifier: notifier,
		mailer: mailer, params: params, audit: audit, clock: clk, signer: signer,
		baseURL: baseURL, oauth: oauth,
	}
}

// Session is what a successful sign-in returns.
type Session struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	Customer     *ports.CustomerView
	Staff        *ports.StaffView
}

// genericAuthError is returned for every failed credential check, so the
// response cannot distinguish "no such account" from "wrong password".
func genericAuthError() error {
	return apierror.Unauthorized("Those sign-in details are not correct.")
}

// Register creates an email+password account and sends a verification link
// (D24). Registering an existing email returns success without creating
// anything — the mail is what tells the real owner.
func (s *Service) Register(ctx context.Context, email, password, fullName string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return apierror.Validation("Please enter a valid email address.", nil)
	}
	if err := security.ValidatePassword(password); err != nil {
		return apierror.Validation(err.Error(), map[string]any{"min_length": security.MinPasswordLength})
	}

	existing, err := s.customers.ByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // no enumeration
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	created, err := s.customers.Create(ctx, ports.CustomerView{
		FullName: fullName, Email: email, PreferredLanguage: "id",
	}, hash)
	if err != nil {
		return err
	}
	if err := s.customers.LinkIdentity(ctx, created.ID, identity.Password, email, email, false); err != nil {
		return err
	}
	return s.sendEmailVerification(ctx, created.ID, email)
}

func (s *Service) sendEmailVerification(ctx context.Context, customerID uuid.UUID, email string) error {
	token, err := id.Token(32)
	if err != nil {
		return err
	}
	if err := s.tokens.CreateVerification(ctx, "customer", customerID, token, "verify_email", 24*time.Hour); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/verify-email?token=%s", strings.TrimRight(s.baseURL, "/"), token)
	return s.mailer.Send(ctx, email, "Verify your ruuma account",
		"Welcome to ruuma.\n\nConfirm your email address to finish signing up:\n"+link+
			"\n\nThe link is valid for 24 hours.")
}

// VerifyEmail consumes an emailed token.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	_, customerID, err := s.tokens.ConsumeVerification(ctx, token, "verify_email")
	if err != nil {
		return err
	}
	return s.customers.MarkEmailVerified(ctx, customerID)
}

// Login authenticates email + password.
func (s *Service) Login(ctx context.Context, email, password, ua, ip string) (*Session, error) {
	c, err := s.customers.ByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		return nil, err
	}
	if c == nil {
		// Hash anyway so the timing of a missing account matches a wrong
		// password (docs/12, A07).
		_, _ = security.HashPassword(password)
		return nil, genericAuthError()
	}
	if c.LockedUntil != nil && s.clock.Now().Before(*c.LockedUntil) {
		return nil, apierror.Forbidden(apierror.CodeAccountLocked,
			"This account is temporarily locked. Please try again later.")
	}
	if c.EmailVerifiedAt == nil {
		return nil, apierror.Forbidden(apierror.CodeValidation,
			"Please verify your email address first — check your inbox.")
	}

	hash, err := s.customers.PasswordHash(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	ok, err := security.VerifyPassword(password, hash)
	if err != nil || !ok {
		_ = s.customers.RecordFailedLogin(ctx, c.ID, 8, 15*time.Minute)
		return nil, genericAuthError()
	}
	_ = s.customers.ClearFailedLogins(ctx, c.ID)
	return s.issueCustomerSession(ctx, c, ua, ip)
}

// RequestOTP issues a one-time code over the notify provider (BR-2.7.5).
func (s *Service) RequestOTP(ctx context.Context, rawPhone, purpose, ip string) error {
	phone, err := identity.NormalizePhone(rawPhone)
	if err != nil {
		return apierror.Validation("Please enter a valid Indonesian mobile number.", nil)
	}

	code, err := sixDigits()
	if err != nil {
		return err
	}
	ttl := time.Duration(s.params.Int(ctx, nil, "auth.otp_ttl_minutes")) * time.Minute
	if err := s.tokens.CreateOTP(ctx, phone, security.HashToken(code), purpose, ttl, ip); err != nil {
		return err
	}

	// The code goes over the same provider as order notifications (D11/Q4).
	return s.notifier.Queue(ctx, ports.QueuedNotification{
		Channel: "whatsapp", Provider: s.params.String(ctx, nil, "notify.provider"),
		Event: "otp", Target: phone, TemplateKey: "notify.template.otp",
		Language: "id", Body: "Kode verifikasi ruuma Anda: " + code + ". Berlaku " +
			fmt.Sprint(int(ttl.Minutes())) + " menit. Jangan bagikan kode ini.",
	})
}

// VerifyOTP consumes a code: it signs a customer in, or verifies the phone of an
// already-signed-in customer (BR-2.7.4/5).
func (s *Service) VerifyOTP(ctx context.Context, rawPhone, code, purpose string,
	existing *uuid.UUID, ua, ip string) (*Session, error) {

	phone, err := identity.NormalizePhone(rawPhone)
	if err != nil {
		return nil, apierror.Validation("Please enter a valid Indonesian mobile number.", nil)
	}

	stored, err := s.tokens.LatestOTP(ctx, phone, purpose)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, apierror.Unprocessable(apierror.CodeValidation, "That code is not valid.")
	}
	stored.OTP.MaxAttempts = s.params.Int(ctx, nil, "auth.otp_max_attempts")

	if err := identity.CheckOTP(stored.OTP, stored.CodeHash, security.HashToken(code), s.clock.Now()); err != nil {
		_ = s.tokens.RecordOTPAttempt(ctx, stored.ID)
		// One message for every failure mode: wrong, expired, used, exhausted.
		return nil, apierror.Unprocessable(apierror.CodeValidation, "That code is not valid.")
	}
	if err := s.tokens.ConsumeOTP(ctx, stored.ID); err != nil {
		return nil, err
	}

	// Verifying the phone of the signed-in customer (BR-2.7.4).
	if existing != nil {
		if err := s.customers.MarkPhoneVerified(ctx, *existing, phone); err != nil {
			return nil, err
		}
		if err := s.customers.LinkIdentity(ctx, *existing, identity.Phone, phone, "", true); err != nil {
			return nil, err
		}
		c, err := s.customers.Get(ctx, *existing)
		if err != nil {
			return nil, err
		}
		return s.issueCustomerSession(ctx, c, ua, ip)
	}

	// Phone sign-in: link to an existing verified phone, else create.
	c, err := s.customers.ByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}
	if c == nil {
		c, err = s.customers.Create(ctx, ports.CustomerView{
			FullName: "", Phone: phone, PreferredLanguage: "id",
		}, "")
		if err != nil {
			return nil, err
		}
	}
	if err := s.customers.MarkPhoneVerified(ctx, c.ID, phone); err != nil {
		return nil, err
	}
	if err := s.customers.LinkIdentity(ctx, c.ID, identity.Phone, phone, "", true); err != nil {
		return nil, err
	}
	c, err = s.customers.Get(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	return s.issueCustomerSession(ctx, c, ua, ip)
}

// OAuthStart returns a provider's authorize URL.
func (s *Service) OAuthStart(ctx context.Context, provider identity.Provider, state string) (string, error) {
	client, ok := s.oauth[provider]
	if !ok || client == nil || !client.Configured() {
		return "", apierror.Unprocessable(apierror.CodeValidation,
			"That sign-in method is not available yet.")
	}
	if !s.params.Bool(ctx, nil, "auth.provider_"+string(provider)+"_enabled") {
		return "", apierror.Unprocessable(apierror.CodeValidation,
			"That sign-in method is not available yet.")
	}
	return client.AuthorizeURL(state)
}

// OAuthCallback exchanges a code and links or creates an account (BR-2.7.3).
func (s *Service) OAuthCallback(ctx context.Context, provider identity.Provider, code, ua, ip string) (*Session, error) {
	client, ok := s.oauth[provider]
	if !ok || client == nil || !client.Configured() {
		return nil, apierror.Unprocessable(apierror.CodeValidation,
			"That sign-in method is not available yet.")
	}
	profile, err := client.Exchange(ctx, code)
	if err != nil {
		return nil, apierror.Unauthorized("That sign-in did not complete.")
	}

	if c, err := s.customers.ByIdentity(ctx, provider, profile.ProviderUserID); err != nil {
		return nil, err
	} else if c != nil {
		return s.issueCustomerSession(ctx, c, ua, ip)
	}

	var existing *ports.CustomerView
	if profile.Email != "" {
		existing, err = s.customers.ByEmail(ctx, profile.Email)
		if err != nil {
			return nil, err
		}
	}

	in := identity.IncomingIdentity{
		Provider: provider, ProviderUserID: profile.ProviderUserID,
		Email: profile.Email, EmailVerified: profile.EmailVerified,
	}
	var account *identity.ExistingAccount
	if existing != nil {
		account = &identity.ExistingAccount{
			CustomerID: existing.ID.String(), Email: existing.Email,
			EmailVerifiedAt: existing.EmailVerifiedAt, Phone: existing.Phone,
			PhoneVerifiedAt: existing.PhoneVerifiedAt,
		}
	}

	switch identity.DecideLink(in, account) {
	case identity.LinkToExisting:
		if err := s.customers.LinkIdentity(ctx, existing.ID, provider,
			profile.ProviderUserID, profile.Email, profile.EmailVerified); err != nil {
			return nil, err
		}
		return s.issueCustomerSession(ctx, existing, ua, ip)

	case identity.RefuseLink:
		// An unverified match must never inherit an existing account.
		return nil, apierror.Forbidden(apierror.CodeForbidden,
			"An account already uses that email. Please sign in with your password instead.")

	default:
		created, err := s.customers.Create(ctx, ports.CustomerView{
			FullName: profile.FullName, Email: profile.Email, PreferredLanguage: "id",
		}, "")
		if err != nil {
			return nil, err
		}
		if profile.EmailVerified {
			if err := s.customers.MarkEmailVerified(ctx, created.ID); err != nil {
				return nil, err
			}
		}
		if err := s.customers.LinkIdentity(ctx, created.ID, provider,
			profile.ProviderUserID, profile.Email, profile.EmailVerified); err != nil {
			return nil, err
		}
		return s.issueCustomerSession(ctx, created, ua, ip)
	}
}

// StaffLogin authenticates a staff account.
func (s *Service) StaffLogin(ctx context.Context, email, password, ua, ip string) (*Session, error) {
	u, err := s.staff.ByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		return nil, err
	}
	if u == nil {
		_, _ = security.HashPassword(password)
		return nil, genericAuthError()
	}
	if !u.IsActive {
		return nil, genericAuthError()
	}
	if u.LockedUntil != nil && s.clock.Now().Before(*u.LockedUntil) {
		return nil, apierror.Forbidden(apierror.CodeAccountLocked,
			"This account is temporarily locked. Please try again later.")
	}

	hash, err := s.staff.PasswordHash(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	ok, err := security.VerifyPassword(password, hash)
	if err != nil || !ok {
		_ = s.staff.RecordFailedLogin(ctx, u.ID, 5, 15*time.Minute)
		return nil, genericAuthError()
	}
	if err := s.staff.RecordLogin(ctx, u.ID); err != nil {
		return nil, err
	}

	access, refresh, err := s.issue(ctx, security.SubjectStaff, u.ID, security.Role(u.Role), ua, ip)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &u.ID, ActorEmail: u.Email,
		Action: "staff.login", EntityType: "user", EntityID: &u.ID, IP: ip, UserAgent: ua,
	})
	return &Session{
		AccessToken: access, RefreshToken: refresh,
		ExpiresIn: s.params.Int(ctx, nil, "auth.access_token_minutes") * 60, Staff: u,
	}, nil
}

// Refresh rotates a refresh token (BR-2.7.12). Reuse of a rotated token kills
// the whole chain in the repository.
func (s *Service) Refresh(ctx context.Context, raw, ua, ip string) (*Session, error) {
	subjectType, subjectID, jti, err := s.tokens.ConsumeRefresh(ctx, raw)
	if err != nil {
		return nil, err
	}

	if subjectType == "user" || subjectType == string(security.SubjectStaff) {
		u, err := s.staff.Get(ctx, subjectID)
		if err != nil {
			return nil, err
		}
		access, refresh, err := s.issueWithParent(ctx, security.SubjectStaff, u.ID,
			security.Role(u.Role), ua, ip, &jti)
		if err != nil {
			return nil, err
		}
		return &Session{AccessToken: access, RefreshToken: refresh,
			ExpiresIn: s.params.Int(ctx, nil, "auth.access_token_minutes") * 60, Staff: u}, nil
	}

	c, err := s.customers.Get(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	access, refresh, err := s.issueWithParent(ctx, security.SubjectCustomer, c.ID,
		security.RoleCustomer, ua, ip, &jti)
	if err != nil {
		return nil, err
	}
	return &Session{AccessToken: access, RefreshToken: refresh,
		ExpiresIn: s.params.Int(ctx, nil, "auth.access_token_minutes") * 60, Customer: c}, nil
}

// Logout revokes every refresh token for the subject.
func (s *Service) Logout(ctx context.Context, subjectType security.SubjectType, subjectID uuid.UUID) error {
	return s.tokens.RevokeAllRefresh(ctx, string(subjectType), subjectID)
}

func (s *Service) issueCustomerSession(ctx context.Context, c *ports.CustomerView, ua, ip string) (*Session, error) {
	access, refresh, err := s.issue(ctx, security.SubjectCustomer, c.ID, security.RoleCustomer, ua, ip)
	if err != nil {
		return nil, err
	}
	return &Session{
		AccessToken: access, RefreshToken: refresh,
		ExpiresIn: s.params.Int(ctx, nil, "auth.access_token_minutes") * 60, Customer: c,
	}, nil
}

func (s *Service) issue(ctx context.Context, subject security.SubjectType, id uuid.UUID,
	role security.Role, ua, ip string) (string, string, error) {
	return s.issueWithParent(ctx, subject, id, role, ua, ip, nil)
}

func (s *Service) issueWithParent(ctx context.Context, subject security.SubjectType, subjectID uuid.UUID,
	role security.Role, ua, ip string, parent *uuid.UUID) (string, string, error) {

	access, _, err := s.signer.Issue(subject, subjectID, role)
	if err != nil {
		return "", "", err
	}
	refresh, err := randomToken()
	if err != nil {
		return "", "", err
	}
	ttl := time.Duration(s.params.Int(ctx, nil, "auth.refresh_token_days")) * 24 * time.Hour
	if err := s.tokens.StoreRefresh(ctx, string(subject), subjectID, refresh, uuid.New(), parent, ttl, ua, ip); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func randomToken() (string, error) { return id.Token(48) }

func sixDigits() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
