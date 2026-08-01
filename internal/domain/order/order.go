// Package order holds the order state machine (BR-2.4.x).
//
// The transition table below is the whole rule: anything not listed is illegal
// and returns 409 (BR-2.4.3). Keeping it as data rather than a switch means the
// test can walk every pair exhaustively.
package order

import (
	"fmt"
	"sort"
)

// Status is an order's state (BR-2.4.1).
type Status string

const (
	Draft                Status = "DRAFT"
	PendingPayment       Status = "PENDING_PAYMENT"
	AwaitingVerification Status = "AWAITING_VERIFICATION"
	Paid                 Status = "PAID"
	Accepted             Status = "ACCEPTED"
	InKitchen            Status = "IN_KITCHEN"
	Ready                Status = "READY"
	PickedUp             Status = "PICKED_UP"
	OutForDelivery       Status = "OUT_FOR_DELIVERY" // phase 2
	Delivered            Status = "DELIVERED"        // phase 2
	Completed            Status = "COMPLETED"
	Cancelled            Status = "CANCELLED"
	Refunded             Status = "REFUNDED"
)

// AllStatuses lists every state, so tests can walk the full matrix.
func AllStatuses() []Status {
	return []Status{
		Draft, PendingPayment, AwaitingVerification, Paid, Accepted, InKitchen,
		Ready, PickedUp, OutForDelivery, Delivered, Completed, Cancelled, Refunded,
	}
}

// transitions is the legal transition table (BR-2.4.2).
var transitions = map[Status][]Status{
	Draft:                {PendingPayment, Cancelled},
	PendingPayment:       {AwaitingVerification, Cancelled},
	AwaitingVerification: {Paid, PendingPayment, Cancelled}, // back to PENDING on rejection (D26)
	Paid:                 {Accepted, Cancelled, Refunded},
	Accepted:             {InKitchen, Cancelled, Refunded},
	InKitchen:            {Ready, Cancelled, Refunded},
	Ready:                {PickedUp, OutForDelivery, Cancelled, Refunded},
	PickedUp:             {Completed, Refunded},
	OutForDelivery:       {Delivered, Cancelled, Refunded},
	Delivered:            {Completed, Refunded},
	Completed:            {Refunded},
	Cancelled:            {},
	Refunded:             {},
}

// ErrIllegalTransition is returned for any pair not in the table. The caller
// maps it to 409 with both states in the details (BR-2.4.3).
type ErrIllegalTransition struct {
	From Status
	To   Status
}

func (e ErrIllegalTransition) Error() string {
	return fmt.Sprintf("order: illegal transition %s → %s", e.From, e.To)
}

// CanTransition reports whether from → to is legal.
func CanTransition(from, to Status) bool {
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Transition validates a transition, returning ErrIllegalTransition if refused.
func Transition(from, to Status) error {
	if !CanTransition(from, to) {
		return ErrIllegalTransition{From: from, To: to}
	}
	return nil
}

// NextStates lists the legal targets from a state, sorted for stable output.
func NextStates(from Status) []Status {
	out := append([]Status(nil), transitions[from]...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// IsTerminal reports whether a state has no outgoing transitions.
func IsTerminal(s Status) bool { return len(transitions[s]) == 0 }

// HoldsCapacity reports whether an order in this state is still occupying its
// slot. Capacity is released exactly on CANCELLED and never otherwise — a
// refunded order consumed the slot it was cooked for (BR-2.3.12, BR-2.4.7).
func HoldsCapacity(s Status) bool {
	switch s {
	case Cancelled:
		return false
	default:
		return true
	}
}

// IsUnpaid reports whether the order is still waiting for money. These are the
// orders that appear in the ops "unpaid, ageing" list, because phase 1 has no
// auto-cancel (BR-2.3.11, BR-2.3.15, D25).
func IsUnpaid(s Status) bool {
	return s == PendingPayment || s == AwaitingVerification
}

// VisibleToKitchen reports whether the kitchen board should show the order.
// Unpaid orders never reach the kitchen (BR-2.8.5).
func VisibleToKitchen(s Status) bool {
	switch s {
	case Accepted, InKitchen, Ready:
		return true
	default:
		return false
	}
}

// CustomerCancellable reports whether a customer may cancel from this state at
// all; the time window is checked separately (BR-2.3.13).
func CustomerCancellable(s Status) bool {
	switch s {
	case Draft, PendingPayment, AwaitingVerification, Paid, Accepted:
		return true
	default:
		return false
	}
}

// Immutable reports whether the order's lines, slot and prices are frozen. They
// are frozen from creation onward: a change means cancel and re-order
// (BR-2.4.8).
func Immutable(s Status) bool { return s != Draft }

// ActorType is who performed a transition (BR-2.4.4).
type ActorType string

const (
	ActorCustomer ActorType = "customer"
	ActorStaff    ActorType = "staff"
	ActorSystem   ActorType = "system"
)

// Event is one appended history row.
type Event struct {
	From      Status
	To        Status
	ActorType ActorType
	ActorID   string
	Reason    string
}
