package security

import (
	"testing"

	"github.com/google/uuid"
)

// matrix restates docs/02 §3 independently of the implementation. Each entry is
// a permission and the roles that hold it; every other role must be denied.
var matrix = map[Permission][]Role{
	PermOrderCreate:         {RoleCustomer},
	PermOrderViewOwn:        {RoleCustomer},
	PermPaymentProofUpload:  {RoleCustomer},
	PermOrderViewStore:      {RoleKitchen, RoleCounter, RoleFinance, RoleStoreManager, RoleAdmin, RoleOwner},
	PermOrderStatusKitchen:  {RoleKitchen, RoleStoreManager, RoleAdmin, RoleOwner},
	PermOrderStatusHandover: {RoleCounter, RoleStoreManager, RoleAdmin, RoleOwner},
	PermPaymentQueueView:    {RoleFinance, RoleStoreManager, RoleAdmin, RoleOwner},
	PermPaymentVerify:       {RoleFinance, RoleAdmin, RoleOwner},
	PermPaymentRefund:       {RoleFinance, RoleAdmin, RoleOwner},
	PermStoreScheduleManage: {RoleStoreManager, RoleAdmin, RoleOwner},
	PermStoreManage:         {RoleAdmin, RoleOwner},
	PermMenuManage:          {RoleAdmin, RoleOwner},
	PermMenuPriceOverride:   {RoleAdmin, RoleOwner},
	PermMenu86:              {RoleKitchen, RoleStoreManager, RoleAdmin, RoleOwner},
	PermPromotionManage:     {RoleAdmin, RoleOwner},
	PermStaffManage:         {RoleAdmin, RoleOwner},
	PermParametersManage:    {RoleAdmin, RoleOwner},
	PermReportsGroup:        {RoleAdmin, RoleOwner},
	PermAuditView:           {RoleStoreManager, RoleAdmin, RoleOwner},
}

// BR-2.7.6: authorization is deny-by-default, and every cell of the matrix is
// asserted — the denials are the point.
func TestPermissionsMatrix_BR_2_7_6(t *testing.T) {
	for perm, holders := range matrix {
		held := map[Role]bool{}
		for _, r := range holders {
			held[r] = true
		}
		for _, role := range AllRoles() {
			p := Principal{Role: role}
			got := p.Can(perm)
			if got != held[role] {
				t.Errorf("role %s / permission %s: got %v, want %v", role, perm, got, held[role])
			}
		}
	}
}

// A principal with no role holds nothing — an unauthenticated or malformed
// token can never fall through to a permission.
func TestEmptyPrincipalHoldsNothing_BR_2_7_6(t *testing.T) {
	var p Principal
	for _, perm := range AllPermissions() {
		if p.Can(perm) {
			t.Fatalf("a role-less principal must not hold %s", perm)
		}
	}
	if p.CanAccessStore(uuid.New()) {
		t.Fatal("a role-less principal must not access any store")
	}
}

// A customer never holds a staff permission, whatever else changes.
func TestCustomerHoldsNoStaffPermission_BR_2_7_6(t *testing.T) {
	staffOnly := []Permission{
		PermOrderViewStore, PermPaymentVerify, PermPaymentQueueView, PermKitchenBoard,
		PermStoreManage, PermMenuManage, PermParametersManage, PermReportsGroup, PermAuditView,
	}
	p := Principal{Role: RoleCustomer}
	for _, perm := range staffOnly {
		if p.Can(perm) {
			t.Fatalf("customer must not hold %s", perm)
		}
	}
}

// BR-2.7.7/8/9: store scope is a tenancy boundary; admin, owner and group-scoped
// finance see everything, everyone else sees only their assignments.
func TestStoreScope_BR_2_7_7_BR_2_7_8(t *testing.T) {
	storeA, storeB := uuid.New(), uuid.New()

	kitchen := Principal{Role: RoleKitchen, Stores: []uuid.UUID{storeA}}
	if !kitchen.CanAccessStore(storeA) {
		t.Fatal("kitchen must reach its own store")
	}
	if kitchen.CanAccessStore(storeB) {
		t.Fatal("kitchen must not reach another store")
	}
	if scope := kitchen.StoreScope(); len(scope) != 1 || scope[0] != storeA {
		t.Fatalf("scope %v, want exactly [storeA]", scope)
	}

	// A scoped finance user is bounded; a group-scoped one is not (BR-2.7.7).
	scopedFinance := Principal{Role: RoleFinance, Stores: []uuid.UUID{storeA}}
	if scopedFinance.CanAccessStore(storeB) {
		t.Fatal("store-scoped finance must not reach another store")
	}
	groupFinance := Principal{Role: RoleFinance, GroupScope: true}
	if !groupFinance.CanAccessStore(storeB) || groupFinance.StoreScope() != nil {
		t.Fatal("group-scoped finance sees every store")
	}

	for _, role := range []Role{RoleAdmin, RoleOwner} {
		p := Principal{Role: role}
		if !p.CanAccessStore(storeB) || p.StoreScope() != nil {
			t.Fatalf("%s is unscoped by definition", role)
		}
	}

	// StoreScope must hand back a copy: a repository must not be able to mutate
	// the principal's assignments.
	scope := kitchen.StoreScope()
	scope[0] = storeB
	if kitchen.Stores[0] != storeA {
		t.Fatal("StoreScope must return a copy, not the backing array")
	}
}

// Owner is admin plus unscoped reach; the sets must not drift apart silently.
func TestOwnerCoversAdmin(t *testing.T) {
	owner := Principal{Role: RoleOwner}
	for _, perm := range PermissionsFor(RoleAdmin) {
		if !owner.Can(perm) {
			t.Fatalf("owner is missing admin permission %s", perm)
		}
	}
}
