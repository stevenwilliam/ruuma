// Package adminsvc holds the administrative use-cases: store master data,
// schedules, blackouts, menu, availability, promotions, staff and parameters.
//
// Every mutation is store-scope-checked and audited (BR-2.7.8, BR-2.10.1).
package adminsvc

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// StoreWrite is the master-data writer this service needs.
type StoreWrite interface {
	Create(ctx context.Context, in ports.StoreView) (*ports.StoreView, error)
	Update(ctx context.Context, in ports.StoreView) error
	SetActive(ctx context.Context, id uuid.UUID, active bool) error
	ReplaceModes(ctx context.Context, storeID uuid.UUID, modes []string) error
	Hours(ctx context.Context, storeID uuid.UUID) ([]HoursRow, error)
	ReplaceHours(ctx context.Context, storeID uuid.UUID, rows []HoursRow) error
	DateOverrides(ctx context.Context, storeID uuid.UUID, from, to time.Time) ([]OverrideRow, error)
	UpsertDateOverride(ctx context.Context, row OverrideRow, actor uuid.UUID) error
	DeleteDateOverride(ctx context.Context, storeID, id uuid.UUID) error
	Blackouts(ctx context.Context, storeID uuid.UUID, from, to time.Time) ([]BlackoutRow, error)
	AddBlackout(ctx context.Context, row BlackoutRow, actor uuid.UUID) error
	RemoveBlackout(ctx context.Context, storeID uuid.UUID, date time.Time) error
	BankAccounts(ctx context.Context, storeID uuid.UUID) ([]ports.BankAccountView, error)
	SaveBankAccount(ctx context.Context, storeID uuid.UUID, in BankAccountInput) error
	SetPrimaryBankAccount(ctx context.Context, storeID, accountID uuid.UUID) error
}

// HoursRow is one weekday × mode × block of a store's schedule (BR-2.1.4/5).
type HoursRow struct {
	ID         uuid.UUID
	Weekday    int
	Mode       string
	BlockIndex int
	IsClosed   bool
	OpensAt    string
	ClosesAt   string
}

// OverrideRow is a per-date schedule override (BR-2.1.6, D18).
type OverrideRow struct {
	ID         uuid.UUID
	StoreID    uuid.UUID
	Date       time.Time
	Mode       string
	BlockIndex int
	IsClosed   bool
	OpensAt    string
	ClosesAt   string
	Reason     string
}

// BlackoutRow closes a store for a date; today is allowed (BR-2.1.7, D27).
type BlackoutRow struct {
	ID      uuid.UUID
	StoreID uuid.UUID
	Date    time.Time
	Reason  string
}

// BankAccountInput creates or updates a transfer destination.
type BankAccountInput struct {
	ID            uuid.UUID
	BankName      string
	AccountName   string
	AccountNumber string
	IsPrimary     bool
	IsActive      bool
}

// MenuWrite is the menu writer.
type MenuWrite interface {
	SaveCategory(ctx context.Context, in CategoryInput) (uuid.UUID, error)
	SaveItem(ctx context.Context, in ItemInput) (uuid.UUID, error)
	SetStoreOverride(ctx context.Context, storeID, itemID uuid.UUID, available *bool, price *int64, actor uuid.UUID) error
	Add86(ctx context.Context, storeID, itemID uuid.UUID, until *time.Time, reason string, actor uuid.UUID) error
	Lift86(ctx context.Context, storeID, itemID uuid.UUID) error
	SetDailyStock(ctx context.Context, storeID, itemID uuid.UUID, date time.Time, total int) error
	SetItemPhoto(ctx context.Context, itemID uuid.UUID, objectKey string) error
}

// CategoryInput creates or updates a category.
type CategoryInput struct {
	ID        uuid.UUID
	NameID    string
	NameEN    string
	Slug      string
	Cuisine   string
	SortOrder int
	IsActive  bool
}

// ItemInput creates or updates a menu item.
type ItemInput struct {
	ID              uuid.UUID
	CategoryID      uuid.UUID
	SKU             string
	NameID          string
	NameEN          string
	DescriptionID   string
	DescriptionEN   string
	BasePrice       int64
	KitchenUnits    int
	PrepMinutes     int
	MinLeadMinutes  int
	SpiceLevel      int
	IsHalal         bool
	IsVegetarian    bool
	ContainsPork    bool
	ContainsAlcohol bool
	ContainsNuts    bool
	IsActive        bool
	SortOrder       int
}

// ParamWrite manages configuration (BR-1.4.2).
type ParamWrite interface {
	ListGroup(ctx context.Context, q string) ([]ParamRow, error)
	ListStore(ctx context.Context, storeID uuid.UUID) ([]ParamRow, error)
	UpsertGroup(ctx context.Context, key, value string, actor uuid.UUID) error
	UpsertStore(ctx context.Context, storeID uuid.UUID, key, value string, actor uuid.UUID) error
	DeleteGroup(ctx context.Context, key string) error
	DeleteStore(ctx context.Context, storeID uuid.UUID, key string) error
}

// ParamRow is one configuration row; secrets arrive already masked (BR-1.4.3).
type ParamRow struct {
	Key                string
	Value              string
	DataType           string
	Description        string
	IsSecret           bool
	IsStoreOverridable bool
	Source             string
}

// AuditRead backs the audit viewer.
type AuditRead interface {
	List(ctx context.Context, q, entityType string, storeID *uuid.UUID, from, to *time.Time, limit int, scope []uuid.UUID) ([]AuditRow, error)
}

// AuditRow is one audit entry.
type AuditRow struct {
	ID         uuid.UUID
	ActorEmail string
	ActorType  string
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	StoreID    *uuid.UUID
	CreatedAt  time.Time
}

type Service struct {
	stores    ports.Stores
	storeW    StoreWrite
	menuW     MenuWrite
	params    ParamWrite
	staff     ports.Staff
	auditRead AuditRead
	audit     ports.Auditor
	storage   ports.Storage
	clock     ports.Clock
}

func New(stores ports.Stores, storeW StoreWrite, menuW MenuWrite, params ParamWrite,
	staff ports.Staff, auditRead AuditRead, audit ports.Auditor, storage ports.Storage,
	clk ports.Clock) *Service {
	return &Service{
		stores: stores, storeW: storeW, menuW: menuW, params: params, staff: staff,
		auditRead: auditRead, audit: audit, storage: storage, clock: clk,
	}
}

func (s *Service) requireStore(p security.Principal, storeID uuid.UUID) error {
	if !p.CanAccessStore(storeID) {
		return apierror.Forbidden(apierror.CodeStoreOutOfScope, "That store is outside your access.")
	}
	return nil
}

// ── Stores ───────────────────────────────────────────────────────────────────

func (s *Service) Stores(ctx context.Context, p security.Principal, q string) ([]ports.StoreView, error) {
	return s.stores.ListAll(ctx, q, p.StoreScope())
}

func (s *Service) CreateStore(ctx context.Context, p security.Principal, in ports.StoreView) (*ports.StoreView, error) {
	created, err := s.storeW.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.create",
		EntityType: "store", EntityID: &created.ID, StoreID: &created.ID, After: in,
	})
	return created, nil
}

func (s *Service) UpdateStore(ctx context.Context, p security.Principal, in ports.StoreView) error {
	if err := s.requireStore(p, in.ID); err != nil {
		return err
	}
	before, err := s.stores.Get(ctx, in.ID)
	if err != nil {
		return err
	}
	if err := s.storeW.Update(ctx, in); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.update",
		EntityType: "store", EntityID: &in.ID, StoreID: &in.ID, Before: before, After: in,
	})
}

func (s *Service) SetStoreActive(ctx context.Context, p security.Principal, storeID uuid.UUID, active bool) error {
	if err := s.storeW.SetActive(ctx, storeID, active); err != nil {
		return err
	}
	action := "store.deactivate"
	if active {
		action = "store.activate"
	}
	// Deactivation hides the store but never touches its history (BR-2.1.11).
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: action,
		EntityType: "store", EntityID: &storeID, StoreID: &storeID,
	})
}

func (s *Service) SetModes(ctx context.Context, p security.Principal, storeID uuid.UUID, modes []string) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.storeW.ReplaceModes(ctx, storeID, modes); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.modes.replace",
		EntityType: "store", EntityID: &storeID, StoreID: &storeID,
		After: map[string]any{"modes": modes},
	})
}

func (s *Service) Hours(ctx context.Context, p security.Principal, storeID uuid.UUID) ([]HoursRow, error) {
	if err := s.requireStore(p, storeID); err != nil {
		return nil, err
	}
	return s.storeW.Hours(ctx, storeID)
}

// ReplaceHours swaps a store's weekly pattern. Changes apply to slots
// materialised afterwards; booked slots keep their capacity (BR-2.3.16).
func (s *Service) ReplaceHours(ctx context.Context, p security.Principal, storeID uuid.UUID, rows []HoursRow) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	before, _ := s.storeW.Hours(ctx, storeID)
	if err := s.storeW.ReplaceHours(ctx, storeID, rows); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.hours.replace",
		EntityType: "store", EntityID: &storeID, StoreID: &storeID,
		Before: before, After: rows,
	})
}

func (s *Service) DateOverrides(ctx context.Context, p security.Principal, storeID uuid.UUID, from, to time.Time) ([]OverrideRow, error) {
	if err := s.requireStore(p, storeID); err != nil {
		return nil, err
	}
	return s.storeW.DateOverrides(ctx, storeID, from, to)
}

func (s *Service) SaveDateOverride(ctx context.Context, p security.Principal, row OverrideRow) error {
	if err := s.requireStore(p, row.StoreID); err != nil {
		return err
	}
	if err := s.storeW.UpsertDateOverride(ctx, row, p.ID); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.date_override.save",
		EntityType: "store", EntityID: &row.StoreID, StoreID: &row.StoreID, After: row,
	})
}

func (s *Service) DeleteDateOverride(ctx context.Context, p security.Principal, storeID, id uuid.UUID) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.storeW.DeleteDateOverride(ctx, storeID, id); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.date_override.delete",
		EntityType: "store", EntityID: &storeID, StoreID: &storeID,
	})
}

func (s *Service) Blackouts(ctx context.Context, p security.Principal, storeID uuid.UUID, from, to time.Time) ([]BlackoutRow, error) {
	if err := s.requireStore(p, storeID); err != nil {
		return nil, err
	}
	return s.storeW.Blackouts(ctx, storeID, from, to)
}

// BlackoutResult reports how many live orders a closure touches. They are never
// cancelled automatically (BR-2.1.9, D27).
type BlackoutResult struct {
	AffectedOrders int64
	Note           string
}

// AddBlackout closes a store for a date, today included.
func (s *Service) AddBlackout(ctx context.Context, p security.Principal, row BlackoutRow) (*BlackoutResult, error) {
	if err := s.requireStore(p, row.StoreID); err != nil {
		return nil, err
	}
	if row.Reason == "" {
		return nil, apierror.Validation("A closure needs a reason.",
			map[string]any{"reason": "required"})
	}
	if err := s.storeW.AddBlackout(ctx, row, p.ID); err != nil {
		return nil, err
	}

	affected, err := s.stores.CountAffectedOrders(ctx, row.StoreID, row.Date)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.blackout.create",
		EntityType: "store", EntityID: &row.StoreID, StoreID: &row.StoreID,
		After: map[string]any{
			"date": row.Date.Format("2006-01-02"), "reason": row.Reason,
			"affected_orders": affected,
		},
	})
	return &BlackoutResult{
		AffectedOrders: affected,
		Note:           "Existing orders are untouched. Review them under ops/orders/affected.",
	}, nil
}

func (s *Service) RemoveBlackout(ctx context.Context, p security.Principal, storeID uuid.UUID, date time.Time) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.storeW.RemoveBlackout(ctx, storeID, date); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.blackout.delete",
		EntityType: "store", EntityID: &storeID, StoreID: &storeID,
		After: map[string]any{"date": date.Format("2006-01-02")},
	})
}

func (s *Service) BankAccounts(ctx context.Context, p security.Principal, storeID uuid.UUID) ([]ports.BankAccountView, error) {
	if err := s.requireStore(p, storeID); err != nil {
		return nil, err
	}
	return s.storeW.BankAccounts(ctx, storeID)
}

func (s *Service) SaveBankAccount(ctx context.Context, p security.Principal, storeID uuid.UUID, in BankAccountInput) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.storeW.SaveBankAccount(ctx, storeID, in); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "store.bank_account.save",
		EntityType: "store", EntityID: &storeID, StoreID: &storeID,
		After: map[string]any{"bank": in.BankName, "primary": in.IsPrimary},
	})
}

// ── Menu ─────────────────────────────────────────────────────────────────────

func (s *Service) SaveCategory(ctx context.Context, p security.Principal, in CategoryInput) (uuid.UUID, error) {
	id, err := s.menuW.SaveCategory(ctx, in)
	if err != nil {
		return uuid.Nil, err
	}
	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "menu.category.save",
		EntityType: "category", EntityID: &id, After: in,
	})
	return id, nil
}

// SaveItem writes a menu item. A price change is audited before/after, and
// never touches historical orders (BR-2.5.1, BR-2.10.1).
func (s *Service) SaveItem(ctx context.Context, p security.Principal, in ItemInput) (uuid.UUID, error) {
	id, err := s.menuW.SaveItem(ctx, in)
	if err != nil {
		return uuid.Nil, err
	}
	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "menu.item.save",
		EntityType: "menu_item", EntityID: &id, After: in,
	})
	return id, nil
}

func (s *Service) SetItemPhoto(ctx context.Context, p security.Principal, itemID uuid.UUID, photo []byte) (string, error) {
	key, err := s.storage.PutPhoto(ctx, "menu", photo)
	if err != nil {
		return "", err
	}
	if err := s.menuW.SetItemPhoto(ctx, itemID, key); err != nil {
		return "", err
	}
	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "menu.item.photo",
		EntityType: "menu_item", EntityID: &itemID,
	})
	return key, nil
}

func (s *Service) SetStoreOverride(ctx context.Context, p security.Principal, storeID, itemID uuid.UUID, available *bool, price *int64) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	// Only admin and owner may change a price; a store manager may only change
	// availability (docs/02 §3).
	if price != nil && !p.Can(security.PermMenuPriceOverride) {
		return apierror.Forbidden(apierror.CodeForbidden, "You cannot change prices.")
	}
	if err := s.menuW.SetStoreOverride(ctx, storeID, itemID, available, price, p.ID); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "menu.store_override.set",
		EntityType: "menu_item", EntityID: &itemID, StoreID: &storeID,
		After: map[string]any{"is_available": available, "price_override": price},
	})
}

func (s *Service) Add86(ctx context.Context, p security.Principal, storeID, itemID uuid.UUID, until *time.Time, reason string) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.menuW.Add86(ctx, storeID, itemID, until, reason, p.ID); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "menu.86.add",
		EntityType: "menu_item", EntityID: &itemID, StoreID: &storeID,
		After: map[string]any{"until": until, "reason": reason},
	})
}

func (s *Service) Lift86(ctx context.Context, p security.Principal, storeID, itemID uuid.UUID) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.menuW.Lift86(ctx, storeID, itemID); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "menu.86.lift",
		EntityType: "menu_item", EntityID: &itemID, StoreID: &storeID,
	})
}

func (s *Service) SetDailyStock(ctx context.Context, p security.Principal, storeID, itemID uuid.UUID, date time.Time, total int) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.menuW.SetDailyStock(ctx, storeID, itemID, date, total); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "menu.daily_stock.set",
		EntityType: "menu_item", EntityID: &itemID, StoreID: &storeID,
		After: map[string]any{"date": date.Format("2006-01-02"), "total": total},
	})
}

// ── Parameters ───────────────────────────────────────────────────────────────

func (s *Service) Parameters(ctx context.Context, q string) ([]ParamRow, error) {
	return s.params.ListGroup(ctx, q)
}

func (s *Service) StoreParameters(ctx context.Context, p security.Principal, storeID uuid.UUID) ([]ParamRow, error) {
	if err := s.requireStore(p, storeID); err != nil {
		return nil, err
	}
	return s.params.ListStore(ctx, storeID)
}

// SetParameter changes configuration without a deploy, audited before/after
// (BR-2.9.2, BR-2.10.1).
func (s *Service) SetParameter(ctx context.Context, p security.Principal, key, value string) error {
	before, _ := s.params.ListGroup(ctx, key)
	if err := s.params.UpsertGroup(ctx, key, value, p.ID); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "parameter.set",
		EntityType: "sys_parameter", Before: before,
		After: map[string]any{"key": key, "value": value},
	})
}

func (s *Service) SetStoreParameter(ctx context.Context, p security.Principal, storeID uuid.UUID, key, value string) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.params.UpsertStore(ctx, storeID, key, value, p.ID); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "parameter.store.set",
		EntityType: "store_parameter", EntityID: &storeID, StoreID: &storeID,
		After: map[string]any{"key": key, "value": value},
	})
}

func (s *Service) DeleteParameter(ctx context.Context, p security.Principal, key string) error {
	if err := s.params.DeleteGroup(ctx, key); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "parameter.delete",
		EntityType: "sys_parameter", After: map[string]any{"key": key},
	})
}

func (s *Service) DeleteStoreParameter(ctx context.Context, p security.Principal, storeID uuid.UUID, key string) error {
	if err := s.requireStore(p, storeID); err != nil {
		return err
	}
	if err := s.params.DeleteStore(ctx, storeID, key); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "parameter.store.delete",
		EntityType: "store_parameter", EntityID: &storeID, StoreID: &storeID,
		After: map[string]any{"key": key},
	})
}

// ── Staff ────────────────────────────────────────────────────────────────────

func (s *Service) Staff(ctx context.Context, q string) ([]ports.StaffView, error) {
	return s.staff.List(ctx, q)
}

func (s *Service) CreateStaff(ctx context.Context, p security.Principal, in ports.StaffView, password string) (*ports.StaffView, error) {
	if err := security.ValidatePassword(password); err != nil {
		return nil, apierror.Validation(err.Error(), nil)
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}
	created, err := s.staff.Create(ctx, in, hash)
	if err != nil {
		return nil, err
	}
	if len(in.Stores) > 0 {
		if err := s.staff.ReplaceAssignments(ctx, created.ID, in.Stores, p.ID); err != nil {
			return nil, err
		}
	}
	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "staff.create",
		EntityType: "user", EntityID: &created.ID,
		After: map[string]any{"email": in.Email, "role": in.Role, "stores": in.Stores},
	})
	return created, nil
}

func (s *Service) UpdateStaff(ctx context.Context, p security.Principal, in ports.StaffView) error {
	before, err := s.staff.Get(ctx, in.ID)
	if err != nil {
		return err
	}
	if err := s.staff.Update(ctx, in); err != nil {
		return err
	}
	if in.Stores != nil {
		if err := s.staff.ReplaceAssignments(ctx, in.ID, in.Stores, p.ID); err != nil {
			return err
		}
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "staff.update",
		EntityType: "user", EntityID: &in.ID, Before: before, After: in,
	})
}

// DeactivateStaff never deletes: the audit trail must outlive the person
// (docs/06 §2.7).
func (s *Service) DeactivateStaff(ctx context.Context, p security.Principal, userID uuid.UUID) error {
	if err := s.staff.Deactivate(ctx, userID); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "staff.deactivate",
		EntityType: "user", EntityID: &userID,
	})
}

// ── Audit viewer ─────────────────────────────────────────────────────────────

func (s *Service) Audit(ctx context.Context, p security.Principal, q, entityType string,
	storeID *uuid.UUID, from, to *time.Time, limit int) ([]AuditRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.auditRead.List(ctx, q, entityType, storeID, from, to, limit, p.StoreScope())
}
