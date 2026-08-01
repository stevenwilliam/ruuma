// Package ports declares what the app layer needs from the outside world.
//
// The dependency rule (CLAUDE.md §2) points inward: services depend on these
// interfaces and on domain types, never on gorm, gin or the postgres package.
// internal/architecture_test.go enforces it.
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/domain/catalog"
	"github.com/stevenwilliam/ruuma/internal/domain/identity"
	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/domain/order"
	"github.com/stevenwilliam/ruuma/internal/domain/payment"
	"github.com/stevenwilliam/ruuma/internal/domain/pricing"
	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
)

// ── Data transfer objects ────────────────────────────────────────────────────

// StoreView is a store as customers and staff see it.
type StoreView struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Slug        string
	AddressLine string
	City        string
	Phone       string
	Timezone    string
	IsActive    bool
	Modes       []schedule.FulfilmentType
}

// BankAccountView is the destination shown at checkout (BR-2.1.13).
type BankAccountView struct {
	ID            uuid.UUID
	BankName      string
	AccountName   string
	AccountNumber string
}

// MenuItemView is a menu item resolved for one store (BR-2.2.1).
type MenuItemView struct {
	ID              uuid.UUID
	CategoryID      uuid.UUID
	CategoryNameID  string
	CategoryNameEN  string
	Cuisine         string
	SKU             string
	NameID          string
	NameEN          string
	DescriptionID   string
	DescriptionEN   string
	Price           money.Rupiah
	KitchenUnits    int
	PrepMinutes     int
	MinLeadMinutes  int
	PhotoKey        string
	SpiceLevel      int
	IsHalal         bool
	IsVegetarian    bool
	ContainsPork    bool
	ContainsAlcohol bool
	ContainsNuts    bool
	Availability    catalog.Availability
	StockLeft       *int
}

// OptionGroupView is an item's option group with its choices.
type OptionGroupView struct {
	ID         uuid.UUID
	NameID     string
	NameEN     string
	Selection  catalog.Selection
	IsRequired bool
	MinSelect  int
	MaxSelect  int
	Choices    []OptionChoiceView
}

// OptionChoiceView is one selectable choice.
type OptionChoiceView struct {
	ID           uuid.UUID
	NameID       string
	NameEN       string
	PriceDelta   money.Rupiah
	KitchenUnits int
	IsAvailable  bool
}

// SlotView is a slot offered to a customer, always carrying a reason when it is
// not bookable (BR-2.3.6).
type SlotView struct {
	ID              uuid.UUID
	StartsAt        time.Time
	EndsAt          time.Time
	IsBookable      bool
	Reason          schedule.Reason
	RemainingOrders int
	RemainingUnits  int
	AlmostFull      bool
}

// OrderLineView is one line of an order, with its snapshotted names and prices.
type OrderLineView struct {
	ID           uuid.UUID
	MenuItemID   uuid.UUID
	ItemNameID   string
	ItemNameEN   string
	UnitPrice    money.Rupiah
	Qty          int
	OptionsDelta money.Rupiah
	LineTotal    money.Rupiah
	KitchenUnits int
	Notes        string
	Options      []OrderLineOptionView
}

// OrderLineOptionView is a chosen option, snapshotted.
type OrderLineOptionView struct {
	OptionGroupID  uuid.UUID
	OptionChoiceID uuid.UUID
	GroupNameID    string
	ChoiceNameID   string
	ChoiceNameEN   string
	PriceDelta     money.Rupiah
}

// OrderView is an order as any caller sees it.
type OrderView struct {
	ID             uuid.UUID
	OrderCode      string
	StoreID        uuid.UUID
	StoreName      string
	CustomerID     uuid.UUID
	SlotID         uuid.UUID
	FulfilmentType schedule.FulfilmentType
	BusinessDate   time.Time
	SlotStartsAt   time.Time
	SlotEndsAt     time.Time
	Status         order.Status
	ContactName    string
	ContactPhone   string
	Notes          string
	Subtotal       money.Rupiah
	Discount       money.Rupiah
	ServiceCharge  money.Rupiah
	Tax            money.Rupiah
	DeliveryFee    money.Rupiah
	Total          money.Rupiah
	UniqueCode     int
	AmountDue      money.Rupiah
	PromoCode      string
	KitchenUnits   int
	CreatedAt      time.Time
	Lines          []OrderLineView
}

// OrderEventView is one append-only history row (BR-2.4.4).
type OrderEventView struct {
	FromStatus string
	ToStatus   string
	ActorType  string
	Reason     string
	CreatedAt  time.Time
}

// PaymentView is an order's payment state.
type PaymentView struct {
	ID              uuid.UUID
	OrderID         uuid.UUID
	StoreID         uuid.UUID
	Method          payment.Method
	Status          payment.Status
	AmountDue       money.Rupiah
	DeclaredAmount  money.Rupiah
	SenderName      string
	HasProof        bool
	ProofObjectKey  string
	ProofUploadedAt *time.Time
	RejectionReason string
	RejectionNote   string
	VerifiedAt      *time.Time
	RefundedAmount  money.Rupiah
}

// QueueItemView is one row of the finance queue with its order context.
type QueueItemView struct {
	Payment      PaymentView
	OrderCode    string
	ContactName  string
	StoreName    string
	SlotStartsAt time.Time
	AgeMinutes   int
}

// ProductionRow is one aggregated kitchen line (BR-2.8.2).
type ProductionRow struct {
	MenuItemID  uuid.UUID
	ItemName    string
	OptionName  string
	Qty         int
	PrepMinutes int
}

// ReconciliationView is a store's daily takings (docs/06 §3).
type ReconciliationView struct {
	StoreID     uuid.UUID
	StoreName   string
	Orders      int
	OrderTotal  money.Rupiah
	UniqueCodes money.Rupiah
	Declared    money.Rupiah
	Refunds     money.Rupiah
	Mismatches  int
	Rejections  int
}

// CustomerView is a customer account.
type CustomerView struct {
	ID                uuid.UUID
	FullName          string
	Email             string
	EmailVerifiedAt   *time.Time
	Phone             string
	PhoneVerifiedAt   *time.Time
	PreferredLanguage string
	MarketingOptIn    bool
	IsActive          bool
	HasPassword       bool
	LockedUntil       *time.Time
}

// StaffView is a staff account with its store assignments.
type StaffView struct {
	ID           uuid.UUID
	Email        string
	FullName     string
	Role         string
	IsGroupScope bool
	IsActive     bool
	Stores       []uuid.UUID
	LockedUntil  *time.Time
}

// NewOrderInput is a fully-priced order ready to be persisted.
type NewOrderInput struct {
	StoreID          uuid.UUID
	CustomerID       uuid.UUID
	SlotID           uuid.UUID
	FulfilmentType   schedule.FulfilmentType
	BusinessDate     time.Time
	SlotStartsAt     time.Time
	SlotEndsAt       time.Time
	ContactName      string
	ContactPhone     string
	Notes            string
	Totals           pricing.Totals
	TaxBps           money.Bps
	ServiceChargeBps money.Bps
	KitchenUnits     int
	PromotionID      *uuid.UUID
	PromoCode        string
	BankAccountID    *uuid.UUID
	MaxUnpaid        int
	Lines            []NewOrderLineInput
}

// NewOrderLineInput is one priced line with its snapshots.
type NewOrderLineInput struct {
	MenuItemID   uuid.UUID
	ItemNameID   string
	ItemNameEN   string
	UnitPrice    money.Rupiah
	Qty          int
	OptionsDelta money.Rupiah
	LineTotal    money.Rupiah
	KitchenUnits int
	Notes        string
	Options      []NewOrderLineOptionInput
}

// NewOrderLineOptionInput is one chosen option, snapshotted.
type NewOrderLineOptionInput struct {
	OptionGroupID  uuid.UUID
	OptionChoiceID uuid.UUID
	GroupNameID    string
	ChoiceNameID   string
	ChoiceNameEN   string
	PriceDelta     money.Rupiah
}

// ── Repository ports ─────────────────────────────────────────────────────────

// Stores reads store master data and assembles the domain's schedule value.
type Stores interface {
	ListActive(ctx context.Context, q string) ([]StoreView, error)
	ListAll(ctx context.Context, q string, scope []uuid.UUID) ([]StoreView, error)
	Get(ctx context.Context, id uuid.UUID) (*StoreView, error)
	LoadSchedule(ctx context.Context, storeID uuid.UUID, from, to schedule.Date) (schedule.Store, error)
	PrimaryBankAccount(ctx context.Context, storeID uuid.UUID) (*BankAccountView, error)
	AssignedStores(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	CountAffectedOrders(ctx context.Context, storeID uuid.UUID, date time.Time) (int64, error)
}

// MenuQuery filters the customer-facing menu.
type MenuQuery struct {
	StoreID    uuid.UUID
	Q          string
	CategoryID *uuid.UUID
	Cuisine    string
	Diet       string
	Sort       string
	Limit      int
	Offset     int
}

// Catalogue resolves the menu against a store.
type Catalogue interface {
	Menu(ctx context.Context, q MenuQuery, now time.Time) ([]MenuItemView, error)
	Item(ctx context.Context, storeID, itemID uuid.UUID, now time.Time) (*MenuItemView, error)
	Options(ctx context.Context, itemID uuid.UUID) ([]OptionGroupView, error)
	ResolveForSlot(ctx context.Context, storeID uuid.UUID, itemIDs []uuid.UUID, now, slotStartLocal time.Time) ([]MenuItemView, error)
	Categories(ctx context.Context, q string, activeOnly bool) ([]CategoryView, error)
}

// CategoryView is a menu category.
type CategoryView struct {
	ID       uuid.UUID
	NameID   string
	NameEN   string
	Slug     string
	Cuisine  string
	IsActive bool
}

// Slots materialises and reads fulfilment slots.
type Slots interface {
	Materialise(ctx context.Context, storeID uuid.UUID, generated []schedule.Slot, params schedule.Params) (int, error)
	ListForDate(ctx context.Context, storeID uuid.UUID, date time.Time, mode string) ([]schedule.SlotState, []uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*SlotDetail, error)
}

// SlotDetail is a materialised slot with its identity.
type SlotDetail struct {
	ID      uuid.UUID
	StoreID uuid.UUID
	State   schedule.SlotState
}

// Orders persists and reads orders.
type Orders interface {
	Create(ctx context.Context, in NewOrderInput) (*OrderView, error)
	GetForCustomer(ctx context.Context, orderID, customerID uuid.UUID) (*OrderView, error)
	GetByCodeForCustomer(ctx context.Context, code string, customerID uuid.UUID) (*OrderView, error)
	GetInScope(ctx context.Context, orderID uuid.UUID, scope []uuid.UUID) (*OrderView, error)
	ListForCustomer(ctx context.Context, customerID uuid.UUID, q string, limit int, before *time.Time) ([]OrderView, error)
	Board(ctx context.Context, storeID *uuid.UUID, date *time.Time, statuses []string, q string, limit int, scope []uuid.UUID) ([]OrderView, error)
	Unpaid(ctx context.Context, storeID *uuid.UUID, scope []uuid.UUID) ([]OrderView, error)
	AffectedByClosure(ctx context.Context, storeID uuid.UUID, date time.Time, scope []uuid.UUID) ([]OrderView, error)
	Transition(ctx context.Context, orderID uuid.UUID, to order.Status, actorType order.ActorType, actorID *uuid.UUID, reason string, scope []uuid.UUID) error
	Events(ctx context.Context, orderID uuid.UUID) ([]OrderEventView, error)
	ProductionSummary(ctx context.Context, slotID uuid.UUID, scope []uuid.UUID) ([]ProductionRow, error)
}

// Payments runs the finance queue and decisions.
type Payments interface {
	ForOrder(ctx context.Context, orderID uuid.UUID) (*PaymentView, error)
	Get(ctx context.Context, id uuid.UUID, scope []uuid.UUID) (*PaymentView, error)
	Queue(ctx context.Context, storeID *uuid.UUID, statuses []string, q string, limit int, scope []uuid.UUID) ([]QueueItemView, error)
	AttachProof(ctx context.Context, orderID, customerID uuid.UUID, objectKey string, declared money.Rupiah, sender string) error
	Verify(ctx context.Context, in Decision) (*PaymentView, error)
	Reject(ctx context.Context, in Decision, reason payment.RejectionReason, note string) error
	Refund(ctx context.Context, in Decision, amount money.Rupiah, reference, reason, proofKey string) error
	Reconciliation(ctx context.Context, date time.Time, storeID *uuid.UUID, scope []uuid.UUID) ([]ReconciliationView, error)
}

// Decision is a finance action.
type Decision struct {
	PaymentID      uuid.UUID
	ActorID        uuid.UUID
	ActorRole      string
	IsFinance      bool
	Scope          []uuid.UUID
	AcceptMismatch bool
	MismatchReason string
}

// Promotions loads promo codes with their explicit store scope (D15).
type Promotions interface {
	ByCode(ctx context.Context, code string, customerID uuid.UUID) (uuid.UUID, pricing.Promotion, error)
}

// Customers owns accounts and identities.
type Customers interface {
	Get(ctx context.Context, id uuid.UUID) (*CustomerView, error)
	ByEmail(ctx context.Context, email string) (*CustomerView, error)
	ByPhone(ctx context.Context, phone string) (*CustomerView, error)
	ByIdentity(ctx context.Context, provider identity.Provider, providerUserID string) (*CustomerView, error)
	PasswordHash(ctx context.Context, customerID uuid.UUID) (string, error)
	Create(ctx context.Context, in CustomerView, passwordHash string) (*CustomerView, error)
	LinkIdentity(ctx context.Context, customerID uuid.UUID, provider identity.Provider, providerUserID, email string, verified bool) error
	MarkPhoneVerified(ctx context.Context, customerID uuid.UUID, phone string) error
	MarkEmailVerified(ctx context.Context, customerID uuid.UUID) error
	RecordFailedLogin(ctx context.Context, customerID uuid.UUID, lockAfter int, lockFor time.Duration) error
	ClearFailedLogins(ctx context.Context, customerID uuid.UUID) error
	UpdateProfile(ctx context.Context, customerID uuid.UUID, fullName, language string, optIn bool) error
}

// Staff owns staff accounts.
type Staff interface {
	Get(ctx context.Context, id uuid.UUID) (*StaffView, error)
	ByEmail(ctx context.Context, email string) (*StaffView, error)
	PasswordHash(ctx context.Context, userID uuid.UUID) (string, error)
	List(ctx context.Context, q string) ([]StaffView, error)
	Create(ctx context.Context, in StaffView, passwordHash string) (*StaffView, error)
	Update(ctx context.Context, in StaffView) error
	Deactivate(ctx context.Context, id uuid.UUID) error
	ReplaceAssignments(ctx context.Context, userID uuid.UUID, storeIDs []uuid.UUID, actor uuid.UUID) error
	RecordFailedLogin(ctx context.Context, userID uuid.UUID, lockAfter int, lockFor time.Duration) error
	RecordLogin(ctx context.Context, userID uuid.UUID) error
}

// Tokens stores hashed sessions, verification tokens and OTPs.
type Tokens interface {
	StoreRefresh(ctx context.Context, subjectType string, subjectID uuid.UUID, raw string, jti uuid.UUID, parent *uuid.UUID, ttl time.Duration, ua, ip string) error
	ConsumeRefresh(ctx context.Context, raw string) (subjectType string, subjectID uuid.UUID, jti uuid.UUID, err error)
	RevokeAllRefresh(ctx context.Context, subjectType string, subjectID uuid.UUID) error
	CreateVerification(ctx context.Context, subjectType string, subjectID uuid.UUID, raw, purpose string, ttl time.Duration) error
	ConsumeVerification(ctx context.Context, raw, purpose string) (subjectType string, subjectID uuid.UUID, err error)
	CreateOTP(ctx context.Context, phone, codeHash, purpose string, ttl time.Duration, ip string) error
	LatestOTP(ctx context.Context, phone, purpose string) (*OTPView, error)
	RecordOTPAttempt(ctx context.Context, id uuid.UUID) error
	ConsumeOTP(ctx context.Context, id uuid.UUID) error
}

// OTPView is a stored one-time code (the code itself is only ever a hash).
type OTPView struct {
	ID       uuid.UUID
	CodeHash string
	OTP      identity.OTP
}

// Params resolves configuration store → group → fallback (BR-1.4.4).
type Params interface {
	String(ctx context.Context, storeID *uuid.UUID, key string) string
	Int(ctx context.Context, storeID *uuid.UUID, key string) int
	Bool(ctx context.Context, storeID *uuid.UUID, key string) bool
	Bps(ctx context.Context, storeID *uuid.UUID, key string) money.Bps
}

// Auditor appends privileged-action records (BR-2.10.1).
type Auditor interface {
	Write(ctx context.Context, e AuditEntry) error
}

// AuditEntry is one audit record.
type AuditEntry struct {
	ActorType  string
	ActorID    *uuid.UUID
	ActorEmail string
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	StoreID    *uuid.UUID
	Before     any
	After      any
	IP         string
	UserAgent  string
	RequestID  string
}

// Notifier queues customer messages (BR-2.10.3).
type Notifier interface {
	Queue(ctx context.Context, n QueuedNotification) error
}

// QueuedNotification is one message waiting to be sent.
type QueuedNotification struct {
	OrderID     *uuid.UUID
	CustomerID  *uuid.UUID
	Channel     string
	Provider    string
	Event       string
	Target      string
	TemplateKey string
	Language    string
	Body        string
}

// Storage stores private objects and issues presigned URLs (docs/12 §3).
type Storage interface {
	PutProof(ctx context.Context, prefix string, data []byte) (string, error)
	PutPhoto(ctx context.Context, prefix string, data []byte) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// Mailer sends transactional email.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Clock is the injected time source (docs/05 §3.3).
type Clock interface {
	Now() time.Time
}
