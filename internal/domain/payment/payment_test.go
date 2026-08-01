package payment

import (
	"errors"
	"testing"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
)

func financeDecision() Decision {
	return Decision{
		IsFinance:      true,
		InScope:        true,
		ActorID:        "finance-1",
		OrderCreator:   "customer-9",
		Status:         Submitted,
		HasProof:       true,
		AmountDue:      214847,
		DeclaredAmount: 214847,
	}
}

// BR-2.6.2: amount due is total + kode unik, and the code is bounded 1..999.
func TestAmountDue_BR_2_6_2(t *testing.T) {
	got, err := AmountDue(214500, 347)
	if err != nil {
		t.Fatalf("AmountDue: %v", err)
	}
	if got != 214847 {
		t.Fatalf("amount due %d, want 214847", got)
	}
	for _, bad := range []int{0, -1, 1000} {
		if _, err := AmountDue(214500, bad); !errors.Is(err, ErrUniqueCodeRange) {
			t.Fatalf("unique code %d: got %v, want ErrUniqueCodeRange", bad, err)
		}
	}
}

// BR-2.6.5: only finance, and only in scope.
func TestOnlyFinanceInScope_BR_2_6_5(t *testing.T) {
	d := financeDecision()
	d.IsFinance = false
	if _, err := Verify(d); !errors.Is(err, ErrNotFinance) {
		t.Fatalf("non-finance verify: got %v, want ErrNotFinance", err)
	}

	d = financeDecision()
	d.InScope = false
	if _, err := Verify(d); !errors.Is(err, ErrOutOfScope) {
		t.Fatalf("cross-store verify: got %v, want ErrOutOfScope", err)
	}
	if err := Reject(d, NotReceived); !errors.Is(err, ErrOutOfScope) {
		t.Fatalf("cross-store reject: got %v, want ErrOutOfScope", err)
	}
}

// BR-2.6.6: a finance user may never verify a payment for an order they created.
func TestNoSelfVerification_BR_2_6_6(t *testing.T) {
	d := financeDecision()
	d.OrderCreator = d.ActorID
	if _, err := Verify(d); !errors.Is(err, ErrSelfVerification) {
		t.Fatalf("self verification: got %v, want ErrSelfVerification", err)
	}
	if err := Reject(d, Duplicate); !errors.Is(err, ErrSelfVerification) {
		t.Fatalf("self rejection: got %v, want ErrSelfVerification", err)
	}
}

// BR-2.6.4: nothing can be verified before a proof is attached.
func TestProofRequired_BR_2_6_4(t *testing.T) {
	d := financeDecision()
	d.HasProof = false
	d.Status = Pending
	if _, err := Verify(d); !errors.Is(err, ErrNoProof) {
		t.Fatalf("verify without proof: got %v, want ErrNoProof", err)
	}
}

// BR-2.6.7: an amount mismatch must be accepted explicitly, with a reason.
func TestMismatchMustBeAcceptedExplicitly_BR_2_6_7(t *testing.T) {
	// underpaid
	d := financeDecision()
	d.DeclaredAmount = d.AmountDue - 1000
	out, err := Verify(d)
	if !errors.Is(err, ErrMismatchNotAccepted) {
		t.Fatalf("underpayment: got %v, want ErrMismatchNotAccepted", err)
	}
	if out.MismatchAmount != -1000 {
		t.Fatalf("mismatch amount %d, want -1000", out.MismatchAmount)
	}

	// overpaid, accepted with a reason
	d = financeDecision()
	d.DeclaredAmount = d.AmountDue + 500
	d.AcceptMismatch = true
	d.MismatchReason = "customer rounded up"
	out, err = Verify(d)
	if err != nil {
		t.Fatalf("accepted mismatch should verify: %v", err)
	}
	if out.MismatchAmount != 500 {
		t.Fatalf("mismatch amount %d, want 500", out.MismatchAmount)
	}

	// accepting without a reason is not accepting
	d.MismatchReason = ""
	if _, err := Verify(d); !errors.Is(err, ErrMismatchNotAccepted) {
		t.Fatalf("mismatch without reason: got %v, want ErrMismatchNotAccepted", err)
	}
}

// BR-2.6.13: verification is idempotent.
func TestVerifyIsIdempotent_BR_2_6_13(t *testing.T) {
	d := financeDecision()
	d.Status = Verified
	out, err := Verify(d)
	if err != nil {
		t.Fatalf("replayed verification: %v", err)
	}
	if !out.AlreadyVerified {
		t.Fatal("replay should report AlreadyVerified, not verify again")
	}
}

// BR-2.6.8: a rejection requires a reason from the closed set.
func TestRejectionRequiresClosedSetReason_BR_2_6_8(t *testing.T) {
	d := financeDecision()
	for _, r := range ValidRejectionReasons() {
		if err := Reject(d, r); err != nil {
			t.Fatalf("reason %q should be accepted: %v", r, err)
		}
	}
	if err := Reject(d, RejectionReason("BECAUSE_I_SAID_SO")); !errors.Is(err, ErrRejectionReasonEmpty) {
		t.Fatalf("free-form reason: got %v, want ErrRejectionReasonEmpty", err)
	}
	if err := Reject(d, ""); !errors.Is(err, ErrRejectionReasonEmpty) {
		t.Fatalf("empty reason: got %v, want ErrRejectionReasonEmpty", err)
	}

	// A verified payment cannot then be rejected.
	d.Status = Verified
	if err := Reject(d, NotReceived); !errors.Is(err, ErrAlreadyVerified) {
		t.Fatalf("reject after verify: got %v, want ErrAlreadyVerified", err)
	}
}

// BR-2.6.12: refunds apply to verified payments and cannot exceed what was paid.
func TestRefund_BR_2_6_12(t *testing.T) {
	d := financeDecision()
	d.Status = Verified

	if err := Refund(d, d.DeclaredAmount); err != nil {
		t.Fatalf("full refund: %v", err)
	}
	if err := Refund(d, d.DeclaredAmount/2); err != nil {
		t.Fatalf("partial refund should be representable: %v", err)
	}
	if err := Refund(d, d.DeclaredAmount+1); !errors.Is(err, ErrRefundExceedsPaid) {
		t.Fatalf("over-refund: got %v, want ErrRefundExceedsPaid", err)
	}

	unpaid := financeDecision() // still SUBMITTED
	if err := Refund(unpaid, 1000); err == nil {
		t.Fatal("an unverified payment cannot be refunded")
	}
}

func TestClassifyMismatch(t *testing.T) {
	cases := []struct {
		declared, due money.Rupiah
		want          MismatchKind
	}{
		{214847, 214847, MismatchNone},
		{214900, 214847, MismatchOverpaid},
		{214000, 214847, MismatchUnderpaid},
	}
	for _, tc := range cases {
		if got := Classify(tc.declared, tc.due); got != tc.want {
			t.Fatalf("Classify(%d,%d) = %q, want %q", tc.declared, tc.due, got, tc.want)
		}
	}
}
