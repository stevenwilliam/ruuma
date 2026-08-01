//go:build security

package security_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/platform/security"
	"github.com/stevenwilliam/ruuma/test/testenv"
)

// TestCrossStoreAccessRefused_BR_2_7_8 is the tenancy test: staff of one store
// must not read, board, verify or report on another store — one case per role,
// per resource (docs/12, A01).
func TestCrossStoreAccessRefused_BR_2_7_8(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	// An order at store B, which store A's staff must never reach.
	slotB := env.MakeSlot(f.StoreB, "pickup", 12, 0, 5, 100)
	customer := env.MakeCustomers(1)[0]
	created := env.Idempotent(http.MethodPost, "/api/v1/orders",
		env.CustomerToken(customer), env.OrderBody(f.StoreB, slotB))
	if created.Status != http.StatusCreated {
		t.Fatalf("setup order at store B: %d %s", created.Status, created.Raw)
	}
	orderB := created.Body["id"].(string)

	storeAStaff := []struct {
		name string
		id   uuid.UUID
		role security.Role
	}{
		{"kitchen", f.KitchenA, security.RoleKitchen},
		{"counter", f.CounterA, security.RoleCounter},
		{"finance", f.FinanceA, security.RoleFinance},
		{"store_manager", f.ManagerA, security.RoleStoreManager},
	}

	for _, staff := range storeAStaff {
		token := env.StaffToken(staff.id, staff.role)

		t.Run(staff.name+"/board", func(t *testing.T) {
			res := env.Do(http.MethodGet,
				"/api/v1/ops/orders?store_id="+f.StoreB.String(), token, nil)
			if res.Status != http.StatusForbidden {
				t.Fatalf("board of another store: %d %s, want 403", res.Status, res.Raw)
			}
			if res.Code() != "STORE_OUT_OF_SCOPE" {
				t.Fatalf("code %q, want STORE_OUT_OF_SCOPE", res.Code())
			}
		})

		t.Run(staff.name+"/order", func(t *testing.T) {
			// Reading an out-of-scope order is a 404: the repository filter
			// means it does not exist for this caller (BR-2.7.8).
			res := env.Do(http.MethodGet, "/api/v1/ops/orders/"+orderB+"/ticket", token, nil)
			if res.Status != http.StatusNotFound && res.Status != http.StatusForbidden {
				t.Fatalf("ticket of another store's order: %d %s, want 403/404", res.Status, res.Raw)
			}
		})

		t.Run(staff.name+"/unpaid", func(t *testing.T) {
			res := env.Do(http.MethodGet,
				"/api/v1/ops/orders/unpaid?store_id="+f.StoreB.String(), token, nil)
			if res.Status != http.StatusForbidden {
				t.Fatalf("unpaid list of another store: %d, want 403", res.Status)
			}
		})
	}

	// The board scoped to their own store must still work — a scope that denies
	// everything is not a passing test.
	res := env.Do(http.MethodGet, "/api/v1/ops/orders?store_id="+f.StoreA.String(),
		env.StaffToken(f.KitchenA, security.RoleKitchen), nil)
	if res.Status != http.StatusOK {
		t.Fatalf("own store board: %d %s, want 200", res.Status, res.Raw)
	}
}

// TestUnscopedStaffSeeEverything_BR_2_7_7 is the other half: admin, owner and
// group-scoped finance are deliberately unscoped.
func TestUnscopedStaffSeeEverything_BR_2_7_7(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	for _, c := range []struct {
		name string
		id   uuid.UUID
		role security.Role
	}{
		{"admin", f.Admin, security.RoleAdmin},
		{"owner", f.Owner, security.RoleOwner},
		{"group finance", f.FinanceGroup, security.RoleFinance},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, store := range []uuid.UUID{f.StoreA, f.StoreB} {
				res := env.Do(http.MethodGet,
					"/api/v1/ops/orders?store_id="+store.String(),
					env.StaffToken(c.id, c.role), nil)
				if res.Status != http.StatusOK {
					t.Fatalf("%s at %s: %d %s", c.name, store, res.Status, res.Raw)
				}
			}
		})
	}
}

// TestRolePermissionMatrix_BR_2_7_6 walks the denials in docs/02 §3: the point
// of a permissions matrix is the cells that say no.
func TestRolePermissionMatrix_BR_2_7_6(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures

	type call struct {
		method, path string
		body         any
	}

	cases := []struct {
		name    string
		id      uuid.UUID
		role    security.Role
		allowed []call
		denied  []call
	}{
		{
			name: "kitchen", id: f.KitchenA, role: security.RoleKitchen,
			allowed: []call{{http.MethodGet, "/api/v1/ops/orders", nil}},
			denied: []call{
				{http.MethodGet, "/api/v1/finance/payments", nil},
				{http.MethodGet, "/api/v1/admin/sys-parameters", nil},
				{http.MethodGet, "/api/v1/admin/users", nil},
			},
		},
		{
			name: "counter", id: f.CounterA, role: security.RoleCounter,
			allowed: []call{{http.MethodGet, "/api/v1/ops/orders", nil}},
			denied: []call{
				{http.MethodGet, "/api/v1/finance/payments", nil},
				{http.MethodGet, "/api/v1/admin/sys-parameters", nil},
			},
		},
		{
			name: "finance", id: f.FinanceA, role: security.RoleFinance,
			allowed: []call{{http.MethodGet, "/api/v1/finance/payments?store_id=" + f.StoreA.String(), nil}},
			denied: []call{
				{http.MethodGet, "/api/v1/admin/users", nil},
				{http.MethodGet, "/api/v1/admin/sys-parameters", nil},
			},
		},
		{
			name: "store_manager", id: f.ManagerA, role: security.RoleStoreManager,
			allowed: []call{{http.MethodGet, "/api/v1/admin/stores/" + f.StoreA.String() + "/hours", nil}},
			denied: []call{
				{http.MethodGet, "/api/v1/admin/users", nil},
				{http.MethodGet, "/api/v1/admin/sys-parameters", nil},
				{http.MethodPost, "/api/v1/admin/menu-items", map[string]any{}},
			},
		},
		{
			name: "customer", id: f.Customer, role: security.RoleCustomer,
			allowed: []call{{http.MethodGet, "/api/v1/orders", nil}},
			denied: []call{
				{http.MethodGet, "/api/v1/ops/orders", nil},
				{http.MethodGet, "/api/v1/finance/payments", nil},
				{http.MethodGet, "/api/v1/admin/stores", nil},
				{http.MethodGet, "/api/v1/admin/audit-log", nil},
			},
		},
	}

	for _, tc := range cases {
		subject := security.SubjectStaff
		if tc.role == security.RoleCustomer {
			subject = security.SubjectCustomer
		}
		token := env.Token(subject, tc.id, tc.role)

		for _, c := range tc.denied {
			t.Run(tc.name+" denied "+c.path, func(t *testing.T) {
				res := env.Do(c.method, c.path, token, c.body)
				if res.Status != http.StatusForbidden {
					t.Fatalf("%s %s as %s: %d %s, want 403",
						c.method, c.path, tc.name, res.Status, res.Raw)
				}
			})
		}
		for _, c := range tc.allowed {
			t.Run(tc.name+" allowed "+c.path, func(t *testing.T) {
				res := env.Do(c.method, c.path, token, c.body)
				if res.Status == http.StatusForbidden || res.Status == http.StatusUnauthorized {
					t.Fatalf("%s %s as %s: %d %s, want access",
						c.method, c.path, tc.name, res.Status, res.Raw)
				}
			})
		}
	}
}

// TestUnauthenticatedIsDenied_BR_2_7_6 proves deny-by-default: no token, no
// access, on every protected surface.
func TestUnauthenticatedIsDenied_BR_2_7_6(t *testing.T) {
	env := testenv.New(t)

	protected := []string{
		"/api/v1/me",
		"/api/v1/orders",
		"/api/v1/ops/orders",
		"/api/v1/finance/payments",
		"/api/v1/admin/stores",
		"/api/v1/admin/users",
		"/api/v1/admin/sys-parameters",
		"/api/v1/admin/audit-log",
	}
	for _, path := range protected {
		res := env.Do(http.MethodGet, path, "", nil)
		if res.Status != http.StatusUnauthorized {
			t.Errorf("%s without a token: %d, want 401", path, res.Status)
		}
	}

	// Public reads stay public — a store picker behind a login is not the
	// product we agreed (docs/01 §3.1).
	for _, path := range []string{"/api/v1/stores", "/health"} {
		res := env.Do(http.MethodGet, path, "", nil)
		if res.Status != http.StatusOK {
			t.Errorf("%s: %d, want 200", path, res.Status)
		}
	}
}
