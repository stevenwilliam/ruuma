package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/order"
	dpay "github.com/stevenwilliam/ruuma/internal/domain/payment"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
)

// PaymentRepo runs the finance queue and the verify/reject/refund decisions
// (BR-2.6.x). Every decision appends an immutable payment_events row.
type PaymentRepo struct {
	db     *gorm.DB
	orders *OrderRepo
}

func NewPaymentRepo(db *gorm.DB, orders *OrderRepo) *PaymentRepo {
	return &PaymentRepo{db: db, orders: orders}
}

// QueueFilter selects the finance queue (docs/04 §6).
type QueueFilter struct {
	StoreID  *uuid.UUID
	Statuses []string
	Q        string
	Limit    int
}

// QueueRow is one payment with just enough order context to decide on it.
type QueueRow struct {
	Payment      Payment   `gorm:"embedded"`
	OrderCode    string    `gorm:"column:order_code"`
	ContactName  string    `gorm:"column:contact_name"`
	ContactPhone string    `gorm:"column:contact_phone"`
	StoreName    string    `gorm:"column:store_name"`
	SlotStartsAt time.Time `gorm:"column:slot_starts_at"`
}

// Queue lists payments oldest-first so the customer who has waited longest is
// dealt with first (docs/06 §1).
func (r *PaymentRepo) Queue(ctx context.Context, f QueueFilter, scope []uuid.UUID) ([]QueueRow, error) {
	query := r.db.WithContext(ctx).Table("payments p").
		Select(`p.*, o.order_code, o.contact_name, o.contact_phone,
		        s.name AS store_name, o.slot_starts_at`).
		Joins("JOIN orders o ON o.id = p.order_id").
		Joins("JOIN stores s ON s.id = p.store_id").
		Order("p.proof_uploaded_at NULLS LAST, p.created_at")

	query = scoped(query, "p.store_id", scope)
	if f.StoreID != nil {
		query = query.Where("p.store_id = ?", *f.StoreID)
	}
	if len(f.Statuses) > 0 {
		query = query.Where("p.status IN ?", f.Statuses)
	}
	if f.Q != "" {
		like := "%" + f.Q + "%"
		query = query.Where("o.order_code ILIKE ? OR o.contact_name ILIKE ? OR p.sender_name ILIKE ?",
			like, like, like)
	}
	if f.Limit > 0 {
		query = query.Limit(f.Limit)
	}

	var out []QueueRow
	return out, query.Scan(&out).Error
}

// Get reads a payment inside the caller's store scope (BR-2.6.5).
func (r *PaymentRepo) Get(ctx context.Context, id uuid.UUID, scope []uuid.UUID) (*Payment, error) {
	var p Payment
	err := scoped(r.db.WithContext(ctx).Model(&Payment{}), "store_id", scope).
		Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Payment not found.")
	}
	return &p, err
}

// ForOrder returns an order's payment row.
func (r *PaymentRepo) ForOrder(ctx context.Context, orderID uuid.UUID) (*Payment, error) {
	var p Payment
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).
		Order("created_at DESC").First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Payment not found.")
	}
	return &p, err
}

// AttachProof records the uploaded proof and moves the order to
// AWAITING_VERIFICATION — the only way into that state (BR-2.6.4, BR-2.4.5).
func (r *PaymentRepo) AttachProof(ctx context.Context, orderID, customerID uuid.UUID,
	objectKey string, declared int64, sender string) error {

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var o Order
		if err := tx.Where("id = ? AND customer_id = ?", orderID, customerID).First(&o).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// A proof that does not resolve to the uploader's own order is
				// discarded rather than stored (BR-2.6.4).
				return apierror.NotFound("Order not found.")
			}
			return err
		}
		if o.Status != string(order.PendingPayment) {
			return apierror.Conflict(apierror.CodeIllegalTransition,
				"This order is not waiting for a transfer proof.").
				WithDetails(map[string]any{"status": o.Status})
		}

		var p Payment
		if err := tx.Where("order_id = ?", orderID).First(&p).Error; err != nil {
			return err
		}

		now := time.Now()
		if err := tx.Model(&Payment{}).Where("id = ?", p.ID).Updates(map[string]any{
			"status": string(dpay.Submitted), "proof_object_key": objectKey,
			"proof_uploaded_at": now, "declared_amount": declared, "sender_name": sender,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}

		if err := tx.Model(&Order{}).Where("id = ?", o.ID).Updates(map[string]any{
			"status": string(order.AwaitingVerification), "updated_at": now,
		}).Error; err != nil {
			return err
		}

		from := order.Status(o.Status)
		if err := appendOrderEvent(tx, o.ID, &from, order.AwaitingVerification,
			order.ActorCustomer, &customerID, "transfer proof uploaded", nil); err != nil {
			return err
		}
		return appendPaymentEvent(tx, p.ID, o.ID, "PROOF_SUBMITTED", &customerID, "customer", &declared, "")
	})
}

// Decide is the finance action being applied.
type Decide struct {
	PaymentID      uuid.UUID
	ActorID        uuid.UUID
	ActorRole      string
	IsFinance      bool
	Scope          []uuid.UUID
	AcceptMismatch bool
	MismatchReason string
}

// Verify marks a payment verified and moves the order PAID → ACCEPTED
// (BR-2.6.5 → BR-2.6.7, BR-2.4.6). It is idempotent (BR-2.6.13).
func (r *PaymentRepo) Verify(ctx context.Context, d Decide) (*Payment, error) {
	var out *Payment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, o, err := r.loadForDecision(ctx, tx, d.PaymentID, d.Scope)
		if err != nil {
			return err
		}

		decision := dpay.Decision{
			IsFinance: d.IsFinance, InScope: true, // scope already applied by the query
			ActorID: d.ActorID.String(), OrderCreator: o.CustomerID.String(),
			Status:    dpay.Status(p.Status),
			HasProof:  p.ProofObjectKey != nil,
			AmountDue: moneyOf(p.AmountDue), DeclaredAmount: moneyOf(deref(p.DeclaredAmount)),
			AcceptMismatch: d.AcceptMismatch, MismatchReason: d.MismatchReason,
		}
		result, err := dpay.Verify(decision)
		if err != nil {
			return mapPaymentError(err)
		}
		if result.AlreadyVerified {
			out = p
			return nil // replay returns the original outcome
		}

		now := time.Now()
		updates := map[string]any{
			"status": string(dpay.Verified), "verified_by": d.ActorID,
			"verified_at": now, "updated_at": now,
		}
		if result.MismatchAmount != 0 {
			updates["mismatch_accepted"] = true
			updates["mismatch_reason"] = d.MismatchReason
		}
		if err := tx.Model(&Payment{}).Where("id = ?", p.ID).Updates(updates).Error; err != nil {
			return err
		}

		// PAID then ACCEPTED: the order reaches the kitchen board only once the
		// money is confirmed (BR-2.4.6, BR-2.8.5).
		from := order.Status(o.Status)
		if err := order.Transition(from, order.Paid); err != nil {
			return apierror.Conflict(apierror.CodeIllegalTransition,
				"This order is not awaiting verification.").WithCause(err)
		}
		if err := tx.Model(&Order{}).Where("id = ?", o.ID).
			Updates(map[string]any{"status": string(order.Accepted), "updated_at": now}).Error; err != nil {
			return err
		}
		if err := appendOrderEvent(tx, o.ID, &from, order.Paid, order.ActorStaff, &d.ActorID,
			"payment verified", nil); err != nil {
			return err
		}
		paid := order.Paid
		if err := appendOrderEvent(tx, o.ID, &paid, order.Accepted, order.ActorSystem, nil,
			"accepted after payment", nil); err != nil {
			return err
		}

		amount := deref(p.DeclaredAmount)
		if err := appendPaymentEvent(tx, p.ID, o.ID, "VERIFIED", &d.ActorID, d.ActorRole,
			&amount, d.MismatchReason); err != nil {
			return err
		}
		if result.MismatchAmount != 0 {
			mismatch := int64(result.MismatchAmount)
			if err := appendPaymentEvent(tx, p.ID, o.ID, "MISMATCH_ACCEPTED", &d.ActorID,
				d.ActorRole, &mismatch, d.MismatchReason); err != nil {
				return err
			}
		}

		// Reflect what was just written, so the response carries the
		// verification timestamp rather than the pre-update nil.
		p.Status = string(dpay.Verified)
		p.VerifiedAt = &now
		p.VerifiedBy = &d.ActorID
		if result.MismatchAmount != 0 {
			p.MismatchAccepted = true
			p.MismatchReason = &d.MismatchReason
		}
		out = p
		return nil
	})
	return out, err
}

// Reject returns the order to PENDING_PAYMENT with a reason, and never touches
// the slot (BR-2.6.8, D26).
func (r *PaymentRepo) Reject(ctx context.Context, d Decide, reason dpay.RejectionReason, note string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, o, err := r.loadForDecision(ctx, tx, d.PaymentID, d.Scope)
		if err != nil {
			return err
		}

		decision := dpay.Decision{
			IsFinance: d.IsFinance, InScope: true,
			ActorID: d.ActorID.String(), OrderCreator: o.CustomerID.String(),
			Status: dpay.Status(p.Status), HasProof: p.ProofObjectKey != nil,
		}
		if err := dpay.Reject(decision, reason); err != nil {
			return mapPaymentError(err)
		}

		now := time.Now()
		if err := tx.Model(&Payment{}).Where("id = ?", p.ID).Updates(map[string]any{
			"status": string(dpay.Rejected), "rejection_reason": string(reason),
			"rejection_note": nullableString(note), "rejected_by": d.ActorID,
			"rejected_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}

		from := order.Status(o.Status)
		if err := order.Transition(from, order.PendingPayment); err != nil {
			return apierror.Conflict(apierror.CodeIllegalTransition,
				"This order is not awaiting verification.").WithCause(err)
		}
		if err := tx.Model(&Order{}).Where("id = ?", o.ID).Updates(map[string]any{
			"status": string(order.PendingPayment), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := appendOrderEvent(tx, o.ID, &from, order.PendingPayment, order.ActorStaff,
			&d.ActorID, "payment rejected: "+string(reason), nil); err != nil {
			return err
		}
		return appendPaymentEvent(tx, p.ID, o.ID, "REJECTED", &d.ActorID, d.ActorRole, nil,
			string(reason)+" "+note)
	})
}

// Refund records a refund and moves the order to REFUNDED. Capacity is not
// returned: the slot was consumed (BR-2.4.7, BR-2.6.12).
func (r *PaymentRepo) Refund(ctx context.Context, d Decide, amount int64, reference, reason, proofKey string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, o, err := r.loadForDecision(ctx, tx, d.PaymentID, d.Scope)
		if err != nil {
			return err
		}

		decision := dpay.Decision{
			IsFinance: d.IsFinance, InScope: true,
			ActorID: d.ActorID.String(), OrderCreator: o.CustomerID.String(),
			Status: dpay.Status(p.Status), HasProof: p.ProofObjectKey != nil,
			DeclaredAmount: moneyOf(deref(p.DeclaredAmount)),
		}
		if err := dpay.Refund(decision, moneyOf(amount)); err != nil {
			return mapPaymentError(err)
		}

		now := time.Now()
		if err := tx.Model(&Payment{}).Where("id = ?", p.ID).Updates(map[string]any{
			"status": string(dpay.Refunded), "refunded_amount": amount,
			"refund_reference": nullableString(reference), "refund_proof_key": nullableString(proofKey),
			"refunded_by": d.ActorID, "refunded_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}

		from := order.Status(o.Status)
		if err := order.Transition(from, order.Refunded); err != nil {
			return apierror.Conflict(apierror.CodeIllegalTransition,
				"This order cannot be refunded from its current state.").WithCause(err)
		}
		if err := tx.Model(&Order{}).Where("id = ?", o.ID).Updates(map[string]any{
			"status": string(order.Refunded), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := appendOrderEvent(tx, o.ID, &from, order.Refunded, order.ActorStaff, &d.ActorID,
			reason, nil); err != nil {
			return err
		}
		return appendPaymentEvent(tx, p.ID, o.ID, "REFUNDED", &d.ActorID, d.ActorRole, &amount, reason)
	})
}

// loadForDecision reads the payment and its order inside the caller's scope. A
// payment outside scope is 403, not 404: staff know other stores exist
// (BR-2.6.5, BR-2.7.10).
func (r *PaymentRepo) loadForDecision(ctx context.Context, tx *gorm.DB, paymentID uuid.UUID,
	scope []uuid.UUID) (*Payment, *Order, error) {

	var p Payment
	if err := tx.WithContext(ctx).First(&p, "id = ?", paymentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, apierror.NotFound("Payment not found.")
		}
		return nil, nil, err
	}
	if scope != nil && !containsUUID(scope, p.StoreID) {
		return nil, nil, apierror.Forbidden(apierror.CodeStoreOutOfScope,
			"That payment belongs to a store you do not have access to.")
	}

	var o Order
	if err := tx.WithContext(ctx).First(&o, "id = ?", p.OrderID).Error; err != nil {
		return nil, nil, err
	}
	return &p, &o, nil
}

// Events returns a payment's append-only history (BR-2.6.10).
func (r *PaymentRepo) Events(ctx context.Context, paymentID uuid.UUID) ([]PaymentEvent, error) {
	var out []PaymentEvent
	return out, r.db.WithContext(ctx).Where("payment_id = ?", paymentID).
		Order("created_at").Find(&out).Error
}

// ReconciliationRow is one line of the daily takings report (docs/06 §3).
type ReconciliationRow struct {
	StoreID     uuid.UUID `gorm:"column:store_id"`
	StoreName   string    `gorm:"column:store_name"`
	Orders      int       `gorm:"column:orders"`
	OrderTotal  int64     `gorm:"column:order_total"`
	UniqueCodes int64     `gorm:"column:unique_codes"`
	Declared    int64     `gorm:"column:declared"`
	Refunds     int64     `gorm:"column:refunds"`
	Mismatches  int       `gorm:"column:mismatches"`
	Rejections  int       `gorm:"column:rejections"`
}

// Reconciliation totals a store's day: verified takings, the kode unik
// component (BR-2.6.3), refunds, mismatches and rejections.
func (r *PaymentRepo) Reconciliation(ctx context.Context, date time.Time, storeID *uuid.UUID, scope []uuid.UUID) ([]ReconciliationRow, error) {
	var scopeArg any = nil
	if scope != nil {
		scopeArg = uuidList(scope)
	}
	var storeArg any = nil
	if storeID != nil {
		storeArg = *storeID
	}

	var out []ReconciliationRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT s.id AS store_id,
		       s.name AS store_name,
		       COUNT(*) FILTER (WHERE p.status = 'VERIFIED')::int              AS orders,
		       COALESCE(SUM(o.total) FILTER (WHERE p.status = 'VERIFIED'), 0)  AS order_total,
		       COALESCE(SUM(o.unique_code) FILTER (WHERE p.status = 'VERIFIED'), 0) AS unique_codes,
		       COALESCE(SUM(p.declared_amount) FILTER (WHERE p.status = 'VERIFIED'), 0) AS declared,
		       COALESCE(SUM(p.refunded_amount) FILTER (WHERE p.status = 'REFUNDED'), 0) AS refunds,
		       COUNT(*) FILTER (WHERE p.mismatch_accepted)::int                AS mismatches,
		       COUNT(*) FILTER (WHERE p.status = 'REJECTED')::int              AS rejections
		  FROM payments p
		  JOIN orders o ON o.id = p.order_id
		  JOIN stores s ON s.id = p.store_id
		 WHERE o.business_date = $1
		   AND ($2::uuid IS NULL OR p.store_id = $2::uuid)
		   AND ($3::uuid[] IS NULL OR p.store_id = ANY($3::uuid[]))
		 GROUP BY s.id, s.name
		 ORDER BY s.name`, date, storeArg, scopeArg).Scan(&out).Error
	return out, err
}

// OldestPending backs the finance SLA gauge (BR-2.9.1).
func (r *PaymentRepo) OldestPending(ctx context.Context) (map[uuid.UUID]time.Time, error) {
	type row struct {
		StoreID uuid.UUID
		Oldest  time.Time
	}
	var rows []row
	err := r.db.WithContext(ctx).Raw(`
		SELECT store_id, MIN(proof_uploaded_at) AS oldest
		  FROM payments WHERE status = 'SUBMITTED' AND proof_uploaded_at IS NOT NULL
		 GROUP BY store_id`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]time.Time{}
	for _, r := range rows {
		out[r.StoreID] = r.Oldest
	}
	return out, nil
}

func appendPaymentEvent(tx *gorm.DB, paymentID, orderID uuid.UUID, eventType string,
	actorID *uuid.UUID, actorRole string, amount *int64, reason string) error {

	ev := PaymentEvent{
		ID: uuid.New(), PaymentID: paymentID, OrderID: orderID,
		EventType: eventType, ActorID: actorID, Amount: amount, CreatedAt: time.Now(),
	}
	if actorRole != "" {
		ev.ActorRole = &actorRole
	}
	if strings.TrimSpace(reason) != "" {
		ev.Reason = &reason
	}
	return tx.Create(&ev).Error
}

// mapPaymentError turns a domain refusal into the right HTTP shape (docs/04 §2).
func mapPaymentError(err error) error {
	switch {
	case errors.Is(err, dpay.ErrNotFinance):
		return apierror.Forbidden(apierror.CodeForbidden, "Only finance may decide a payment.")
	case errors.Is(err, dpay.ErrOutOfScope):
		return apierror.Forbidden(apierror.CodeStoreOutOfScope,
			"That payment belongs to a store you do not have access to.")
	case errors.Is(err, dpay.ErrSelfVerification):
		return apierror.Forbidden(apierror.CodeSelfVerificationForbidden,
			"You cannot verify a payment for an order you placed.")
	case errors.Is(err, dpay.ErrNoProof):
		return apierror.Unprocessable(apierror.CodeProofRequired,
			"No transfer proof has been uploaded for this order.")
	case errors.Is(err, dpay.ErrMismatchNotAccepted):
		return apierror.Unprocessable(apierror.CodeValidation,
			"The declared amount does not match the amount due. Accept the difference with a reason to continue.")
	case errors.Is(err, dpay.ErrRejectionReasonEmpty):
		return apierror.Unprocessable(apierror.CodeRejectionReasonRequired,
			"A rejection needs a reason.")
	case errors.Is(err, dpay.ErrAlreadyVerified):
		return apierror.Conflict(apierror.CodePaymentAlreadyVerified,
			"This payment has already been verified.")
	case errors.Is(err, dpay.ErrRefundExceedsPaid):
		return apierror.Unprocessable(apierror.CodeValidation,
			"A refund cannot exceed the amount paid.")
	default:
		return err
	}
}

func containsUUID(list []uuid.UUID, v uuid.UUID) bool {
	for _, u := range list {
		if u == v {
			return true
		}
	}
	return false
}

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func deref(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
