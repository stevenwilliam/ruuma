package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/identity"
	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/domain/order"
	dpay "github.com/stevenwilliam/ruuma/internal/domain/payment"
	"github.com/stevenwilliam/ruuma/internal/domain/pricing"
	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
)

// The types below adapt the repositories to the app layer's ports. They are
// thin: all behaviour lives in the repositories, all shapes in mappers.go.

// ── Stores ───────────────────────────────────────────────────────────────────

type StoresPort struct {
	repo *StoreRepo
	db   *gorm.DB
}

func NewStoresPort(repo *StoreRepo, db *gorm.DB) *StoresPort { return &StoresPort{repo: repo, db: db} }

var _ ports.Stores = (*StoresPort)(nil)

func (p *StoresPort) ListActive(ctx context.Context, q string) ([]ports.StoreView, error) {
	rows, err := p.repo.ListActive(ctx, q)
	if err != nil {
		return nil, err
	}
	return p.withModes(ctx, rows)
}

func (p *StoresPort) ListAll(ctx context.Context, q string, scope []uuid.UUID) ([]ports.StoreView, error) {
	rows, err := p.repo.ListAll(ctx, q, scope)
	if err != nil {
		return nil, err
	}
	return p.withModes(ctx, rows)
}

func (p *StoresPort) withModes(ctx context.Context, rows []Store) ([]ports.StoreView, error) {
	out := make([]ports.StoreView, 0, len(rows))
	for _, s := range rows {
		modes, err := p.repo.Modes(ctx, s.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toStoreView(s, modes))
	}
	return out, nil
}

func (p *StoresPort) Get(ctx context.Context, id uuid.UUID) (*ports.StoreView, error) {
	s, err := p.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	modes, err := p.repo.Modes(ctx, id)
	if err != nil {
		return nil, err
	}
	v := toStoreView(*s, modes)
	return &v, nil
}

func (p *StoresPort) LoadSchedule(ctx context.Context, storeID uuid.UUID, from, to schedule.Date) (schedule.Store, error) {
	s, _, err := p.repo.LoadSchedule(ctx, storeID, from, to)
	return s, err
}

func (p *StoresPort) PrimaryBankAccount(ctx context.Context, storeID uuid.UUID) (*ports.BankAccountView, error) {
	a, err := p.repo.PrimaryBankAccount(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return &ports.BankAccountView{
		ID: a.ID, BankName: a.BankName, AccountName: a.AccountName, AccountNumber: a.AccountNumber,
	}, nil
}

func (p *StoresPort) AssignedStores(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return p.repo.AssignedStores(ctx, userID)
}

func (p *StoresPort) CountAffectedOrders(ctx context.Context, storeID uuid.UUID, date time.Time) (int64, error) {
	return p.repo.CountAffectedOrders(ctx, storeID, date)
}

// ── Catalogue ────────────────────────────────────────────────────────────────

type CataloguePort struct{ repo *CatalogRepo }

func NewCataloguePort(repo *CatalogRepo) *CataloguePort { return &CataloguePort{repo: repo} }

var _ ports.Catalogue = (*CataloguePort)(nil)

func (p *CataloguePort) Menu(ctx context.Context, q ports.MenuQuery, now time.Time) ([]ports.MenuItemView, error) {
	rows, err := p.repo.Menu(ctx, MenuFilter{
		StoreID: q.StoreID, Q: q.Q, CategoryID: q.CategoryID, Cuisine: q.Cuisine,
		Diet: q.Diet, Sort: q.Sort, Limit: q.Limit, Offset: q.Offset,
	}, now)
	if err != nil {
		return nil, err
	}
	out := make([]ports.MenuItemView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toMenuItemView(r))
	}
	return out, nil
}

func (p *CataloguePort) Item(ctx context.Context, storeID, itemID uuid.UUID, now time.Time) (*ports.MenuItemView, error) {
	r, err := p.repo.Item(ctx, storeID, itemID, now)
	if err != nil {
		return nil, err
	}
	v := toMenuItemView(*r)
	return &v, nil
}

func (p *CataloguePort) Options(ctx context.Context, itemID uuid.UUID) ([]ports.OptionGroupView, error) {
	_, groups, choices, err := p.repo.OptionsFor(ctx, itemID)
	if err != nil {
		return nil, err
	}
	ordered := make([]OptionGroup, 0, len(groups))
	for _, g := range groups {
		ordered = append(ordered, g)
	}
	return toOptionGroupViews(ordered, choices), nil
}

func (p *CataloguePort) ResolveForSlot(ctx context.Context, storeID uuid.UUID, itemIDs []uuid.UUID,
	now, slotStartLocal time.Time) ([]ports.MenuItemView, error) {

	rows, err := p.repo.ResolveForSlot(ctx, storeID, itemIDs, now, slotStartLocal)
	if err != nil {
		return nil, err
	}
	out := make([]ports.MenuItemView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toMenuItemView(r))
	}
	return out, nil
}

func (p *CataloguePort) Categories(ctx context.Context, q string, activeOnly bool) ([]ports.CategoryView, error) {
	rows, err := p.repo.Categories(ctx, q, activeOnly)
	if err != nil {
		return nil, err
	}
	out := make([]ports.CategoryView, 0, len(rows))
	for _, c := range rows {
		out = append(out, ports.CategoryView{
			ID: c.ID, NameID: c.NameID, NameEN: c.NameEN, Slug: c.Slug,
			Cuisine: c.Cuisine, IsActive: c.IsActive,
		})
	}
	return out, nil
}

// ── Slots ────────────────────────────────────────────────────────────────────

type SlotsPort struct{ repo *SlotRepo }

func NewSlotsPort(repo *SlotRepo) *SlotsPort { return &SlotsPort{repo: repo} }

var _ ports.Slots = (*SlotsPort)(nil)

func (p *SlotsPort) Materialise(ctx context.Context, storeID uuid.UUID, generated []schedule.Slot, params schedule.Params) (int, error) {
	return p.repo.Materialise(ctx, storeID, generated, params)
}

func (p *SlotsPort) ListForDate(ctx context.Context, storeID uuid.UUID, date time.Time, mode string) ([]schedule.SlotState, []uuid.UUID, error) {
	rows, err := p.repo.ListForDate(ctx, storeID, date, mode)
	if err != nil {
		return nil, nil, err
	}
	states := make([]schedule.SlotState, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, s := range rows {
		states = append(states, s.State())
		ids = append(ids, s.ID)
	}
	return states, ids, nil
}

func (p *SlotsPort) Get(ctx context.Context, id uuid.UUID) (*ports.SlotDetail, error) {
	s, err := p.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ports.SlotDetail{ID: s.ID, StoreID: s.StoreID, State: s.State()}, nil
}

// ── Orders ───────────────────────────────────────────────────────────────────

type OrdersPort struct {
	repo   *OrderRepo
	stores *StoreRepo
	db     *gorm.DB
}

func NewOrdersPort(repo *OrderRepo, stores *StoreRepo, db *gorm.DB) *OrdersPort {
	return &OrdersPort{repo: repo, stores: stores, db: db}
}

var _ ports.Orders = (*OrdersPort)(nil)

func (p *OrdersPort) hydrate(ctx context.Context, o *Order) (*ports.OrderView, error) {
	lines, opts, err := p.repo.Lines(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	name := ""
	if s, err := p.stores.Get(ctx, o.StoreID); err == nil {
		name = s.Name
	}
	v := toOrderView(*o, name, lines, opts)
	return &v, nil
}

func (p *OrdersPort) hydrateAll(ctx context.Context, rows []Order) ([]ports.OrderView, error) {
	out := make([]ports.OrderView, 0, len(rows))
	for i := range rows {
		v, err := p.hydrate(ctx, &rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, nil
}

func (p *OrdersPort) Create(ctx context.Context, in ports.NewOrderInput) (*ports.OrderView, error) {
	o, err := p.repo.Create(ctx, toNewOrder(in))
	if err != nil {
		return nil, err
	}
	return p.hydrate(ctx, o)
}

func (p *OrdersPort) GetForCustomer(ctx context.Context, orderID, customerID uuid.UUID) (*ports.OrderView, error) {
	o, err := p.repo.GetForCustomer(ctx, orderID, customerID)
	if err != nil {
		return nil, err
	}
	return p.hydrate(ctx, o)
}

func (p *OrdersPort) GetByCodeForCustomer(ctx context.Context, code string, customerID uuid.UUID) (*ports.OrderView, error) {
	o, err := p.repo.GetByCodeForCustomer(ctx, code, customerID)
	if err != nil {
		return nil, err
	}
	return p.hydrate(ctx, o)
}

func (p *OrdersPort) GetInScope(ctx context.Context, orderID uuid.UUID, scope []uuid.UUID) (*ports.OrderView, error) {
	o, err := p.repo.GetInScope(ctx, orderID, scope)
	if err != nil {
		return nil, err
	}
	return p.hydrate(ctx, o)
}

func (p *OrdersPort) ListForCustomer(ctx context.Context, customerID uuid.UUID, q string, limit int, before *time.Time) ([]ports.OrderView, error) {
	rows, err := p.repo.ListForCustomer(ctx, customerID, q, limit, before)
	if err != nil {
		return nil, err
	}
	return p.hydrateAll(ctx, rows)
}

func (p *OrdersPort) Board(ctx context.Context, storeID *uuid.UUID, date *time.Time, statuses []string, q string, limit int, scope []uuid.UUID) ([]ports.OrderView, error) {
	rows, err := p.repo.Board(ctx, BoardFilter{
		StoreID: storeID, Date: date, Statuses: statuses, Q: q, Limit: limit,
	}, scope)
	if err != nil {
		return nil, err
	}
	return p.hydrateAll(ctx, rows)
}

func (p *OrdersPort) Unpaid(ctx context.Context, storeID *uuid.UUID, scope []uuid.UUID) ([]ports.OrderView, error) {
	rows, err := p.repo.Unpaid(ctx, storeID, scope)
	if err != nil {
		return nil, err
	}
	return p.hydrateAll(ctx, rows)
}

func (p *OrdersPort) AffectedByClosure(ctx context.Context, storeID uuid.UUID, date time.Time, scope []uuid.UUID) ([]ports.OrderView, error) {
	rows, err := p.repo.AffectedByClosure(ctx, storeID, date, scope)
	if err != nil {
		return nil, err
	}
	return p.hydrateAll(ctx, rows)
}

func (p *OrdersPort) Transition(ctx context.Context, orderID uuid.UUID, to order.Status,
	actorType order.ActorType, actorID *uuid.UUID, reason string, scope []uuid.UUID) error {
	return p.repo.Transition(ctx, orderID, to, actorType, actorID, reason, scope)
}

func (p *OrdersPort) Events(ctx context.Context, orderID uuid.UUID) ([]ports.OrderEventView, error) {
	rows, err := p.repo.Events(ctx, orderID)
	if err != nil {
		return nil, err
	}
	out := make([]ports.OrderEventView, 0, len(rows))
	for _, e := range rows {
		out = append(out, ports.OrderEventView{
			FromStatus: str(e.FromStatus), ToStatus: e.ToStatus,
			ActorType: e.ActorType, Reason: str(e.Reason), CreatedAt: e.CreatedAt,
		})
	}
	return out, nil
}

func (p *OrdersPort) ProductionSummary(ctx context.Context, slotID uuid.UUID, scope []uuid.UUID) ([]ports.ProductionRow, error) {
	rows, err := p.repo.ProductionSummary(ctx, slotID, scope)
	if err != nil {
		return nil, err
	}
	out := make([]ports.ProductionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ports.ProductionRow{
			MenuItemID: r.MenuItemID, ItemName: r.ItemName, OptionName: str(r.OptionName),
			Qty: r.Qty, PrepMinutes: r.PrepMinutes,
		})
	}
	return out, nil
}

// ── Payments ─────────────────────────────────────────────────────────────────

type PaymentsPort struct {
	repo *PaymentRepo
	now  func() time.Time
}

func NewPaymentsPort(repo *PaymentRepo, now func() time.Time) *PaymentsPort {
	if now == nil {
		now = time.Now
	}
	return &PaymentsPort{repo: repo, now: now}
}

var _ ports.Payments = (*PaymentsPort)(nil)

func (p *PaymentsPort) ForOrder(ctx context.Context, orderID uuid.UUID) (*ports.PaymentView, error) {
	row, err := p.repo.ForOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	v := toPaymentView(*row)
	return &v, nil
}

func (p *PaymentsPort) Get(ctx context.Context, id uuid.UUID, scope []uuid.UUID) (*ports.PaymentView, error) {
	row, err := p.repo.Get(ctx, id, scope)
	if err != nil {
		return nil, err
	}
	v := toPaymentView(*row)
	return &v, nil
}

func (p *PaymentsPort) Queue(ctx context.Context, storeID *uuid.UUID, statuses []string, q string, limit int, scope []uuid.UUID) ([]ports.QueueItemView, error) {
	rows, err := p.repo.Queue(ctx, QueueFilter{StoreID: storeID, Statuses: statuses, Q: q, Limit: limit}, scope)
	if err != nil {
		return nil, err
	}
	now := p.now()
	out := make([]ports.QueueItemView, 0, len(rows))
	for _, r := range rows {
		out = append(out, ports.QueueItemView{
			Payment: toPaymentView(r.Payment), OrderCode: r.OrderCode,
			ContactName: r.ContactName, StoreName: r.StoreName,
			SlotStartsAt: r.SlotStartsAt,
			AgeMinutes:   ageMinutes(r.Payment.ProofUploadedAt, now),
		})
	}
	return out, nil
}

func (p *PaymentsPort) AttachProof(ctx context.Context, orderID, customerID uuid.UUID, key string, declared money.Rupiah, sender string) error {
	return p.repo.AttachProof(ctx, orderID, customerID, key, int64(declared), sender)
}

func toDecide(in ports.Decision) Decide {
	return Decide{
		PaymentID: in.PaymentID, ActorID: in.ActorID, ActorRole: in.ActorRole,
		IsFinance: in.IsFinance, Scope: in.Scope,
		AcceptMismatch: in.AcceptMismatch, MismatchReason: in.MismatchReason,
	}
}

func (p *PaymentsPort) Verify(ctx context.Context, in ports.Decision) (*ports.PaymentView, error) {
	row, err := p.repo.Verify(ctx, toDecide(in))
	if err != nil {
		return nil, err
	}
	v := toPaymentView(*row)
	return &v, nil
}

func (p *PaymentsPort) Reject(ctx context.Context, in ports.Decision, reason dpay.RejectionReason, note string) error {
	return p.repo.Reject(ctx, toDecide(in), reason, note)
}

func (p *PaymentsPort) Refund(ctx context.Context, in ports.Decision, amount money.Rupiah, reference, reason, proofKey string) error {
	return p.repo.Refund(ctx, toDecide(in), int64(amount), reference, reason, proofKey)
}

func (p *PaymentsPort) Reconciliation(ctx context.Context, date time.Time, storeID *uuid.UUID, scope []uuid.UUID) ([]ports.ReconciliationView, error) {
	rows, err := p.repo.Reconciliation(ctx, date, storeID, scope)
	if err != nil {
		return nil, err
	}
	out := make([]ports.ReconciliationView, 0, len(rows))
	for _, r := range rows {
		out = append(out, ports.ReconciliationView{
			StoreID: r.StoreID, StoreName: r.StoreName, Orders: r.Orders,
			OrderTotal: money.Rupiah(r.OrderTotal), UniqueCodes: money.Rupiah(r.UniqueCodes),
			Declared: money.Rupiah(r.Declared), Refunds: money.Rupiah(r.Refunds),
			Mismatches: r.Mismatches, Rejections: r.Rejections,
		})
	}
	return out, nil
}

// ── Promotions ───────────────────────────────────────────────────────────────

type PromotionsPort struct{ repo *PromoRepo }

func NewPromotionsPort(repo *PromoRepo) *PromotionsPort { return &PromotionsPort{repo: repo} }

var _ ports.Promotions = (*PromotionsPort)(nil)

func (p *PromotionsPort) ByCode(ctx context.Context, code string, customerID uuid.UUID) (uuid.UUID, pricing.Promotion, error) {
	row, dp, err := p.repo.ByCode(ctx, code, customerID)
	if err != nil {
		return uuid.Nil, pricing.Promotion{}, err
	}
	return row.ID, dp, nil
}

// ── Customers and staff ──────────────────────────────────────────────────────

type CustomersPort struct{ repo *CustomerRepo }

func NewCustomersPort(repo *CustomerRepo) *CustomersPort { return &CustomersPort{repo: repo} }

var _ ports.Customers = (*CustomersPort)(nil)

func (p *CustomersPort) Get(ctx context.Context, id uuid.UUID) (*ports.CustomerView, error) {
	c, err := p.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	v := toCustomerView(*c)
	return &v, nil
}

func (p *CustomersPort) ByEmail(ctx context.Context, email string) (*ports.CustomerView, error) {
	c, err := p.repo.ByEmail(ctx, email)
	if err != nil || c == nil {
		return nil, err
	}
	v := toCustomerView(*c)
	return &v, nil
}

func (p *CustomersPort) ByPhone(ctx context.Context, phone string) (*ports.CustomerView, error) {
	c, err := p.repo.ByPhone(ctx, phone)
	if err != nil || c == nil {
		return nil, err
	}
	v := toCustomerView(*c)
	return &v, nil
}

func (p *CustomersPort) ByIdentity(ctx context.Context, provider identity.Provider, providerUserID string) (*ports.CustomerView, error) {
	c, err := p.repo.ByIdentity(ctx, provider, providerUserID)
	if err != nil || c == nil {
		return nil, err
	}
	v := toCustomerView(*c)
	return &v, nil
}

func (p *CustomersPort) PasswordHash(ctx context.Context, customerID uuid.UUID) (string, error) {
	c, err := p.repo.Get(ctx, customerID)
	if err != nil {
		return "", err
	}
	return str(c.PasswordHash), nil
}

func (p *CustomersPort) Create(ctx context.Context, in ports.CustomerView, passwordHash string) (*ports.CustomerView, error) {
	c := Customer{
		FullName: in.FullName, PreferredLanguage: in.PreferredLanguage,
		MarketingOptIn: in.MarketingOptIn,
	}
	if in.Email != "" {
		c.Email = &in.Email
	}
	if in.Phone != "" {
		c.Phone = &in.Phone
	}
	if passwordHash != "" {
		c.PasswordHash = &passwordHash
	}
	if err := p.repo.Create(ctx, &c); err != nil {
		return nil, err
	}
	v := toCustomerView(c)
	return &v, nil
}

func (p *CustomersPort) LinkIdentity(ctx context.Context, customerID uuid.UUID, provider identity.Provider, providerUserID, email string, verified bool) error {
	return p.repo.LinkIdentity(ctx, customerID, provider, providerUserID, email, verified)
}

func (p *CustomersPort) MarkPhoneVerified(ctx context.Context, customerID uuid.UUID, phone string) error {
	return p.repo.MarkPhoneVerified(ctx, customerID, phone)
}

func (p *CustomersPort) MarkEmailVerified(ctx context.Context, customerID uuid.UUID) error {
	return p.repo.MarkEmailVerified(ctx, customerID)
}

func (p *CustomersPort) RecordFailedLogin(ctx context.Context, customerID uuid.UUID, lockAfter int, lockFor time.Duration) error {
	return p.repo.RecordFailedLogin(ctx, customerID, lockAfter, lockFor)
}

func (p *CustomersPort) ClearFailedLogins(ctx context.Context, customerID uuid.UUID) error {
	return p.repo.ClearFailedLogins(ctx, customerID)
}

func (p *CustomersPort) UpdateProfile(ctx context.Context, customerID uuid.UUID, fullName, language string, optIn bool) error {
	c, err := p.repo.Get(ctx, customerID)
	if err != nil {
		return err
	}
	if fullName != "" {
		c.FullName = fullName
	}
	if language != "" {
		c.PreferredLanguage = language
	}
	c.MarketingOptIn = optIn
	return p.repo.Update(ctx, c)
}

type StaffPort struct {
	repo   *UserRepo
	stores *StoreRepo
}

func NewStaffPort(repo *UserRepo, stores *StoreRepo) *StaffPort {
	return &StaffPort{repo: repo, stores: stores}
}

var _ ports.Staff = (*StaffPort)(nil)

func (p *StaffPort) Get(ctx context.Context, id uuid.UUID) (*ports.StaffView, error) {
	u, err := p.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	stores, err := p.stores.AssignedStores(ctx, id)
	if err != nil {
		return nil, err
	}
	v := toStaffView(*u, stores)
	return &v, nil
}

func (p *StaffPort) ByEmail(ctx context.Context, email string) (*ports.StaffView, error) {
	u, err := p.repo.ByEmail(ctx, email)
	if err != nil || u == nil {
		return nil, err
	}
	stores, err := p.stores.AssignedStores(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	v := toStaffView(*u, stores)
	return &v, nil
}

func (p *StaffPort) PasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	u, err := p.repo.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.PasswordHash, nil
}

func (p *StaffPort) List(ctx context.Context, q string) ([]ports.StaffView, error) {
	rows, err := p.repo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]ports.StaffView, 0, len(rows))
	for _, u := range rows {
		stores, err := p.stores.AssignedStores(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toStaffView(u, stores))
	}
	return out, nil
}

func (p *StaffPort) Create(ctx context.Context, in ports.StaffView, passwordHash string) (*ports.StaffView, error) {
	u := User{
		Email: in.Email, FullName: in.FullName, Role: in.Role,
		IsGroupScope: in.IsGroupScope, IsActive: true, PasswordHash: passwordHash,
		MustChangePassword: true,
	}
	if err := p.repo.Create(ctx, &u); err != nil {
		return nil, err
	}
	v := toStaffView(u, nil)
	return &v, nil
}

func (p *StaffPort) Update(ctx context.Context, in ports.StaffView) error {
	u, err := p.repo.Get(ctx, in.ID)
	if err != nil {
		return err
	}
	u.FullName, u.Role, u.IsGroupScope, u.IsActive = in.FullName, in.Role, in.IsGroupScope, in.IsActive
	return p.repo.Update(ctx, u)
}

func (p *StaffPort) Deactivate(ctx context.Context, id uuid.UUID) error {
	return p.repo.Deactivate(ctx, id)
}

func (p *StaffPort) ReplaceAssignments(ctx context.Context, userID uuid.UUID, storeIDs []uuid.UUID, actor uuid.UUID) error {
	return p.stores.ReplaceAssignments(ctx, userID, storeIDs, actor)
}

func (p *StaffPort) RecordFailedLogin(ctx context.Context, userID uuid.UUID, lockAfter int, lockFor time.Duration) error {
	return p.repo.RecordFailedLogin(ctx, userID, lockAfter, lockFor)
}

func (p *StaffPort) RecordLogin(ctx context.Context, userID uuid.UUID) error {
	return p.repo.RecordLogin(ctx, userID)
}

// ── Tokens ───────────────────────────────────────────────────────────────────

type TokensPort struct{ repo *TokenRepo }

func NewTokensPort(repo *TokenRepo) *TokensPort { return &TokensPort{repo: repo} }

var _ ports.Tokens = (*TokensPort)(nil)

func (p *TokensPort) StoreRefresh(ctx context.Context, subjectType string, subjectID uuid.UUID,
	raw string, jti uuid.UUID, parent *uuid.UUID, ttl time.Duration, ua, ip string) error {
	return p.repo.StoreRefresh(ctx, subjectType, subjectID, raw, jti, parent, ttl, ua, ip)
}

func (p *TokensPort) ConsumeRefresh(ctx context.Context, raw string) (string, uuid.UUID, uuid.UUID, error) {
	t, err := p.repo.ConsumeRefresh(ctx, raw)
	if err != nil {
		return "", uuid.Nil, uuid.Nil, err
	}
	return t.SubjectType, t.SubjectID, t.JTI, nil
}

func (p *TokensPort) RevokeAllRefresh(ctx context.Context, subjectType string, subjectID uuid.UUID) error {
	return p.repo.RevokeAllRefresh(ctx, subjectType, subjectID)
}

func (p *TokensPort) CreateVerification(ctx context.Context, subjectType string, subjectID uuid.UUID, raw, purpose string, ttl time.Duration) error {
	return p.repo.CreateVerification(ctx, subjectType, subjectID, raw, purpose, ttl)
}

func (p *TokensPort) ConsumeVerification(ctx context.Context, raw, purpose string) (string, uuid.UUID, error) {
	t, err := p.repo.ConsumeVerification(ctx, raw, purpose)
	if err != nil {
		return "", uuid.Nil, err
	}
	return t.SubjectType, t.SubjectID, nil
}

func (p *TokensPort) CreateOTP(ctx context.Context, phone, codeHash, purpose string, ttl time.Duration, ip string) error {
	return p.repo.CreateOTP(ctx, phone, codeHash, purpose, ttl, ip)
}

func (p *TokensPort) LatestOTP(ctx context.Context, phone, purpose string) (*ports.OTPView, error) {
	o, err := p.repo.LatestOTP(ctx, phone, purpose)
	if err != nil || o == nil {
		return nil, err
	}
	return &ports.OTPView{
		ID: o.ID, CodeHash: o.CodeHash,
		OTP: identity.OTP{
			Purpose: identity.OTPPurpose(o.Purpose), Attempts: o.Attempts,
			ConsumedAt: o.ConsumedAt, ExpiresAt: o.ExpiresAt,
		},
	}, nil
}

func (p *TokensPort) RecordOTPAttempt(ctx context.Context, id uuid.UUID) error {
	return p.repo.RecordOTPAttempt(ctx, id)
}

func (p *TokensPort) ConsumeOTP(ctx context.Context, id uuid.UUID) error {
	return p.repo.ConsumeOTP(ctx, id)
}

// ── Audit and notifications ──────────────────────────────────────────────────

type AuditPort struct{ repo *AuditRepo }

func NewAuditPort(repo *AuditRepo) *AuditPort { return &AuditPort{repo: repo} }

var _ ports.Auditor = (*AuditPort)(nil)

func (p *AuditPort) Write(ctx context.Context, e ports.AuditEntry) error {
	return p.repo.Write(ctx, Entry{
		ActorType: e.ActorType, ActorID: e.ActorID, ActorEmail: e.ActorEmail,
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		StoreID: e.StoreID, Before: e.Before, After: e.After,
		IP: e.IP, UserAgent: e.UserAgent, RequestID: e.RequestID,
	})
}

type NotifyPort struct{ repo *NotifyRepo }

func NewNotifyPort(repo *NotifyRepo) *NotifyPort { return &NotifyPort{repo: repo} }

var _ ports.Notifier = (*NotifyPort)(nil)

func (p *NotifyPort) Queue(ctx context.Context, n ports.QueuedNotification) error {
	return p.repo.Queue(ctx, &Notification{
		OrderID: n.OrderID, CustomerID: n.CustomerID, Channel: n.Channel,
		Provider: n.Provider, Event: n.Event, Target: n.Target,
		TemplateKey: n.TemplateKey, Language: n.Language, Body: n.Body,
	})
}

// ── Params ───────────────────────────────────────────────────────────────────

var _ ports.Params = (*ParamRepo)(nil)
