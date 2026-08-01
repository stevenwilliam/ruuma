package security

import (
	"github.com/google/uuid"
)

// Role is a staff or customer role (BR-2.7.6).
type Role string

const (
	RoleCustomer     Role = "customer"
	RoleKitchen      Role = "kitchen"
	RoleCounter      Role = "counter"
	RoleFinance      Role = "finance"
	RoleStoreManager Role = "store_manager"
	RoleAdmin        Role = "admin"
	RoleOwner        Role = "owner"
)

// Permission is a single capability a handler may require. Authorization is
// deny-by-default: a route that declares no permission serves nobody
// (BR-2.7.6, docs/12 A01).
type Permission string

const (
	// Customer
	PermOrderCreate        Permission = "order.create"
	PermOrderViewOwn       Permission = "order.view.own"
	PermOrderCancelOwn     Permission = "order.cancel.own"
	PermPaymentProofUpload Permission = "payment.proof.upload"
	PermProfileManage      Permission = "profile.manage"

	// Store-scoped operations
	PermOrderViewStore      Permission = "order.view.store"
	PermOrderCancelStaff    Permission = "order.cancel.staff"
	PermOrderStatusKitchen  Permission = "order.status.kitchen"  // IN_KITCHEN, READY
	PermOrderStatusHandover Permission = "order.status.handover" // PICKED_UP
	PermKitchenBoard        Permission = "kitchen.board"
	PermTicketPrint         Permission = "kitchen.ticket.print"

	// Finance
	PermPaymentQueueView Permission = "payment.queue.view"
	PermPaymentVerify    Permission = "payment.verify"
	PermPaymentRefund    Permission = "payment.refund"
	PermReconciliation   Permission = "finance.reconciliation"

	// Store master data
	PermStoreScheduleManage Permission = "store.schedule.manage"
	PermStoreCapacityManage Permission = "store.capacity.manage"
	PermStoreManage         Permission = "store.manage"
	PermStoreBankRead       Permission = "store.bank.read"
	PermStoreBankManage     Permission = "store.bank.manage"

	// Menu
	PermMenuManage        Permission = "menu.manage"
	PermMenuAvailability  Permission = "menu.availability.manage"
	PermMenu86            Permission = "menu.86"
	PermMenuPriceOverride Permission = "menu.price.override"

	// Group administration
	PermPromotionManage  Permission = "promotion.manage"
	PermStaffManage      Permission = "staff.manage"
	PermParametersManage Permission = "parameters.manage"
	PermReportsStore     Permission = "reports.store"
	PermReportsGroup     Permission = "reports.group"
	PermAuditView        Permission = "audit.view"
)

// rolePermissions is the permissions matrix from docs/02 §3, in code. It is the
// single source of truth for authorization; handlers declare a permission and
// never test a role directly.
var rolePermissions = map[Role]map[Permission]bool{
	RoleCustomer: set(
		PermOrderCreate, PermOrderViewOwn, PermOrderCancelOwn,
		PermPaymentProofUpload, PermProfileManage,
	),
	RoleKitchen: set(
		PermOrderViewStore, PermKitchenBoard, PermTicketPrint,
		PermOrderStatusKitchen, PermMenu86,
	),
	RoleCounter: set(
		PermOrderViewStore, PermKitchenBoard, PermTicketPrint,
		PermOrderStatusHandover, PermOrderCancelStaff,
	),
	RoleFinance: set(
		PermOrderViewStore, PermPaymentQueueView, PermPaymentVerify,
		PermPaymentRefund, PermReconciliation, PermReportsStore, PermStoreBankRead,
	),
	RoleStoreManager: set(
		PermOrderViewStore, PermOrderCancelStaff, PermKitchenBoard, PermTicketPrint,
		PermOrderStatusKitchen, PermOrderStatusHandover,
		PermStoreScheduleManage, PermStoreCapacityManage,
		PermMenuAvailability, PermMenu86,
		PermPaymentQueueView, PermReconciliation, PermReportsStore, PermAuditView,
	),
	RoleAdmin: set(
		PermOrderViewStore, PermOrderCancelStaff, PermKitchenBoard, PermTicketPrint,
		PermOrderStatusKitchen, PermOrderStatusHandover,
		PermPaymentQueueView, PermPaymentVerify, PermPaymentRefund, PermReconciliation,
		PermStoreScheduleManage, PermStoreCapacityManage, PermStoreManage,
		PermStoreBankRead, PermStoreBankManage,
		PermMenuManage, PermMenuAvailability, PermMenu86, PermMenuPriceOverride,
		PermPromotionManage, PermStaffManage, PermParametersManage,
		PermReportsStore, PermReportsGroup, PermAuditView,
	),
	// Owner is admin plus owner-level parameters and store deactivation; both
	// are covered by the same permissions, so the sets are equal by construction
	// and RoleOwner is unscoped (see Principal.CanAccessStore).
	RoleOwner: nil, // filled in init from RoleAdmin
}

func init() {
	owner := make(map[Permission]bool, len(rolePermissions[RoleAdmin]))
	for p := range rolePermissions[RoleAdmin] {
		owner[p] = true
	}
	rolePermissions[RoleOwner] = owner
}

func set(perms ...Permission) map[Permission]bool {
	m := make(map[Permission]bool, len(perms))
	for _, p := range perms {
		m[p] = true
	}
	return m
}

// SubjectType distinguishes a customer account from a staff account.
type SubjectType string

const (
	SubjectCustomer SubjectType = "customer"
	SubjectStaff    SubjectType = "staff"
)

// Principal is the authenticated caller. Stores is the set of stores a staff
// member is assigned to (BR-2.7.7); it is resolved server-side from the
// database and never from a client claim (BR-2.7.9).
type Principal struct {
	SubjectType SubjectType
	ID          uuid.UUID
	Role        Role
	Stores      []uuid.UUID
	GroupScope  bool // finance may be scoped to the whole group (BR-2.7.7)
	TokenID     uuid.UUID
}

// Can reports whether the principal holds a permission.
func (p Principal) Can(perm Permission) bool {
	if p.Role == "" {
		return false // deny by default
	}
	return rolePermissions[p.Role][perm]
}

// IsUnscoped reports whether the principal sees every store: admin, owner, or a
// group-scoped finance user.
func (p Principal) IsUnscoped() bool {
	return p.Role == RoleAdmin || p.Role == RoleOwner || p.GroupScope
}

// CanAccessStore reports whether the principal may act on a store. This is the
// tenancy check behind every store-scoped request (BR-2.7.8/9).
func (p Principal) CanAccessStore(storeID uuid.UUID) bool {
	if p.IsUnscoped() {
		return true
	}
	for _, s := range p.Stores {
		if s == storeID {
			return true
		}
	}
	return false
}

// StoreScope returns the store filter for the repository layer: nil means
// "every store" (admin, owner, group-scoped finance), otherwise the query must
// be restricted to exactly these ids. Repositories take this value, so scope
// cannot be forgotten in a handler (BR-2.7.8).
func (p Principal) StoreScope() []uuid.UUID {
	if p.IsUnscoped() {
		return nil
	}
	out := make([]uuid.UUID, len(p.Stores))
	copy(out, p.Stores)
	return out
}

// PermissionsFor exposes a role's permission set (used by the admin UI and by
// the authorization tests).
func PermissionsFor(r Role) []Permission {
	out := make([]Permission, 0, len(rolePermissions[r]))
	for p := range rolePermissions[r] {
		out = append(out, p)
	}
	return out
}

// AllRoles lists every role, for tests that must walk the whole matrix.
func AllRoles() []Role {
	return []Role{RoleCustomer, RoleKitchen, RoleCounter, RoleFinance, RoleStoreManager, RoleAdmin, RoleOwner}
}

// AllPermissions lists every permission, for the deny-by-default routing test.
func AllPermissions() []Permission {
	seen := map[Permission]bool{}
	var out []Permission
	for _, perms := range rolePermissions {
		for p := range perms {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}
