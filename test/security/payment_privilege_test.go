//go:build security

package security_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/platform/security"
	"github.com/stevenwilliam/ruuma/test/testenv"
)

// submitOrderWithProof creates an order and attaches a proof, leaving it in the
// finance queue.
func submitOrderWithProof(t *testing.T, env *testenv.Env, storeID uuid.UUID) (orderID, paymentID string, amountDue int64) {
	t.Helper()
	f := env.Fixtures

	slot := env.MakeSlot(storeID, "pickup", 12, 0, 5, 100)
	created := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(f.Customer), env.OrderBody(storeID, slot))
	if created.Status != http.StatusCreated {
		t.Fatalf("create order: %d %s", created.Status, created.Raw)
	}
	orderID = created.Body["id"].(string)
	amountDue = int64(created.Body["amount_due"].(float64))

	env.AttachProof(orderID, f.Customer, amountDue)

	paymentID = env.PaymentIDForOrder(orderID)
	return orderID, paymentID, amountDue
}

// TestOnlyFinanceMayVerify_BR_2_6_5 walks every other role.
func TestOnlyFinanceMayVerify_BR_2_6_5(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	_, paymentID, _ := submitOrderWithProof(t, env, f.StoreA)

	for _, c := range []struct {
		name string
		id   uuid.UUID
		role security.Role
	}{
		{"kitchen", f.KitchenA, security.RoleKitchen},
		{"counter", f.CounterA, security.RoleCounter},
		{"store_manager", f.ManagerA, security.RoleStoreManager},
		{"customer", f.Customer, security.RoleCustomer},
	} {
		t.Run(c.name, func(t *testing.T) {
			subject := security.SubjectStaff
			if c.role == security.RoleCustomer {
				subject = security.SubjectCustomer
			}
			res := env.Idempotent(http.MethodPost,
				"/api/v1/finance/payments/"+paymentID+"/verify",
				env.Token(subject, c.id, c.role), map[string]any{})
			if res.Status != http.StatusForbidden {
				t.Fatalf("%s verify: %d %s, want 403", c.name, res.Status, res.Raw)
			}
		})
	}

	t.Run("finance in scope succeeds", func(t *testing.T) {
		res := env.Idempotent(http.MethodPost,
			"/api/v1/finance/payments/"+paymentID+"/verify",
			env.StaffToken(f.FinanceA, security.RoleFinance), map[string]any{})
		if res.Status != http.StatusOK {
			t.Fatalf("finance verify: %d %s, want 200", res.Status, res.Raw)
		}
	})
}

// TestFinanceCannotVerifyOutOfScope_BR_2_6_5 is the tenancy half of the same
// rule: the right role, the wrong store.
func TestFinanceCannotVerifyOutOfScope_BR_2_6_5(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	_, paymentID, _ := submitOrderWithProof(t, env, f.StoreB)

	res := env.Idempotent(http.MethodPost,
		"/api/v1/finance/payments/"+paymentID+"/verify",
		env.StaffToken(f.FinanceA, security.RoleFinance), map[string]any{})
	if res.Status != http.StatusForbidden {
		t.Fatalf("store-A finance verifying a store-B payment: %d %s, want 403", res.Status, res.Raw)
	}
	if res.Code() != "STORE_OUT_OF_SCOPE" {
		t.Fatalf("code %q, want STORE_OUT_OF_SCOPE", res.Code())
	}

	// Group-scoped finance may (BR-2.7.7).
	ok := env.Idempotent(http.MethodPost,
		"/api/v1/finance/payments/"+paymentID+"/verify",
		env.StaffToken(f.FinanceGroup, security.RoleFinance), map[string]any{})
	if ok.Status != http.StatusOK {
		t.Fatalf("group finance verifying store B: %d %s, want 200", ok.Status, ok.Raw)
	}
}

// TestAmountMismatchNeedsExplicitAcceptance_BR_2_6_7 proves a mismatch cannot
// pass silently.
func TestAmountMismatchNeedsExplicitAcceptance_BR_2_6_7(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	slot := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)
	created := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(f.Customer), env.OrderBody(f.StoreA, slot))
	orderID := created.Body["id"].(string)
	amountDue := int64(created.Body["amount_due"].(float64))

	// The customer declares 10.000 less than they owe.
	env.AttachProof(orderID, f.Customer, amountDue-10000)
	paymentID := env.PaymentIDForOrder(orderID)
	financeToken := env.StaffToken(f.FinanceA, security.RoleFinance)

	res := env.Idempotent(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		financeToken, map[string]any{})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("silent mismatch: %d %s, want 422", res.Status, res.Raw)
	}

	// Accepting without a reason is not accepting.
	res = env.Idempotent(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		financeToken, map[string]any{"accept_mismatch": true})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("acceptance without a reason: %d %s, want 422", res.Status, res.Raw)
	}

	// With a reason it goes through, and the reason is recorded.
	res = env.Idempotent(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		financeToken, map[string]any{
			"accept_mismatch": true, "mismatch_reason": "customer transferred the old total",
		})
	if res.Status != http.StatusOK {
		t.Fatalf("accepted mismatch: %d %s, want 200", res.Status, res.Raw)
	}
}

// TestRejectionNeedsAReason_BR_2_6_8 and keeps the slot (D26).
func TestRejectionNeedsAReason_BR_2_6_8(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	slot := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)
	created := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(f.Customer), env.OrderBody(f.StoreA, slot))
	orderID := created.Body["id"].(string)
	env.AttachProof(orderID, f.Customer, int64(created.Body["amount_due"].(float64)))
	paymentID := env.PaymentIDForOrder(orderID)
	financeToken := env.StaffToken(f.FinanceA, security.RoleFinance)

	before, _ := env.SlotCounters(slot)

	res := env.Idempotent(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/reject",
		financeToken, map[string]any{})
	if res.Status != http.StatusUnprocessableEntity && res.Status != http.StatusBadRequest {
		t.Fatalf("rejection without a reason: %d %s, want refusal", res.Status, res.Raw)
	}

	res = env.Idempotent(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/reject",
		financeToken, map[string]any{"reason": "NOT_RECEIVED", "note": "nothing arrived"})
	if res.Status != http.StatusOK {
		t.Fatalf("rejection with a reason: %d %s, want 200", res.Status, res.Raw)
	}

	// The slot is still held: rejection is not cancellation (D26).
	after, _ := env.SlotCounters(slot)
	if after != before {
		t.Fatalf("slot released on rejection (%d → %d) — it must not be", before, after)
	}

	// And no automated message went out (D28).
	for _, msg := range env.Notify.Queued {
		if msg.Event == "payment_rejected" {
			t.Fatal("a rejection must not queue an automated message (D28)")
		}
	}

	// The customer can see why, and upload again.
	order := env.Do(http.MethodGet, "/api/v1/orders/"+orderID, env.CustomerToken(f.Customer), nil)
	payment, _ := order.Body["payment"].(map[string]any)
	if payment["rejection_reason"] != "NOT_RECEIVED" {
		t.Fatalf("customer cannot see the rejection reason: %v", payment)
	}
	if order.Body["status"] != "PENDING_PAYMENT" {
		t.Fatalf("status %v, want PENDING_PAYMENT so the customer can retry", order.Body["status"])
	}
}

// TestVerifyIsIdempotent_BR_2_6_13 — a replayed key returns the first outcome.
func TestVerifyIsIdempotent_BR_2_6_13(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	_, paymentID, _ := submitOrderWithProof(t, env, f.StoreA)
	financeToken := env.StaffToken(f.FinanceA, security.RoleFinance)

	key := uuid.NewString()
	first := env.Do(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		financeToken, map[string]any{}, [2]string{"Idempotency-Key", key})
	if first.Status != http.StatusOK {
		t.Fatalf("first verify: %d %s", first.Status, first.Raw)
	}

	second := env.Do(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		financeToken, map[string]any{}, [2]string{"Idempotency-Key", key})
	if second.Status != http.StatusOK {
		t.Fatalf("replay: %d %s, want 200", second.Status, second.Raw)
	}
	// The replay is semantically the first response; the stored payload is
	// JSONB, so key order is not part of the contract.
	if !reflect.DeepEqual(first.Body, second.Body) {
		t.Fatalf("replay body %v, want the original %v", second.Body, first.Body)
	}
	if second.Body["verified_at"] == nil {
		t.Fatal("a verified payment must report when it was verified")
	}

	// A different body under the same key is a conflict (docs/04 §1).
	mismatch := env.Do(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		financeToken, map[string]any{"accept_mismatch": true, "mismatch_reason": "x"},
		[2]string{"Idempotency-Key", key})
	if mismatch.Status != http.StatusConflict {
		t.Fatalf("same key, different body: %d %s, want 409", mismatch.Status, mismatch.Raw)
	}
}

// TestIdempotencyKeyRequired_docs04 — money endpoints refuse to run without one.
func TestIdempotencyKeyRequired_docs04(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	slot := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)

	res := env.Do(http.MethodPost, "/api/v1/orders", env.CustomerToken(f.Customer),
		env.OrderBody(f.StoreA, slot))
	if res.Status != http.StatusBadRequest {
		t.Fatalf("order without Idempotency-Key: %d %s, want 400", res.Status, res.Raw)
	}
}
