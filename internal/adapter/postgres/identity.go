package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/identity"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// CustomerRepo owns customer accounts and their sign-in identities (D24).
type CustomerRepo struct{ db *gorm.DB }

func NewCustomerRepo(db *gorm.DB) *CustomerRepo { return &CustomerRepo{db: db} }

func (r *CustomerRepo) Get(ctx context.Context, id uuid.UUID) (*Customer, error) {
	var c Customer
	err := r.db.WithContext(ctx).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Customer not found.")
	}
	return &c, err
}

// ByEmail looks up an account. The caller must not reveal the outcome to an
// unauthenticated client — auth responses are identical either way (docs/12 A07).
func (r *CustomerRepo) ByEmail(ctx context.Context, email string) (*Customer, error) {
	var c Customer
	err := r.db.WithContext(ctx).Where("email = ?", strings.TrimSpace(email)).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

func (r *CustomerRepo) ByPhone(ctx context.Context, phone string) (*Customer, error) {
	var c Customer
	err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

// ByIdentity finds the account behind a provider identity.
func (r *CustomerRepo) ByIdentity(ctx context.Context, provider identity.Provider, providerUserID string) (*Customer, error) {
	var ci CustomerIdentity
	err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", string(provider), providerUserID).
		First(&ci).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, ci.CustomerID)
}

func (r *CustomerRepo) Create(ctx context.Context, c *Customer) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.PreferredLanguage == "" {
		c.PreferredLanguage = "id"
	}
	c.IsActive = true
	c.CreatedAt, c.UpdatedAt = time.Now(), time.Now()
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *CustomerRepo) Update(ctx context.Context, c *Customer) error {
	c.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(c).Error
}

// LinkIdentity attaches a provider identity to a customer (BR-2.7.3). The
// caller decides whether linking is allowed via identity.DecideLink; this only
// writes the row.
func (r *CustomerRepo) LinkIdentity(ctx context.Context, customerID uuid.UUID,
	provider identity.Provider, providerUserID, email string, verified bool) error {

	row := CustomerIdentity{
		ID: uuid.New(), CustomerID: customerID, Provider: string(provider),
		ProviderUserID: providerUserID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if email != "" {
		row.Email = &email
	}
	if verified {
		now := time.Now()
		row.VerifiedAt = &now
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO customer_identities
			(id, customer_id, provider, provider_user_id, email, verified_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6, now(), now())
		ON CONFLICT (provider, provider_user_id) DO UPDATE
		SET email = EXCLUDED.email, verified_at = COALESCE(customer_identities.verified_at, EXCLUDED.verified_at),
		    updated_at = now()`,
		row.ID, row.CustomerID, row.Provider, row.ProviderUserID, row.Email, row.VerifiedAt).Error
}

// Identities lists a customer's sign-in methods.
func (r *CustomerRepo) Identities(ctx context.Context, customerID uuid.UUID) ([]CustomerIdentity, error) {
	var out []CustomerIdentity
	return out, r.db.WithContext(ctx).Where("customer_id = ?", customerID).Find(&out).Error
}

// MarkPhoneVerified opens the gate on ordering (BR-2.7.4).
func (r *CustomerRepo) MarkPhoneVerified(ctx context.Context, customerID uuid.UUID, phone string) error {
	return r.db.WithContext(ctx).Model(&Customer{}).Where("id = ?", customerID).
		Updates(map[string]any{
			"phone": phone, "phone_verified_at": time.Now(), "updated_at": time.Now(),
		}).Error
}

func (r *CustomerRepo) MarkEmailVerified(ctx context.Context, customerID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&Customer{}).Where("id = ?", customerID).
		Updates(map[string]any{"email_verified_at": time.Now(), "updated_at": time.Now()}).Error
}

// RecordFailedLogin drives progressive lockout (docs/12 A07).
func (r *CustomerRepo) RecordFailedLogin(ctx context.Context, customerID uuid.UUID, lockAfter int, lockFor time.Duration) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE customers
		   SET failed_attempts = failed_attempts + 1,
		       locked_until = CASE WHEN failed_attempts + 1 >= $2 THEN now() + $3::interval ELSE locked_until END,
		       updated_at = now()
		 WHERE id = $1`, customerID, lockAfter, lockFor.String()).Error
}

func (r *CustomerRepo) ClearFailedLogins(ctx context.Context, customerID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&Customer{}).Where("id = ?", customerID).
		Updates(map[string]any{"failed_attempts": 0, "locked_until": nil, "updated_at": time.Now()}).Error
}

// ── Staff ────────────────────────────────────────────────────────────────────

// UserRepo owns staff accounts (BR-2.7.6, BR-2.7.12).
type UserRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Get(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := r.db.WithContext(ctx).First(&u, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("User not found.")
	}
	return &u, err
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.WithContext(ctx).Where("email = ?", strings.TrimSpace(email)).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

// List returns staff, searchable (BR-1.5.1).
func (r *UserRepo) List(ctx context.Context, q string) ([]User, error) {
	query := r.db.WithContext(ctx).Model(&User{}).Order("full_name")
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("full_name ILIKE ? OR email ILIKE ? OR role ILIKE ?", like, like, like)
	}
	var out []User
	return out, query.Find(&out).Error
}

func (r *UserRepo) Create(ctx context.Context, u *User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	u.CreatedAt, u.UpdatedAt = time.Now(), time.Now()
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *UserRepo) Update(ctx context.Context, u *User) error {
	u.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(u).Error
}

// Deactivate never deletes: the audit trail must survive the person
// (docs/06 §2.7).
func (r *UserRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{"is_active": false, "updated_at": time.Now()}).Error
}

func (r *UserRepo) RecordFailedLogin(ctx context.Context, userID uuid.UUID, lockAfter int, lockFor time.Duration) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE users
		   SET failed_attempts = failed_attempts + 1,
		       locked_until = CASE WHEN failed_attempts + 1 >= $2 THEN now() + $3::interval ELSE locked_until END,
		       updated_at = now()
		 WHERE id = $1`, userID, lockAfter, lockFor.String()).Error
}

func (r *UserRepo) RecordLogin(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Updates(map[string]any{
			"failed_attempts": 0, "locked_until": nil,
			"last_login_at": time.Now(), "updated_at": time.Now(),
		}).Error
}

// ── Tokens and OTP ───────────────────────────────────────────────────────────

// TokenRepo stores hashed refresh tokens, verification tokens and OTPs
// (BR-2.7.5, BR-2.7.12).
type TokenRepo struct{ db *gorm.DB }

func NewTokenRepo(db *gorm.DB) *TokenRepo { return &TokenRepo{db: db} }

// StoreRefresh records a rotating refresh token by hash.
func (r *TokenRepo) StoreRefresh(ctx context.Context, subjectType string, subjectID uuid.UUID,
	rawToken string, jti uuid.UUID, parent *uuid.UUID, ttl time.Duration, ua, ip string) error {

	row := RefreshToken{
		ID: uuid.New(), SubjectType: subjectType, SubjectID: subjectID,
		TokenHash: security.HashToken(rawToken), JTI: jti, ParentJTI: parent,
		ExpiresAt: time.Now().Add(ttl), CreatedAt: time.Now(),
	}
	if ua != "" {
		row.UserAgent = &ua
	}
	if ip != "" {
		row.IP = &ip
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

// ConsumeRefresh validates and rotates a refresh token in one step. Reusing an
// already-revoked token revokes the whole chain — the classic replay defence
// (docs/12 A02).
func (r *TokenRepo) ConsumeRefresh(ctx context.Context, rawToken string) (*RefreshToken, error) {
	hash := security.HashToken(rawToken)

	var out *RefreshToken
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t RefreshToken
		if err := tx.Where("token_hash = ?", hash).First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apierror.Unauthorized("Session expired. Please sign in again.")
			}
			return err
		}
		if t.RevokedAt != nil {
			// Replay of a rotated token: kill every live session for the subject.
			if err := tx.Model(&RefreshToken{}).
				Where("subject_type = ? AND subject_id = ? AND revoked_at IS NULL", t.SubjectType, t.SubjectID).
				Update("revoked_at", time.Now()).Error; err != nil {
				return err
			}
			return apierror.Unauthorized("Session expired. Please sign in again.")
		}
		if !time.Now().Before(t.ExpiresAt) {
			return apierror.Unauthorized("Session expired. Please sign in again.")
		}
		if err := tx.Model(&RefreshToken{}).Where("id = ?", t.ID).
			Update("revoked_at", time.Now()).Error; err != nil {
			return err
		}
		out = &t
		return nil
	})
	return out, err
}

// RevokeAllRefresh ends every session for a subject — used on logout and on any
// privilege change (BR-2.7.12).
func (r *TokenRepo) RevokeAllRefresh(ctx context.Context, subjectType string, subjectID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&RefreshToken{}).
		Where("subject_type = ? AND subject_id = ? AND revoked_at IS NULL", subjectType, subjectID).
		Update("revoked_at", time.Now()).Error
}

// CreateVerification stores an email-verification or password-reset token by
// hash.
func (r *TokenRepo) CreateVerification(ctx context.Context, subjectType string, subjectID uuid.UUID,
	rawToken, purpose string, ttl time.Duration) error {

	return r.db.WithContext(ctx).Create(&VerificationToken{
		ID: uuid.New(), SubjectType: subjectType, SubjectID: subjectID,
		TokenHash: security.HashToken(rawToken), Purpose: purpose,
		ExpiresAt: time.Now().Add(ttl), CreatedAt: time.Now(),
	}).Error
}

// ConsumeVerification validates a token once.
func (r *TokenRepo) ConsumeVerification(ctx context.Context, rawToken, purpose string) (*VerificationToken, error) {
	hash := security.HashToken(rawToken)

	var out *VerificationToken
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t VerificationToken
		if err := tx.Where("token_hash = ? AND purpose = ?", hash, purpose).First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apierror.Unprocessable(apierror.CodeValidation, "That link is not valid.")
			}
			return err
		}
		if t.ConsumedAt != nil || !time.Now().Before(t.ExpiresAt) {
			return apierror.Unprocessable(apierror.CodeValidation, "That link has expired.")
		}
		if err := tx.Model(&VerificationToken{}).Where("id = ?", t.ID).
			Update("consumed_at", time.Now()).Error; err != nil {
			return err
		}
		out = &t
		return nil
	})
	return out, err
}

// CreateOTP stores a hashed one-time code (BR-2.7.5).
func (r *TokenRepo) CreateOTP(ctx context.Context, phone, codeHash, purpose string, ttl time.Duration, ip string) error {
	row := OTPCode{
		ID: uuid.New(), Phone: phone, CodeHash: codeHash, Purpose: purpose,
		ExpiresAt: time.Now().Add(ttl), CreatedAt: time.Now(),
	}
	if ip != "" {
		row.RequestIP = &ip
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

// LatestOTP returns the newest live code for a phone and purpose.
func (r *TokenRepo) LatestOTP(ctx context.Context, phone, purpose string) (*OTPCode, error) {
	var o OTPCode
	err := r.db.WithContext(ctx).
		Where("phone = ? AND purpose = ? AND consumed_at IS NULL", phone, purpose).
		Order("created_at DESC").First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &o, err
}

// RecordOTPAttempt increments the attempt counter (BR-2.7.5).
func (r *TokenRepo) RecordOTPAttempt(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&OTPCode{}).Where("id = ?", id).
		Update("attempts", gorm.Expr("attempts + 1")).Error
}

// ConsumeOTP marks a code used; it can never be presented twice.
func (r *TokenRepo) ConsumeOTP(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&OTPCode{}).
		Where("id = ? AND consumed_at IS NULL", id).
		Update("consumed_at", time.Now()).Error
}

// SweepExpired clears spent auth rows; run by the worker.
func (r *TokenRepo) SweepExpired(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Where("expires_at < now()").Delete(&OTPCode{}).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Where("expires_at < now()").Delete(&VerificationToken{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Where("expires_at < now() - interval '30 days'").Delete(&RefreshToken{}).Error
}
