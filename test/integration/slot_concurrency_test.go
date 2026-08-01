//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/test/testenv"
)

// TestSlotCannotOversell_BR_2_3_10 is the flagship test of the whole system
// (docs/07 §2.3): N customers check out simultaneously against a slot with room
// for one, and exactly one may win.
//
// It runs against the real database, through the real router, so it exercises
// the SELECT ... FOR UPDATE (BR-2.3.8), the capacity-checked UPDATE and the
// CHECK constraints (BR-2.3.9) together.
func TestSlotCannotOversell_BR_2_3_10(t *testing.T) {
	const rounds = 10
	const racers = 20

	for round := 0; round < rounds; round++ {
		t.Run(fmt.Sprintf("round-%d", round), func(t *testing.T) {
			env := testenv.New(t)
			f := env.Fixtures

			slotID := env.MakeSlot(f.StoreA, "pickup", 12, 0, 1, 100)
			customers := env.MakeCustomers(racers)

			var wg sync.WaitGroup
			results := make([]int, racers)

			start := make(chan struct{})
			for i := 0; i < racers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start // release everyone at once
					res := env.Idempotent(http.MethodPost, "/api/v1/orders",
						env.CustomerToken(customers[i]), env.OrderBody(f.StoreA, slotID))
					results[i] = res.Status
				}(i)
			}
			close(start)
			wg.Wait()

			created, conflicts, other := 0, 0, 0
			for _, status := range results {
				switch {
				case status == http.StatusCreated:
					created++
				case status == http.StatusConflict || status == http.StatusUnprocessableEntity:
					conflicts++
				default:
					other++
				}
			}

			if created != 1 {
				t.Fatalf("%d orders created, want exactly 1", created)
			}
			if other != 0 {
				t.Fatalf("%d requests failed with an unexpected status: %v", other, results)
			}
			if conflicts != racers-1 {
				t.Fatalf("%d refusals, want %d", conflicts, racers-1)
			}

			reserved, max := env.SlotCounters(slotID)
			if reserved != 1 || reserved > max {
				t.Fatalf("slot counters: reserved=%d max=%d, want reserved 1", reserved, max)
			}
		})
	}
}

// TestKitchenUnitsAxis_BR_2_3_7 proves the second capacity axis is real: a slot
// with room for plenty of orders can still be full of cooking.
func TestKitchenUnitsAxis_BR_2_3_7(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	// Ten orders allowed, but only 8 kitchen units — and the duck weighs 8.
	slotID := env.MakeSlot(f.StoreA, "pickup", 12, 0, 10, 8)
	customers := env.MakeCustomers(2)

	first := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(customers[0]), env.OrderBodyItem(f.StoreA, slotID, f.ItemBig, 1, nil))
	if first.Status != http.StatusCreated {
		t.Fatalf("first order: %d %s", first.Status, first.Raw)
	}

	second := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(customers[1]), env.OrderBody(f.StoreA, slotID))
	if second.Status == http.StatusCreated {
		t.Fatal("second order should be refused: the slot is out of kitchen units")
	}
	if second.Code() != "SLOT_FULL" {
		t.Fatalf("got %q, want SLOT_FULL", second.Code())
	}

	reserved, _ := env.SlotCounters(slotID)
	if reserved != 1 {
		t.Fatalf("reserved orders %d, want 1", reserved)
	}
}

// TestDatabaseRefusesOversell_BR_2_3_9 bypasses the application entirely: the
// CHECK constraint must refuse an oversell even when the write does not come
// through our code.
func TestDatabaseRefusesOversell_BR_2_3_9(t *testing.T) {
	env := testenv.New(t)
	slotID := env.MakeSlot(env.Fixtures.StoreA, "pickup", 12, 0, 1, 10)

	err := env.DB.Exec(`UPDATE slots SET reserved_orders = max_orders + 1 WHERE id = ?`, slotID).Error
	if err == nil {
		t.Fatal("the database must refuse reserved_orders > max_orders (BR-2.3.9)")
	}

	err = env.DB.Exec(
		`UPDATE slots SET reserved_kitchen_units = max_kitchen_units + 1 WHERE id = ?`, slotID).Error
	if err == nil {
		t.Fatal("the database must refuse reserved_kitchen_units > max_kitchen_units (BR-2.3.9)")
	}
}

// TestCancelReleasesCapacityOnce_BR_2_3_12 covers the other half of the
// counter: a cancellation frees exactly one place, and a repeat does nothing.
func TestCancelReleasesCapacityOnce_BR_2_3_12(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	slotID := env.MakeSlot(f.StoreA, "pickup", 12, 0, 2, 100)
	customer := env.MakeCustomers(1)[0]
	token := env.CustomerToken(customer)

	res := env.Idempotent(http.MethodPost, "/api/v1/orders", token, env.OrderBody(f.StoreA, slotID))
	if res.Status != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Status, res.Raw)
	}
	orderID := uuid.MustParse(res.Body["id"].(string))

	if reserved, _ := env.SlotCounters(slotID); reserved != 1 {
		t.Fatalf("after create reserved=%d, want 1", reserved)
	}

	cancel := env.Do(http.MethodPost, "/api/v1/orders/"+orderID.String()+"/cancel", token,
		map[string]any{"reason": "changed my mind"})
	if cancel.Status != http.StatusOK {
		t.Fatalf("cancel: %d %s", cancel.Status, cancel.Raw)
	}
	if reserved, _ := env.SlotCounters(slotID); reserved != 0 {
		t.Fatalf("after cancel reserved=%d, want 0", reserved)
	}

	// Cancelling again is an illegal transition, and must not double-release.
	again := env.Do(http.MethodPost, "/api/v1/orders/"+orderID.String()+"/cancel", token,
		map[string]any{"reason": "again"})
	if again.Status == http.StatusOK {
		t.Fatal("a second cancellation must be refused")
	}
	if reserved, _ := env.SlotCounters(slotID); reserved != 0 {
		t.Fatalf("after double cancel reserved=%d, want 0", reserved)
	}
}

// TestNoAutoExpiry_BR_2_3_11 pins the phase-1 decision: nothing releases an
// unpaid order's capacity except a human (D25).
func TestNoAutoExpiry_BR_2_3_11(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	slotID := env.MakeSlot(f.StoreA, "pickup", 12, 0, 1, 100)
	customer := env.MakeCustomers(1)[0]

	res := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(customer), env.OrderBody(f.StoreA, slotID))
	if res.Status != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Status, res.Raw)
	}

	// The order is unpaid. Simulate the passage of time by ageing the row: no
	// job exists that would act on it, so capacity must still be held.
	if err := env.DB.Exec(
		`UPDATE orders SET created_at = now() - interval '30 days' WHERE id = ?`,
		res.Body["id"]).Error; err != nil {
		t.Fatalf("age order: %v", err)
	}

	reserved, _ := env.SlotCounters(slotID)
	if reserved != 1 {
		t.Fatalf("unpaid order released its slot after 30 days (reserved=%d) — phase 1 has no auto-cancel", reserved)
	}
}
