// Package payment holds the manual bank-transfer rules (BR-2.6.x).
//
// Phase 1 has one method: a customer transfers, uploads a proof, and finance
// verifies or rejects it (D13, D26). Nothing here talks to a gateway; the
// provider port lives in the app layer so qris and gateway can be added without
// reshaping the order flow (BR-2.6.1, D25).
package payment

import (
	"errors"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
)

// Method is how an order is paid.
type Method string

const (
	ManualTransfer Method = "manual_transfer"
	QRIS           Method = "qris"    // phase 3
	Gateway        Method = "gateway" // phase 3
)

// Status is the payment's own state, separate from the order's (BR-2.6).
type Status string

const (
	Pending   Status = "PENDING"   // order created, nothing uploaded yet
	Submitted Status = "SUBMITTED" // proof attached, waiting for finance
	Verified  Status = "VERIFIED"
	Rejected  Status = "REJECTED"
	Refunded  Status = "REFUNDED"
)

// RejectionReason is the closed set finance must choose from (BR-2.6.8).
type RejectionReason string

const (
	AmountMismatch  RejectionReason = "AMOUNT_MISMATCH"
	ProofUnreadable RejectionReason = "PROOF_UNREADABLE"
	NotReceived     RejectionReason = "NOT_RECEIVED"
	Duplicate       RejectionReason = "DUPLICATE"
	OtherReason     RejectionReason = "OTHER"
)

// ValidRejectionReasons lists the closed set, for validation and for the UI.
func ValidRejectionReasons() []RejectionReason {
	return []RejectionReason{AmountMismatch, ProofUnreadable, NotReceived, Duplicate, OtherReason}
}

// IsValidRejectionReason reports whether r is in the closed set.
func IsValidRejectionReason(r RejectionReason) bool {
	for _, v := range ValidRejectionReasons() {
		if v == r {
			return true
		}
	}
	return false
}

var (
	ErrNoProof              = errors.New("payment: a proof must be attached before verification")
	ErrAlreadyVerified      = errors.New("payment: already verified")
	ErrSelfVerification     = errors.New("payment: a user may not verify a payment for their own order")
	ErrNotFinance           = errors.New("payment: only finance may decide a payment")
	ErrOutOfScope           = errors.New("payment: store is outside the caller's scope")
	ErrMismatchNotAccepted  = errors.New("payment: amount mismatch must be explicitly accepted with a reason")
	ErrRejectionReasonEmpty = errors.New("payment: a rejection requires a reason from the closed set")
	ErrRefundExceedsPaid    = errors.New("payment: refund exceeds the amount paid")
	ErrUniqueCodeRange      = errors.New("payment: unique code must be between 1 and 999")
)

// AmountDue is total + kode unik, the figure the customer actually transfers
// (BR-2.6.2). The unique code is what lets finance match one transfer to one
// order without a gateway.
func AmountDue(total money.Rupiah, uniqueCode int) (money.Rupiah, error) {
	if uniqueCode < 1 || uniqueCode > 999 {
		return 0, ErrUniqueCodeRange
	}
	return money.Add(total, money.Rupiah(uniqueCode))
}

// Decision describes a finance action being attempted.
type Decision struct {
	// Actor
	IsFinance    bool
	InScope      bool // the payment's store is in the actor's scope (BR-2.6.5)
	ActorID      string
	OrderCreator string // who created the order (BR-2.6.6)

	// Payment state
	Status         Status
	HasProof       bool
	AmountDue      money.Rupiah
	DeclaredAmount money.Rupiah

	// Finance input
	AcceptMismatch bool
	MismatchReason string
}

// CanDecide applies the authorization gates shared by verify and reject:
// finance only, in scope only, never your own order (BR-2.6.5, BR-2.6.6).
func CanDecide(d Decision) error {
	if !d.IsFinance {
		return ErrNotFinance
	}
	if !d.InScope {
		return ErrOutOfScope
	}
	if d.ActorID != "" && d.ActorID == d.OrderCreator {
		return ErrSelfVerification
	}
	return nil
}

// VerifyOutcome reports what a verification did.
type VerifyOutcome struct {
	AlreadyVerified bool         // idempotent replay (BR-2.6.13)
	MismatchAmount  money.Rupiah // declared − due; negative means underpaid
}

// Verify checks whether finance may verify this payment (BR-2.6.5 → BR-2.6.7,
// BR-2.6.13). It does not mutate: the app layer performs the transition inside
// a transaction and appends the payment event.
func Verify(d Decision) (VerifyOutcome, error) {
	var out VerifyOutcome

	if d.Status == Verified {
		// Idempotent: a replayed verification returns the original result
		// rather than a second PAID transition (BR-2.6.13).
		out.AlreadyVerified = true
		return out, nil
	}
	if err := CanDecide(d); err != nil {
		return out, err
	}
	if !d.HasProof || d.Status == Pending {
		return out, ErrNoProof // BR-2.6.4
	}

	diff := d.DeclaredAmount - d.AmountDue
	if diff != 0 {
		// Over or under payment can never pass silently (BR-2.6.7).
		if !d.AcceptMismatch || d.MismatchReason == "" {
			out.MismatchAmount = diff
			return out, ErrMismatchNotAccepted
		}
		out.MismatchAmount = diff
	}
	return out, nil
}

// Reject checks whether finance may reject, and that a reason was given
// (BR-2.6.8). Rejection never releases the slot — that is the order layer's
// contract (BR-2.4.7, D26).
func Reject(d Decision, reason RejectionReason) error {
	if d.Status == Verified {
		return ErrAlreadyVerified
	}
	if err := CanDecide(d); err != nil {
		return err
	}
	if !d.HasProof {
		return ErrNoProof
	}
	if !IsValidRejectionReason(reason) {
		return ErrRejectionReasonEmpty
	}
	return nil
}

// Refund validates a refund request (BR-2.6.12). Phase 1 refunds in full; the
// amount is a parameter so partial refunds need no schema change.
func Refund(d Decision, amount money.Rupiah) error {
	if err := CanDecide(d); err != nil {
		return err
	}
	if d.Status != Verified {
		return errors.New("payment: only a verified payment can be refunded")
	}
	if amount <= 0 {
		return errors.New("payment: refund amount must be positive")
	}
	if amount > d.DeclaredAmount {
		return ErrRefundExceedsPaid
	}
	return nil
}

// MismatchKind labels a difference for the reconciliation report.
type MismatchKind string

const (
	MismatchNone      MismatchKind = ""
	MismatchOverpaid  MismatchKind = "OVERPAID"
	MismatchUnderpaid MismatchKind = "UNDERPAID"
)

// Classify labels the difference between what was declared and what was due.
func Classify(declared, due money.Rupiah) MismatchKind {
	switch {
	case declared > due:
		return MismatchOverpaid
	case declared < due:
		return MismatchUnderpaid
	default:
		return MismatchNone
	}
}
