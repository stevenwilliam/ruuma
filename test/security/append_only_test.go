//go:build security

package security_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stevenwilliam/ruuma/internal/platform/security"
	"github.com/stevenwilliam/ruuma/test/testenv"
)

// TestEventTablesAreAppendOnly_BR_2_10_2 attacks the tables directly. These
// three are financial and legal evidence: a correction is a new row, never an
// edit, and the database enforces that regardless of role.
func TestEventTablesAreAppendOnly_BR_2_10_2(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	// Produce one row in each table by doing real work.
	slot := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)
	created := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(f.Customer), env.OrderBody(f.StoreA, slot))
	if created.Status != http.StatusCreated {
		t.Fatalf("setup: %d %s", created.Status, created.Raw)
	}
	orderID := created.Body["id"].(string)
	env.AttachProof(orderID, f.Customer, int64(created.Body["amount_due"].(float64)))
	paymentID := env.PaymentIDForOrder(orderID)

	verify := env.Idempotent(http.MethodPost, "/api/v1/finance/payments/"+paymentID+"/verify",
		env.StaffToken(f.FinanceA, security.RoleFinance), map[string]any{})
	if verify.Status != http.StatusOK {
		t.Fatalf("verify: %d %s", verify.Status, verify.Raw)
	}

	for _, table := range []string{"order_events", "payment_events", "audit_log"} {
		var count int64
		if err := env.DB.Raw("SELECT count(*) FROM " + table).Scan(&count).Error; err != nil {
			t.Fatalf("%s count: %v", table, err)
		}
		if count == 0 {
			t.Fatalf("%s has no rows — the test would prove nothing", table)
		}

		t.Run(table+"/update", func(t *testing.T) {
			err := env.DB.Exec("UPDATE " + table + " SET created_at = now()").Error
			if err == nil {
				t.Fatalf("UPDATE on %s succeeded; it must be refused (BR-2.10.2)", table)
			}
			if !strings.Contains(err.Error(), "append-only") {
				t.Fatalf("%s refused with %v, want the append-only guard", table, err)
			}
		})

		t.Run(table+"/delete", func(t *testing.T) {
			if err := env.DB.Exec("DELETE FROM " + table).Error; err == nil {
				t.Fatalf("DELETE on %s succeeded; it must be refused (BR-2.10.2)", table)
			}
		})

		var after int64
		if err := env.DB.Raw("SELECT count(*) FROM " + table).Scan(&after).Error; err != nil {
			t.Fatalf("%s recount: %v", table, err)
		}
		if after != count {
			t.Fatalf("%s lost rows: %d → %d", table, count, after)
		}
	}
}

// TestPrivilegedActionsAreAudited_BR_2_10_1 — every money and configuration
// action leaves a trail with an actor.
func TestPrivilegedActionsAreAudited_BR_2_10_1(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	admin := env.StaffToken(f.Admin, security.RoleAdmin)

	res := env.Do(http.MethodPut, "/api/v1/admin/sys-parameters", admin,
		map[string]any{"key": "scheduling.lead_time_minutes", "value": "45"})
	if res.Status != http.StatusOK {
		t.Fatalf("parameter change: %d %s", res.Status, res.Raw)
	}

	var actions []string
	if err := env.DB.Raw(
		`SELECT action FROM audit_log WHERE actor_id = ? ORDER BY created_at DESC`, f.Admin).
		Scan(&actions).Error; err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("a parameter change left no audit trail (BR-2.10.1)")
	}

	found := false
	for _, a := range actions {
		if a == "parameter.set" {
			found = true
		}
	}
	if !found {
		t.Fatalf("audit actions %v do not include parameter.set", actions)
	}
}

// TestParameterChangeAppliesWithoutRestart_BR_2_9_2 proves the configuration
// promise: an operator retunes capacity and the next order feels it.
func TestParameterChangeAppliesWithoutRestart_BR_2_9_2(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	admin := env.StaffToken(f.Admin, security.RoleAdmin)

	before := env.Do(http.MethodGet, "/api/v1/admin/sys-parameters?q=lead_time", admin, nil)
	if before.Status != http.StatusOK {
		t.Fatalf("read parameters: %d %s", before.Status, before.Raw)
	}

	set := env.Do(http.MethodPut, "/api/v1/admin/sys-parameters", admin,
		map[string]any{"key": "orders.max_unpaid_per_customer", "value": "1"})
	if set.Status != http.StatusOK {
		t.Fatalf("set: %d %s", set.Status, set.Raw)
	}

	// The new cap must bite immediately — no restart (BR-2.9.2).
	token := env.CustomerToken(f.Customer)
	slotA := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)
	if first := env.Idempotent(http.MethodPost, "/api/v1/orders", token,
		env.OrderBody(f.StoreA, slotA)); first.Status != http.StatusCreated {
		t.Fatalf("first order: %d %s", first.Status, first.Raw)
	}

	slotB := env.MakeSlot(f.StoreA, "pickup", 13, 0, 5, 100)
	second := env.Idempotent(http.MethodPost, "/api/v1/orders", token, env.OrderBody(f.StoreA, slotB))
	if second.Code() != "UNPAID_LIMIT_REACHED" {
		t.Fatalf("cap of 1 not applied without restart: %d %s", second.Status, second.Raw)
	}
}
