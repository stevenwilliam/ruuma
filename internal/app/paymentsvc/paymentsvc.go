// Package paymentsvc runs the manual bank-transfer flow: the customer's proof
// upload and finance's verify / reject / refund decisions (BR-2.6.x).
package paymentsvc

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/money"
	dpay "github.com/stevenwilliam/ruuma/internal/domain/payment"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

type Service struct {
	payments ports.Payments
	orders   ports.Orders
	stores   ports.Stores
	storage  ports.Storage
	notifier ports.Notifier
	params   ports.Params
	audit    ports.Auditor
	clock    ports.Clock
}

func New(payments ports.Payments, orders ports.Orders, stores ports.Stores, storage ports.Storage,
	notifier ports.Notifier, params ports.Params, audit ports.Auditor, clk ports.Clock) *Service {
	return &Service{
		payments: payments, orders: orders, stores: stores, storage: storage,
		notifier: notifier, params: params, audit: audit, clock: clk,
	}
}

// UploadProof stores a transfer proof and moves the order to
// AWAITING_VERIFICATION (BR-2.6.4, BR-2.6.11).
//
// The file is validated and re-encoded by the storage adapter; the object is
// private and only ever reachable through a short-lived presigned URL.
func (s *Service) UploadProof(ctx context.Context, orderID, customerID uuid.UUID,
	file []byte, declared money.Rupiah, sender string) (*ports.PaymentView, error) {

	if len(file) == 0 {
		return nil, apierror.Validation("Please attach your transfer proof.", nil)
	}
	if declared <= 0 {
		return nil, apierror.Validation("Please state the amount you transferred.", nil)
	}

	// Reading the order first means a proof for someone else's order is never
	// even written to storage (BR-2.6.4).
	o, err := s.orders.GetForCustomer(ctx, orderID, customerID)
	if err != nil {
		return nil, err
	}

	key, err := s.storage.PutProof(ctx, "proofs/"+o.StoreID.String(), file)
	if err != nil {
		return nil, err
	}
	if err := s.payments.AttachProof(ctx, orderID, customerID, key, declared, sender); err != nil {
		return nil, err
	}

	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "customer", ActorID: &customerID, Action: "payment.proof.upload",
		EntityType: "order", EntityID: &orderID, StoreID: &o.StoreID,
		After: map[string]any{"declared_amount": int64(declared)},
	})
	return s.payments.ForOrder(ctx, orderID)
}

// Queue lists payments awaiting a decision, oldest first (docs/06 §1).
func (s *Service) Queue(ctx context.Context, p security.Principal, storeID *uuid.UUID, status, q string, limit int) ([]ports.QueueItemView, error) {
	statuses := []string{string(dpay.Submitted)}
	switch status {
	case "all":
		statuses = nil
	case "verified":
		statuses = []string{string(dpay.Verified)}
	case "rejected":
		statuses = []string{string(dpay.Rejected)}
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if storeID != nil && !p.CanAccessStore(*storeID) {
		return nil, apierror.Forbidden(apierror.CodeStoreOutOfScope,
			"That store is outside your access.")
	}
	return s.payments.Queue(ctx, storeID, statuses, q, limit, p.StoreScope())
}

// ProofURL issues a short-lived presigned link to a proof (BR-2.6.11).
func (s *Service) ProofURL(ctx context.Context, p security.Principal, paymentID uuid.UUID) (string, error) {
	pay, err := s.payments.Get(ctx, paymentID, p.StoreScope())
	if err != nil {
		return "", err
	}
	if !pay.HasProof {
		return "", apierror.NotFound("No proof has been uploaded for this payment.")
	}
	return s.storage.PresignGet(ctx, pay.ProofObjectKey, 10*time.Minute)
}

// Verify confirms a transfer. The domain refuses self-verification, out-of-scope
// stores and silent mismatches (BR-2.6.5 → BR-2.6.7).
func (s *Service) Verify(ctx context.Context, p security.Principal, paymentID uuid.UUID,
	acceptMismatch bool, mismatchReason string) (*ports.PaymentView, error) {

	pay, err := s.payments.Verify(ctx, ports.Decision{
		PaymentID: paymentID, ActorID: p.ID, ActorRole: string(p.Role),
		IsFinance: p.Can(security.PermPaymentVerify), Scope: p.StoreScope(),
		AcceptMismatch: acceptMismatch, MismatchReason: mismatchReason,
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "payment.verify",
		EntityType: "payment", EntityID: &paymentID, StoreID: &pay.StoreID,
		After: map[string]any{
			"declared": int64(pay.DeclaredAmount), "due": int64(pay.AmountDue),
			"mismatch_accepted": acceptMismatch,
		},
	})

	// BR-2.10.3: the customer is told their payment cleared.
	s.queueNotification(ctx, pay.OrderID, "payment_verified")
	return pay, nil
}

// Reject returns the order to PENDING_PAYMENT with a reason. No automated
// message is sent — finance and operations follow up by hand (D28).
func (s *Service) Reject(ctx context.Context, p security.Principal, paymentID uuid.UUID,
	reason dpay.RejectionReason, note string) error {

	pay, err := s.payments.Get(ctx, paymentID, p.StoreScope())
	if err != nil {
		return err
	}
	if err := s.payments.Reject(ctx, ports.Decision{
		PaymentID: paymentID, ActorID: p.ID, ActorRole: string(p.Role),
		IsFinance: p.Can(security.PermPaymentVerify), Scope: p.StoreScope(),
	}, reason, note); err != nil {
		return err
	}

	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "payment.reject",
		EntityType: "payment", EntityID: &paymentID, StoreID: &pay.StoreID,
		After: map[string]any{"reason": string(reason), "note": note},
	})
}

// Refund records a refund against a verified payment (BR-2.6.12).
func (s *Service) Refund(ctx context.Context, p security.Principal, paymentID uuid.UUID,
	amount money.Rupiah, reference, reason string, proof []byte) error {

	pay, err := s.payments.Get(ctx, paymentID, p.StoreScope())
	if err != nil {
		return err
	}

	proofKey := ""
	if len(proof) > 0 {
		key, err := s.storage.PutProof(ctx, "refunds/"+pay.StoreID.String(), proof)
		if err != nil {
			return err
		}
		proofKey = key
	}

	if err := s.payments.Refund(ctx, ports.Decision{
		PaymentID: paymentID, ActorID: p.ID, ActorRole: string(p.Role),
		IsFinance: p.Can(security.PermPaymentRefund), Scope: p.StoreScope(),
	}, amount, reference, reason, proofKey); err != nil {
		return err
	}

	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "payment.refund",
		EntityType: "payment", EntityID: &paymentID, StoreID: &pay.StoreID,
		After: map[string]any{"amount": int64(amount), "reference": reference, "reason": reason},
	})
}

// Reconciliation totals a store's day (docs/06 §3).
func (s *Service) Reconciliation(ctx context.Context, p security.Principal, date time.Time, storeID *uuid.UUID) ([]ports.ReconciliationView, error) {
	if storeID != nil && !p.CanAccessStore(*storeID) {
		return nil, apierror.Forbidden(apierror.CodeStoreOutOfScope,
			"That store is outside your access.")
	}
	return s.payments.Reconciliation(ctx, date, storeID, p.StoreScope())
}

func (s *Service) queueNotification(ctx context.Context, orderID uuid.UUID, event string) {
	if !s.params.Bool(ctx, nil, "notify.event."+event+"_enabled") {
		return
	}
	o, err := s.orders.GetInScope(ctx, orderID, nil)
	if err != nil {
		return
	}
	_ = s.notifier.Queue(ctx, ports.QueuedNotification{
		OrderID: &o.ID, CustomerID: &o.CustomerID, Channel: "whatsapp",
		Provider: s.params.String(ctx, nil, "notify.provider"), Event: event,
		Target: o.ContactPhone, TemplateKey: "notify.template." + event, Language: "id",
	})
}
