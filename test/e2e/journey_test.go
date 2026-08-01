//go:build e2e

package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/platform/security"
	"github.com/stevenwilliam/ruuma/test/testenv"
)

// TestDefinitionOfDone walks the journey named in the build brief, end to end,
// through the real API:
//
//	choose a store → order across three cuisines → pick a date and a slot that
//	respects the store's closed weekdays, capacity and cutoffs → upload a
//	transfer proof → finance verifies → the confirmation is queued → the kitchen
//	sees only its own store → an admin changes a configurable value with no
//	deploy.
func TestDefinitionOfDone(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	admin := env.StaffToken(f.Admin, security.RoleAdmin)

	// ── The group sells three cuisines ───────────────────────────────────────
	chinese := createCategory(t, env, admin, "Tionghoa", "Chinese", "chinese-e2e", "chinese")
	western := createCategory(t, env, admin, "Barat", "Western", "western-e2e", "western")
	kwetiau := createItem(t, env, admin, chinese, "E2E-CHN", "Kwetiau Sapi", "Beef Noodles", 56000)
	burger := createItem(t, env, admin, western, "E2E-WST", "Beef Burger", "Beef Burger", 72000)

	// ── 1. The customer sees the stores and their honest state ───────────────
	stores := env.Do(http.MethodGet, "/api/v1/stores", "", nil)
	if stores.Status != http.StatusOK {
		t.Fatalf("stores: %d %s", stores.Status, stores.Raw)
	}
	if len(stores.Body["items"].([]any)) < 2 {
		t.Fatal("expected both test stores to be listed")
	}

	// ── 2. The closed store offers nothing ───────────────────────────────────
	// Store B closes at the weekend. Its Sunday must produce no bookable slots
	// at all — for any mode (BR-2.1.4).
	sunday := env.SundayISO()
	closed := env.Do(http.MethodGet,
		"/api/v1/availability/slots?store_id="+f.StoreB.String()+"&date="+sunday+"&type=pickup", "", nil)
	if closed.Status != http.StatusUnprocessableEntity {
		t.Fatalf("closed weekday returned %d %s, want a refusal", closed.Status, closed.Raw)
	}
	if closed.Code() != "DATE_NOT_BOOKABLE" {
		t.Fatalf("closed weekday code %q, want DATE_NOT_BOOKABLE", closed.Code())
	}

	// ── 3. The open store offers slots, and the customer takes one ───────────
	slot := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)
	customerToken := env.CustomerToken(f.Customer)

	menu := env.Do(http.MethodGet, "/api/v1/menu?store_id="+f.StoreA.String(), "", nil)
	if menu.Status != http.StatusOK {
		t.Fatalf("menu: %d %s", menu.Status, menu.Raw)
	}

	body := map[string]any{
		"store_id":        f.StoreA,
		"slot_id":         slot,
		"fulfilment_type": "pickup",
		"contact_name":    "Budi Test",
		"contact_phone":   "+628111111111",
		"lines": []map[string]any{
			{"menu_item_id": f.ItemMain, "qty": 1, "option_choice_ids": []uuid.UUID{f.RiceWhite}},
			{"menu_item_id": kwetiau, "qty": 2},
			{"menu_item_id": burger, "qty": 1},
		},
	}

	created := env.Idempotent(http.MethodPost, "/api/v1/orders", customerToken, body)
	if created.Status != http.StatusCreated {
		t.Fatalf("create order: %d %s", created.Status, created.Raw)
	}

	orderID := created.Body["id"].(string)
	orderCode := created.Body["order_code"].(string)
	total := int64(created.Body["total"].(float64))
	uniqueCode := int(created.Body["unique_code"].(float64))
	amountDue := int64(created.Body["amount_due"].(float64))

	// The customer transfers the exact amount, including the kode unik that
	// makes their transfer identifiable (BR-2.6.2).
	if amountDue != total+int64(uniqueCode) {
		t.Fatalf("amount due %d ≠ total %d + kode unik %d", amountDue, total, uniqueCode)
	}
	if uniqueCode < 1 || uniqueCode > 999 {
		t.Fatalf("kode unik %d out of range", uniqueCode)
	}
	if len(orderCode) != 8 {
		t.Fatalf("order code %q is not the agreed 8-character form", orderCode)
	}
	if created.Body["bank_account"] == nil {
		t.Fatal("checkout did not return the store's bank account")
	}

	// Prices are the server's: 50.000 + 2×56.000 + 72.000 = 234.000, +10% PB1.
	subtotal := int64(created.Body["subtotal"].(float64))
	tax := int64(created.Body["tax"].(float64))
	if subtotal != 234000 {
		t.Fatalf("subtotal %d, want 234000", subtotal)
	}
	if tax != 23400 {
		t.Fatalf("PB1 %d, want 23400", tax)
	}
	if total != subtotal+tax {
		t.Fatalf("total %d ≠ subtotal %d + tax %d", total, subtotal, tax)
	}

	// ── 4. The customer uploads a transfer proof ─────────────────────────────
	env.AttachProof(orderID, f.Customer, amountDue)

	tracked := env.Do(http.MethodGet, "/api/v1/orders/track?code="+orderCode, customerToken, nil)
	if tracked.Status != http.StatusOK {
		t.Fatalf("track: %d %s", tracked.Status, tracked.Raw)
	}
	if tracked.Body["status"] != "AWAITING_VERIFICATION" {
		t.Fatalf("status %v, want AWAITING_VERIFICATION", tracked.Body["status"])
	}

	// ── 5. Finance verifies, and only the right finance user can ─────────────
	paymentID := env.PaymentIDForOrder(orderID)

	wrongStore := env.Idempotent(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		env.StaffToken(f.KitchenB, security.RoleKitchen), map[string]any{})
	if wrongStore.Status != http.StatusForbidden {
		t.Fatalf("kitchen staff verifying a payment: %d, want 403", wrongStore.Status)
	}

	verified := env.Idempotent(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		env.StaffToken(f.FinanceA, security.RoleFinance), map[string]any{})
	if verified.Status != http.StatusOK {
		t.Fatalf("verify: %d %s", verified.Status, verified.Raw)
	}

	// ── 6. The customer is told, over WhatsApp ───────────────────────────────
	var confirmed bool
	for _, msg := range env.Notify.Queued {
		if msg.Event == "payment_verified" {
			confirmed = true
			if msg.Target != "+628111111111" {
				t.Fatalf("notification target %q", msg.Target)
			}
		}
	}
	if !confirmed {
		t.Fatal("no payment-verified notification was queued (BR-2.10.3)")
	}

	// The order-received message carries the exact amount and the kode unik —
	// without them the customer cannot pay correctly (BR-2.6.2, BR-2.10.3).
	var instructions string
	for _, msg := range env.Notify.Queued {
		if msg.Event == "order_received" {
			instructions = msg.Body
		}
	}
	if instructions == "" {
		t.Fatal("no order-received message was queued")
	}
	for _, want := range []string{orderCode, "214.847"[0:0] + formatRupiahLike(amountDue)} {
		if !strings.Contains(instructions, want) {
			t.Fatalf("order-received message is missing %q: %s", want, instructions)
		}
	}

	// ── 7. The kitchen sees its own store, and only its own ──────────────────
	kitchenA := env.StaffToken(f.KitchenA, security.RoleKitchen)
	board := env.Do(http.MethodGet, "/api/v1/ops/orders?store_id="+f.StoreA.String(), kitchenA, nil)
	if board.Status != http.StatusOK {
		t.Fatalf("board: %d %s", board.Status, board.Raw)
	}
	groups := board.Body["items"].([]any)
	if len(groups) == 0 {
		t.Fatal("the verified order did not reach the kitchen board")
	}

	// Store B's kitchen sees nothing of store A (BR-2.7.8).
	foreign := env.Do(http.MethodGet, "/api/v1/ops/orders?store_id="+f.StoreA.String(),
		env.StaffToken(f.KitchenB, security.RoleKitchen), nil)
	if foreign.Status != http.StatusForbidden {
		t.Fatalf("store B kitchen reading store A: %d, want 403", foreign.Status)
	}

	// The production summary aggregates what to cook (BR-2.8.2).
	production := env.Do(http.MethodGet, "/api/v1/ops/slots/"+slot.String()+"/production", kitchenA, nil)
	if production.Status != http.StatusOK {
		t.Fatalf("production: %d %s", production.Status, production.Raw)
	}
	if len(production.Body["items"].([]any)) == 0 {
		t.Fatal("empty production summary for a slot with an order")
	}

	// ── 8. Kitchen cooks, counter hands over ─────────────────────────────────
	for _, step := range []struct {
		status string
		token  string
	}{
		{"IN_KITCHEN", kitchenA},
		{"READY", kitchenA},
		{"PICKED_UP", env.StaffToken(f.CounterA, security.RoleCounter)},
	} {
		res := env.Idempotent(http.MethodPost, "/api/v1/ops/orders/"+orderID+"/status",
			step.token, map[string]any{"status": step.status})
		if res.Status != http.StatusOK {
			t.Fatalf("advance to %s: %d %s", step.status, res.Status, res.Raw)
		}
	}

	final := env.Do(http.MethodGet, "/api/v1/orders/"+orderID, customerToken, nil)
	if final.Body["status"] != "PICKED_UP" {
		t.Fatalf("final status %v, want PICKED_UP", final.Body["status"])
	}

	// The customer can see the whole history (BR-2.4.4).
	history, _ := final.Body["history"].([]any)
	if len(history) < 5 {
		t.Fatalf("history has %d entries, want the full trail", len(history))
	}

	// ── 9. The admin retunes the business without a deploy ───────────────────
	change := env.Do(http.MethodPut, "/api/v1/admin/sys-parameters", admin,
		map[string]any{"key": "pricing.tax_bps", "value": "1100"})
	if change.Status != http.StatusOK {
		t.Fatalf("parameter change: %d %s", change.Status, change.Raw)
	}

	slot2 := env.MakeSlot(f.StoreA, "pickup", 14, 0, 5, 100)
	next := env.Do(http.MethodPost, "/api/v1/cart/quote", customerToken, map[string]any{
		"store_id": f.StoreA, "slot_id": slot2, "fulfilment_type": "pickup",
		"lines": []map[string]any{
			{"menu_item_id": f.ItemMain, "qty": 1, "option_choice_ids": []uuid.UUID{f.RiceWhite}},
		},
	})
	if next.Status != http.StatusOK {
		t.Fatalf("quote after parameter change: %d %s", next.Status, next.Raw)
	}
	if int64(next.Body["tax_bps"].(float64)) != 1100 {
		t.Fatalf("tax_bps %v, want the new 1100 with no restart", next.Body["tax_bps"])
	}
	// 50.000 at 11% = 5.500 exactly.
	if int64(next.Body["tax"].(float64)) != 5500 {
		t.Fatalf("tax %v, want 5500", next.Body["tax"])
	}

	// ── 10. The historical order keeps the rate it was priced at ─────────────
	unchanged := env.Do(http.MethodGet, "/api/v1/orders/"+orderID, customerToken, nil)
	if int64(unchanged.Body["tax"].(float64)) != tax {
		t.Fatalf("a past order's tax changed with the parameter (%v ≠ %d) — snapshots must hold (BR-2.5.1)",
			unchanged.Body["tax"], tax)
	}
}

func createCategory(t *testing.T, env *testenv.Env, token, nameID, nameEN, slug, cuisine string) uuid.UUID {
	t.Helper()
	res := env.Do(http.MethodPost, "/api/v1/admin/categories", token, map[string]any{
		"name_id": nameID, "name_en": nameEN, "slug": slug, "cuisine": cuisine, "is_active": true,
	})
	if res.Status != http.StatusCreated {
		t.Fatalf("create category %s: %d %s", slug, res.Status, res.Raw)
	}
	return uuid.MustParse(res.Body["id"].(string))
}

func createItem(t *testing.T, env *testenv.Env, token string, category uuid.UUID,
	sku, nameID, nameEN string, price int64) uuid.UUID {
	t.Helper()
	res := env.Do(http.MethodPost, "/api/v1/admin/menu-items", token, map[string]any{
		"category_id": category, "sku": sku, "name_id": nameID, "name_en": nameEN,
		"base_price": price, "kitchen_units": 1, "prep_minutes": 10,
		"is_halal": true, "is_active": true,
	})
	if res.Status != http.StatusCreated {
		t.Fatalf("create item %s: %d %s", sku, res.Status, res.Raw)
	}
	return uuid.MustParse(res.Body["id"].(string))
}

// formatRupiahLike renders an amount the way the ID template does, so the test
// asserts on what the customer actually reads.
func formatRupiahLike(v int64) string {
	digits := fmt.Sprintf("%d", v)
	var out []byte
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, digits[i])
	}
	return string(out)
}
