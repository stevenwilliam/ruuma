package order

import (
	"errors"
	"testing"
)

// legal is the transition table restated independently of the implementation,
// so the test would catch an edit to the map rather than agreeing with it.
var legal = map[Status]map[Status]bool{
	Draft:                {PendingPayment: true, Cancelled: true},
	PendingPayment:       {AwaitingVerification: true, Cancelled: true},
	AwaitingVerification: {Paid: true, PendingPayment: true, Cancelled: true},
	Paid:                 {Accepted: true, Cancelled: true, Refunded: true},
	Accepted:             {InKitchen: true, Cancelled: true, Refunded: true},
	InKitchen:            {Ready: true, Cancelled: true, Refunded: true},
	Ready:                {PickedUp: true, OutForDelivery: true, Cancelled: true, Refunded: true},
	PickedUp:             {Completed: true, Refunded: true},
	OutForDelivery:       {Delivered: true, Cancelled: true, Refunded: true},
	Delivered:            {Completed: true, Refunded: true},
	Completed:            {Refunded: true},
	Cancelled:            {},
	Refunded:             {},
}

// BR-2.4.2 / BR-2.4.3: every legal pair succeeds and every other pair — all
// 13×13 of them — is refused.
func TestTransitionMatrixIsExhaustive_BR_2_4_2_BR_2_4_3(t *testing.T) {
	all := AllStatuses()
	if len(all) != 13 {
		t.Fatalf("expected 13 states, got %d", len(all))
	}
	for _, from := range all {
		for _, to := range all {
			want := legal[from][to]
			err := Transition(from, to)
			got := err == nil
			if got != want {
				t.Fatalf("%s → %s: allowed=%v, want %v", from, to, got, want)
			}
			if !want {
				var illegal ErrIllegalTransition
				if !errors.As(err, &illegal) || illegal.From != from || illegal.To != to {
					t.Fatalf("%s → %s: error should name both states, got %v", from, to, err)
				}
			}
		}
	}
}

// A state may never transition to itself — re-sending a status must be a 409,
// not a silent second event (BR-2.4.4).
func TestNoSelfTransitions_BR_2_4_3(t *testing.T) {
	for _, s := range AllStatuses() {
		if CanTransition(s, s) {
			t.Fatalf("%s → %s should be illegal", s, s)
		}
	}
}

// D26: finance rejection returns the order to PENDING_PAYMENT so the customer
// can upload a new proof; it does not cancel and does not free the slot.
func TestRejectionReturnsToPendingPayment_D26(t *testing.T) {
	if err := Transition(AwaitingVerification, PendingPayment); err != nil {
		t.Fatalf("rejection path must be legal: %v", err)
	}
	if !HoldsCapacity(PendingPayment) {
		t.Fatal("a rejected order still holds its slot (BR-2.6.8)")
	}
}

// BR-2.3.12 / BR-2.4.7: capacity is released on CANCELLED and only there.
func TestCapacityReleasedOnlyOnCancel_BR_2_3_12(t *testing.T) {
	for _, s := range AllStatuses() {
		want := s != Cancelled
		if got := HoldsCapacity(s); got != want {
			t.Fatalf("HoldsCapacity(%s) = %v, want %v", s, got, want)
		}
	}
	if HoldsCapacity(Refunded) != true {
		t.Fatal("a refunded order consumed its slot; capacity is not returned")
	}
}

// BR-2.8.5: unpaid orders never reach the kitchen board.
func TestKitchenVisibility_BR_2_8_5(t *testing.T) {
	visible := map[Status]bool{Accepted: true, InKitchen: true, Ready: true}
	for _, s := range AllStatuses() {
		if got := VisibleToKitchen(s); got != visible[s] {
			t.Fatalf("VisibleToKitchen(%s) = %v, want %v", s, got, visible[s])
		}
	}
	for _, s := range []Status{PendingPayment, AwaitingVerification} {
		if VisibleToKitchen(s) {
			t.Fatalf("%s must not be cooked", s)
		}
	}
}

// BR-2.3.11 / D25: the unpaid set is what the ops ageing list works from, since
// nothing expires automatically in phase 1.
func TestUnpaidSet_BR_2_3_11(t *testing.T) {
	unpaid := map[Status]bool{PendingPayment: true, AwaitingVerification: true}
	for _, s := range AllStatuses() {
		if got := IsUnpaid(s); got != unpaid[s] {
			t.Fatalf("IsUnpaid(%s) = %v, want %v", s, got, unpaid[s])
		}
	}
}

func TestTerminalStates(t *testing.T) {
	for _, s := range AllStatuses() {
		want := s == Cancelled || s == Refunded
		if got := IsTerminal(s); got != want {
			t.Fatalf("IsTerminal(%s) = %v, want %v", s, got, want)
		}
	}
}

// BR-2.4.8: everything after DRAFT is frozen.
func TestImmutability_BR_2_4_8(t *testing.T) {
	if Immutable(Draft) {
		t.Fatal("a draft is still editable")
	}
	for _, s := range AllStatuses() {
		if s == Draft {
			continue
		}
		if !Immutable(s) {
			t.Fatalf("%s must be immutable", s)
		}
	}
}

// BR-2.3.13: which states a customer may cancel from (the time window is
// enforced separately by the schedule package).
func TestCustomerCancellableStates_BR_2_3_13(t *testing.T) {
	allowed := map[Status]bool{
		Draft: true, PendingPayment: true, AwaitingVerification: true,
		Paid: true, Accepted: true,
	}
	for _, s := range AllStatuses() {
		if got := CustomerCancellable(s); got != allowed[s] {
			t.Fatalf("CustomerCancellable(%s) = %v, want %v", s, got, allowed[s])
		}
	}
	if CustomerCancellable(InKitchen) {
		t.Fatal("once it is cooking, only staff may cancel")
	}
}
