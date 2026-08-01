//go:build security

package security_test

import (
	"net/http"
	"testing"

	"github.com/stevenwilliam/ruuma/test/testenv"
)

// TestCustomerCannotReachAnotherCustomersOrder_BR_2_7_10 is the IDOR test.
//
// The refusal is 404, not 403: telling one customer that another customer's
// order exists is itself a leak (BR-2.7.10).
func TestCustomerCannotReachAnotherCustomersOrder_BR_2_7_10(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	slot := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)
	owner := env.CustomerToken(f.Customer)
	intruder := env.CustomerToken(f.CustomerOther)

	created := env.Idempotent(http.MethodPost, "/api/v1/orders", owner,
		env.OrderBody(f.StoreA, slot))
	if created.Status != http.StatusCreated {
		t.Fatalf("setup: %d %s", created.Status, created.Raw)
	}
	orderID := created.Body["id"].(string)
	orderCode := created.Body["order_code"].(string)

	t.Run("read", func(t *testing.T) {
		res := env.Do(http.MethodGet, "/api/v1/orders/"+orderID, intruder, nil)
		if res.Status != http.StatusNotFound {
			t.Fatalf("got %d %s, want 404", res.Status, res.Raw)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		res := env.Do(http.MethodPost, "/api/v1/orders/"+orderID+"/cancel", intruder,
			map[string]any{"reason": "not mine"})
		if res.Status != http.StatusNotFound {
			t.Fatalf("got %d %s, want 404", res.Status, res.Raw)
		}
	})

	t.Run("reorder", func(t *testing.T) {
		res := env.Do(http.MethodPost, "/api/v1/orders/"+orderID+"/reorder", intruder, nil)
		if res.Status != http.StatusNotFound {
			t.Fatalf("got %d %s, want 404", res.Status, res.Raw)
		}
	})

	t.Run("track by code", func(t *testing.T) {
		// The order code alone is never enough (BR-2.7.11).
		res := env.Do(http.MethodGet, "/api/v1/orders/track?code="+orderCode, intruder, nil)
		if res.Status != http.StatusNotFound {
			t.Fatalf("tracking someone else's code: %d %s, want 404", res.Status, res.Raw)
		}
	})

	t.Run("payment proof", func(t *testing.T) {
		res := env.Do(http.MethodPost, "/api/v1/orders/"+orderID+"/payment-proof", intruder, nil)
		if res.Status == http.StatusOK || res.Status == http.StatusCreated {
			t.Fatal("uploading a proof to another customer's order must be refused")
		}
	})

	t.Run("owner still has access", func(t *testing.T) {
		res := env.Do(http.MethodGet, "/api/v1/orders/"+orderID, owner, nil)
		if res.Status != http.StatusOK {
			t.Fatalf("owner: %d %s, want 200", res.Status, res.Raw)
		}
	})
}

// TestOrderListIsScopedToTheCaller_BR_2_7_10 makes sure the list endpoint is
// filtered too — an IDOR fix on the detail route alone is not a fix.
func TestOrderListIsScopedToTheCaller_BR_2_7_10(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	slot := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)
	created := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(f.Customer), env.OrderBody(f.StoreA, slot))
	if created.Status != http.StatusCreated {
		t.Fatalf("setup: %d %s", created.Status, created.Raw)
	}

	res := env.Do(http.MethodGet, "/api/v1/orders", env.CustomerToken(f.CustomerOther), nil)
	if res.Status != http.StatusOK {
		t.Fatalf("list: %d %s", res.Status, res.Raw)
	}
	items, _ := res.Body["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("another customer's list returned %d orders, want 0", len(items))
	}
}
