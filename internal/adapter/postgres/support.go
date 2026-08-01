package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
)

func moneyOf(v int64) money.Rupiah { return money.Rupiah(v) }

// ── Audit (BR-2.10.1) ────────────────────────────────────────────────────────

// AuditRepo appends privileged-action records. The table is append-only in the
// database itself (migration 0011), so a wrong entry is corrected by adding
// another, never by editing.
type AuditRepo struct{ db *gorm.DB }

func NewAuditRepo(db *gorm.DB) *AuditRepo { return &AuditRepo{db: db} }

// Entry is one audit record.
type Entry struct {
	ActorType  string
	ActorID    *uuid.UUID
	ActorEmail string
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	StoreID    *uuid.UUID
	Before     any
	After      any
	IP         string
	UserAgent  string
	RequestID  string
}

// Write appends an audit row. Failure to audit must not fail the business
// action, but it must be visible, so the caller logs the error.
func (r *AuditRepo) Write(ctx context.Context, e Entry) error {
	row := AuditLog{
		ID: uuid.New(), ActorType: e.ActorType, ActorID: e.ActorID,
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		StoreID: e.StoreID, CreatedAt: time.Now(),
	}
	if e.ActorEmail != "" {
		row.ActorEmail = &e.ActorEmail
	}
	if e.IP != "" {
		row.IP = &e.IP
	}
	if e.UserAgent != "" {
		row.UserAgent = &e.UserAgent
	}
	if e.RequestID != "" {
		row.RequestID = &e.RequestID
	}
	if e.Before != nil {
		if b, err := json.Marshal(e.Before); err == nil {
			row.Before = b
		}
	}
	if e.After != nil {
		if b, err := json.Marshal(e.After); err == nil {
			row.After = b
		}
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

// AuditFilter searches the audit log (BR-1.5.1).
type AuditFilter struct {
	Q          string
	EntityType string
	ActorID    *uuid.UUID
	StoreID    *uuid.UUID
	From, To   *time.Time
	Limit      int
}

// List returns audit rows within the caller's store scope.
func (r *AuditRepo) List(ctx context.Context, f AuditFilter, scope []uuid.UUID) ([]AuditLog, error) {
	query := r.db.WithContext(ctx).Model(&AuditLog{}).Order("created_at DESC")
	if scope != nil {
		// Group-level rows carry no store; scoped staff see only their stores'
		// rows (BR-2.7.8).
		query = query.Where("store_id IN ?", scope)
	}
	if f.EntityType != "" {
		query = query.Where("entity_type = ?", f.EntityType)
	}
	if f.ActorID != nil {
		query = query.Where("actor_id = ?", *f.ActorID)
	}
	if f.StoreID != nil {
		query = query.Where("store_id = ?", *f.StoreID)
	}
	if f.From != nil {
		query = query.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		query = query.Where("created_at <= ?", *f.To)
	}
	if f.Q != "" {
		like := "%" + f.Q + "%"
		query = query.Where("action ILIKE ? OR entity_type ILIKE ? OR actor_email ILIKE ?",
			like, like, like)
	}
	if f.Limit > 0 {
		query = query.Limit(f.Limit)
	}
	var out []AuditLog
	return out, query.Find(&out).Error
}

// ── Notifications (BR-2.10.3/4) ──────────────────────────────────────────────

// NotifyRepo queues and records notification sends. Every attempt is recorded
// so a failure is visible rather than silent.
type NotifyRepo struct{ db *gorm.DB }

func NewNotifyRepo(db *gorm.DB) *NotifyRepo { return &NotifyRepo{db: db} }

// Queue stores a message to send.
func (r *NotifyRepo) Queue(ctx context.Context, n *Notification) error {
	n.ID = uuid.New()
	n.Status = "queued"
	n.CreatedAt, n.UpdatedAt = time.Now(), time.Now()
	now := time.Now()
	n.NextAttemptAt = &now
	return r.db.WithContext(ctx).Create(n).Error
}

// Due returns notifications ready for an attempt.
func (r *NotifyRepo) Due(ctx context.Context, limit int) ([]Notification, error) {
	var out []Notification
	return out, r.db.WithContext(ctx).
		Where("status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= now())",
			[]string{"queued", "failed"}).
		Where("attempts < ?", 5).
		Order("created_at").Limit(limit).Find(&out).Error
}

// MarkSent records a successful send.
func (r *NotifyRepo) MarkSent(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&Notification{}).Where("id = ?", id).
		Updates(map[string]any{
			"status": "sent", "sent_at": now, "attempts": gorm.Expr("attempts + 1"),
			"updated_at": now,
		}).Error
}

// MarkFailed records a failure and backs off exponentially (BR-2.10.4).
func (r *NotifyRepo) MarkFailed(ctx context.Context, id uuid.UUID, cause string, attempt int) error {
	backoff := time.Duration(1<<uint(min(attempt, 6))) * time.Minute
	next := time.Now().Add(backoff)
	return r.db.WithContext(ctx).Model(&Notification{}).Where("id = ?", id).
		Updates(map[string]any{
			"status": "failed", "last_error": truncate(cause, 500),
			"attempts": gorm.Expr("attempts + 1"), "next_attempt_at": next,
			"updated_at": time.Now(),
		}).Error
}

// MarkSkipped records a message that was deliberately not sent — an opt-out or
// a disabled event switch (BR-2.10.3/4).
func (r *NotifyRepo) MarkSkipped(ctx context.Context, id uuid.UUID, reason string) error {
	return r.db.WithContext(ctx).Model(&Notification{}).Where("id = ?", id).
		Updates(map[string]any{
			"status": "skipped", "last_error": truncate(reason, 500), "updated_at": time.Now(),
		}).Error
}

// ForOrder lists an order's notifications, for support and the admin UI.
func (r *NotifyRepo) ForOrder(ctx context.Context, orderID uuid.UUID) ([]Notification, error) {
	var out []Notification
	return out, r.db.WithContext(ctx).Where("order_id = ?", orderID).
		Order("created_at").Find(&out).Error
}

// ── Idempotency (docs/04 §1) ─────────────────────────────────────────────────

// IdempotencyRepo replays the first response for a repeated key, and refuses a
// key reused with a different body.
type IdempotencyRepo struct{ db *gorm.DB }

func NewIdempotencyRepo(db *gorm.DB) *IdempotencyRepo { return &IdempotencyRepo{db: db} }

// Stored is a previously recorded response.
type Stored struct {
	Code int
	Body []byte
}

// Begin claims a key. It returns the stored response when the same key and body
// have been seen, and an IDEMPOTENCY_MISMATCH error when the body differs.
func (r *IdempotencyRepo) Begin(ctx context.Context, key, subjectType string, subjectID uuid.UUID,
	endpoint string, body []byte) (*Stored, error) {

	hash := sha256.Sum256(body)
	requestHash := hex.EncodeToString(hash[:])

	var existing IdempotencyKey
	err := r.db.WithContext(ctx).
		Where("key = ? AND subject_type = ? AND subject_id = ? AND endpoint = ?",
			key, subjectType, subjectID, endpoint).First(&existing).Error

	switch {
	case err == nil:
		if existing.RequestHash != requestHash {
			return nil, apierror.Conflict(apierror.CodeIdempotencyMismatch,
				"This Idempotency-Key was already used with a different request.")
		}
		if existing.ResponseCode == nil {
			// A concurrent identical request is still in flight.
			return nil, apierror.Conflict(apierror.CodeConflict,
				"An identical request is already being processed.")
		}
		return &Stored{Code: *existing.ResponseCode, Body: existing.ResponseBody}, nil

	case errors.Is(err, gorm.ErrRecordNotFound):
		row := IdempotencyKey{
			ID: uuid.New(), Key: key, SubjectType: subjectType, SubjectID: subjectID,
			Endpoint: endpoint, RequestHash: requestHash, CreatedAt: time.Now(),
		}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			if isUniqueViolation(err) {
				return nil, apierror.Conflict(apierror.CodeConflict,
					"An identical request is already being processed.")
			}
			return nil, err
		}
		return nil, nil

	default:
		return nil, err
	}
}

// Complete stores the response so a replay returns the same result.
//
// The body is stored as JSONB, so a replay is semantically identical rather
// than byte-identical (Postgres normalises key order). Clients switch on the
// decoded payload, never on its bytes.
//
// The body is written with an explicit ::jsonb cast: passed as bytes it would
// be sent as bytea and the assignment would fail, which is exactly the silent
// breakage that makes an "idempotent" endpoint quietly re-execute.
func (r *IdempotencyRepo) Complete(ctx context.Context, key, subjectType string, subjectID uuid.UUID,
	endpoint string, code int, body []byte) error {

	payload := string(body)
	if payload == "" {
		payload = "null"
	}
	return r.db.WithContext(ctx).Exec(`
		UPDATE idempotency_keys
		   SET response_code = $1, response_body = $2::jsonb
		 WHERE key = $3 AND subject_type = $4 AND subject_id = $5 AND endpoint = $6`,
		code, payload, key, subjectType, subjectID, endpoint).Error
}

// Abandon drops a claimed key whose request failed, so the caller may retry.
func (r *IdempotencyRepo) Abandon(ctx context.Context, key, subjectType string, subjectID uuid.UUID, endpoint string) error {
	return r.db.WithContext(ctx).
		Where("key = ? AND subject_type = ? AND subject_id = ? AND endpoint = ? AND response_code IS NULL",
			key, subjectType, subjectID, endpoint).Delete(&IdempotencyKey{}).Error
}

// Sweep removes keys older than the retention window.
func (r *IdempotencyRepo) Sweep(ctx context.Context, olderThan time.Duration) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("created_at < ?", time.Now().Add(-olderThan)).Delete(&IdempotencyKey{})
	return res.RowsAffected, res.Error
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
