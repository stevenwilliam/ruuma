// Package postgres holds the gorm repositories.
//
// Two rules govern this package:
//
//  1. **Store scope is enforced here, not only in handlers** (BR-2.7.8). Every
//     method that reads a store-scoped entity takes a scope []uuid.UUID; nil
//     means "every store" and is only ever produced by
//     security.Principal.StoreScope() for admin, owner or group-scoped finance.
//  2. **Money and capacity paths use raw SQL with placeholders** and integer
//     arithmetic (BR-1.1.2, BR-2.3.8) — never ORM expression building.
//
// The migrations are the source of truth; these structs map onto them and
// AutoMigrate is never called.
package postgres

import (
	"time"

	"github.com/google/uuid"
)

type Store struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	Code         string
	Name         string
	Slug         string
	AddressLine  string
	City         string
	Province     *string
	PostalCode   *string
	Latitude     *float64
	Longitude    *float64
	Phone        string
	Timezone     string
	IsActive     bool
	TicketHeader *string
	TicketFooter *string
	SortOrder    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Store) TableName() string { return "stores" }

type StoreFulfilmentMode struct {
	ID             uuid.UUID `gorm:"primaryKey"`
	StoreID        uuid.UUID
	FulfilmentType string
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (StoreFulfilmentMode) TableName() string { return "store_fulfilment_modes" }

type StoreHour struct {
	ID             uuid.UUID `gorm:"primaryKey"`
	StoreID        uuid.UUID
	Weekday        int
	FulfilmentType string
	BlockIndex     int
	IsClosed       bool
	OpensAt        *string // TIME, read as text and parsed in the mapper
	ClosesAt       *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (StoreHour) TableName() string { return "store_hours" }

type StoreDateOverride struct {
	ID             uuid.UUID `gorm:"primaryKey"`
	StoreID        uuid.UUID
	BusinessDate   time.Time
	FulfilmentType string
	BlockIndex     int
	IsClosed       bool
	OpensAt        *string
	ClosesAt       *string
	Reason         *string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (StoreDateOverride) TableName() string { return "store_date_overrides" }

type StoreBlackoutDate struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	StoreID      uuid.UUID
	BusinessDate time.Time
	Reason       string
	CreatedBy    *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (StoreBlackoutDate) TableName() string { return "store_blackout_dates" }

type StoreBankAccount struct {
	ID            uuid.UUID `gorm:"primaryKey"`
	StoreID       uuid.UUID
	BankName      string
	AccountName   string
	AccountNumber string
	IsPrimary     bool
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (StoreBankAccount) TableName() string { return "store_bank_accounts" }

type StoreParameter struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	StoreID   uuid.UUID
	Key       string
	Value     string
	UpdatedBy *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (StoreParameter) TableName() string { return "store_parameters" }

type SysParameter struct {
	ID                 uuid.UUID `gorm:"primaryKey"`
	Key                string
	Value              string
	DataType           string
	Description        *string
	IsSecret           bool
	IsStoreOverridable bool
	UpdatedBy          *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (SysParameter) TableName() string { return "sys_parameters" }

type User struct {
	ID                 uuid.UUID `gorm:"primaryKey"`
	Email              string
	PasswordHash       string
	FullName           string
	Phone              *string
	Role               string
	IsGroupScope       bool
	IsActive           bool
	MustChangePassword bool
	FailedAttempts     int
	LockedUntil        *time.Time
	LastLoginAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (User) TableName() string { return "users" }

type StaffStoreAssignment struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	UserID    uuid.UUID
	StoreID   uuid.UUID
	CreatedBy *uuid.UUID
	CreatedAt time.Time
}

func (StaffStoreAssignment) TableName() string { return "staff_store_assignments" }

type Customer struct {
	ID                uuid.UUID `gorm:"primaryKey"`
	FullName          string
	Email             *string
	EmailVerifiedAt   *time.Time
	Phone             *string
	PhoneVerifiedAt   *time.Time
	PasswordHash      *string
	PreferredLanguage string
	MarketingOptIn    bool
	IsActive          bool
	FailedAttempts    int
	LockedUntil       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Customer) TableName() string { return "customers" }

type CustomerIdentity struct {
	ID             uuid.UUID `gorm:"primaryKey"`
	CustomerID     uuid.UUID
	Provider       string
	ProviderUserID string
	Email          *string
	VerifiedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (CustomerIdentity) TableName() string { return "customer_identities" }

type Address struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	CustomerID  uuid.UUID
	Label       string
	Recipient   string
	Phone       string
	AddressLine string
	Area        *string
	City        string
	PostalCode  *string
	Notes       *string
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Address) TableName() string { return "addresses" }

type OTPCode struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	Phone      string
	CodeHash   string
	Purpose    string
	Attempts   int
	ConsumedAt *time.Time
	ExpiresAt  time.Time
	RequestIP  *string `gorm:"type:inet"`
	CreatedAt  time.Time
}

func (OTPCode) TableName() string { return "otp_codes" }

type VerificationToken struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	SubjectType string
	SubjectID   uuid.UUID
	TokenHash   string
	Purpose     string
	ConsumedAt  *time.Time
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

func (VerificationToken) TableName() string { return "verification_tokens" }

type RefreshToken struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	SubjectType string
	SubjectID   uuid.UUID
	TokenHash   string
	JTI         uuid.UUID `gorm:"column:jti"`
	ParentJTI   *uuid.UUID
	UserAgent   *string
	IP          *string `gorm:"type:inet"`
	RevokedAt   *time.Time
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

type Category struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	NameID    string    `gorm:"column:name_id"`
	NameEN    string    `gorm:"column:name_en"`
	Slug      string
	Cuisine   string
	SortOrder int
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Category) TableName() string { return "categories" }

type MenuItem struct {
	ID              uuid.UUID `gorm:"primaryKey"`
	CategoryID      uuid.UUID
	SKU             string  `gorm:"column:sku"`
	NameID          string  `gorm:"column:name_id"`
	NameEN          string  `gorm:"column:name_en"`
	DescriptionID   *string `gorm:"column:description_id"`
	DescriptionEN   *string `gorm:"column:description_en"`
	BasePrice       int64
	KitchenUnits    int
	PrepMinutes     int
	MinLeadMinutes  int
	PhotoKey        *string
	SpiceLevel      int
	IsHalal         bool
	IsVegetarian    bool
	ContainsPork    bool
	ContainsAlcohol bool
	ContainsNuts    bool
	IsActive        bool
	SortOrder       int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (MenuItem) TableName() string { return "menu_items" }

type OptionGroup struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	MenuItemID uuid.UUID
	NameID     string `gorm:"column:name_id"`
	NameEN     string `gorm:"column:name_en"`
	Selection  string
	IsRequired bool
	MinSelect  int
	MaxSelect  int
	SortOrder  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (OptionGroup) TableName() string { return "option_groups" }

type OptionChoice struct {
	ID            uuid.UUID `gorm:"primaryKey"`
	OptionGroupID uuid.UUID
	NameID        string `gorm:"column:name_id"`
	NameEN        string `gorm:"column:name_en"`
	PriceDelta    int64
	KitchenUnits  int
	IsAvailable   bool
	SortOrder     int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (OptionChoice) TableName() string { return "option_choices" }

type StoreMenuOverride struct {
	ID            uuid.UUID `gorm:"primaryKey"`
	StoreID       uuid.UUID
	MenuItemID    uuid.UUID
	IsAvailable   *bool
	PriceOverride *int64
	UpdatedBy     *uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (StoreMenuOverride) TableName() string { return "store_menu_overrides" }

type Item86 struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	StoreID    uuid.UUID
	MenuItemID uuid.UUID
	StartsAt   time.Time
	EndsAt     *time.Time
	Reason     *string
	CreatedBy  *uuid.UUID
	CreatedAt  time.Time
}

func (Item86) TableName() string { return "item_86s" }

type ItemAvailabilityRule struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	MenuItemID  uuid.UUID
	StoreID     *uuid.UUID
	WeekdayMask int
	FromTime    *string
	ToTime      *string
	CreatedAt   time.Time
}

func (ItemAvailabilityRule) TableName() string { return "item_availability_rules" }

type ItemDailyStock struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	StoreID      uuid.UUID
	MenuItemID   uuid.UUID
	BusinessDate time.Time
	StockTotal   int
	StockUsed    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (ItemDailyStock) TableName() string { return "item_daily_stock" }

type Slot struct {
	ID                   uuid.UUID `gorm:"primaryKey"`
	StoreID              uuid.UUID
	BusinessDate         time.Time
	FulfilmentType       string
	StartsAt             time.Time
	EndsAt               time.Time
	MaxOrders            int
	MaxKitchenUnits      int
	ReservedOrders       int
	ReservedKitchenUnits int
	IsLocked             bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (Slot) TableName() string { return "slots" }

type DeliveryZone struct {
	ID            uuid.UUID `gorm:"primaryKey"`
	StoreID       uuid.UUID
	Name          string
	Fee           int64
	MinOrder      int64
	FreeThreshold *int64
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (DeliveryZone) TableName() string { return "delivery_zones" }

type Promotion struct {
	ID                  uuid.UUID `gorm:"primaryKey"`
	Code                string
	Name                string
	DiscountType        string
	ValueBps            *int
	ValueAmount         *int64
	MaxDiscount         *int64
	MinSpend            int64
	StartsAt            time.Time
	EndsAt              time.Time
	UsageCapTotal       *int
	UsageCapPerCustomer *int
	UsedCount           int
	IsActive            bool
	CreatedBy           *uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (Promotion) TableName() string { return "promotions" }

type PromotionStore struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	PromotionID uuid.UUID
	StoreID     uuid.UUID
}

func (PromotionStore) TableName() string { return "promotion_stores" }

type PromotionCategory struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	PromotionID uuid.UUID
	CategoryID  uuid.UUID
}

func (PromotionCategory) TableName() string { return "promotion_categories" }

type PromotionRedemption struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	PromotionID uuid.UUID
	OrderID     uuid.UUID
	CustomerID  uuid.UUID
	StoreID     uuid.UUID
	Discount    int64
	ReleasedAt  *time.Time
	CreatedAt   time.Time
}

func (PromotionRedemption) TableName() string { return "promotion_redemptions" }

type Order struct {
	ID                 uuid.UUID `gorm:"primaryKey"`
	OrderCode          string
	StoreID            uuid.UUID
	CustomerID         uuid.UUID
	SlotID             uuid.UUID
	FulfilmentType     string
	BusinessDate       time.Time
	SlotStartsAt       time.Time
	SlotEndsAt         time.Time
	Status             string
	ContactName        string
	ContactPhone       string
	AddressID          *uuid.UUID
	DeliveryZoneID     *uuid.UUID
	Notes              *string
	Subtotal           int64
	Discount           int64
	ServiceCharge      int64
	Tax                int64
	DeliveryFee        int64
	Total              int64
	UniqueCode         int
	AmountDue          int64
	TaxBps             int
	ServiceChargeBps   int
	PromotionID        *uuid.UUID
	PromoCode          *string
	KitchenUnits       int
	CancelledReason    *string
	CancelledBy        *uuid.UUID
	CapacityReleasedAt *time.Time
	PlacedAt           *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (Order) TableName() string { return "orders" }

type OrderLine struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	OrderID      uuid.UUID
	MenuItemID   uuid.UUID
	ItemNameID   string `gorm:"column:item_name_id"`
	ItemNameEN   string `gorm:"column:item_name_en"`
	UnitPrice    int64
	Qty          int
	OptionsDelta int64
	LineTotal    int64
	KitchenUnits int
	Notes        *string
	CreatedAt    time.Time
}

func (OrderLine) TableName() string { return "order_lines" }

type OrderLineOption struct {
	ID             uuid.UUID `gorm:"primaryKey"`
	OrderLineID    uuid.UUID
	OptionGroupID  uuid.UUID
	OptionChoiceID uuid.UUID
	GroupNameID    string `gorm:"column:group_name_id"`
	ChoiceNameID   string `gorm:"column:choice_name_id"`
	ChoiceNameEN   string `gorm:"column:choice_name_en"`
	PriceDelta     int64
	CreatedAt      time.Time
}

func (OrderLineOption) TableName() string { return "order_line_options" }

type OrderEvent struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	OrderID    uuid.UUID
	FromStatus *string
	ToStatus   string
	ActorType  string
	ActorID    *uuid.UUID
	Reason     *string
	Metadata   []byte `gorm:"type:jsonb"`
	CreatedAt  time.Time
}

func (OrderEvent) TableName() string { return "order_events" }

type Payment struct {
	ID               uuid.UUID `gorm:"primaryKey"`
	OrderID          uuid.UUID
	StoreID          uuid.UUID
	Method           string
	Status           string
	AmountDue        int64
	DeclaredAmount   *int64
	SenderName       *string
	BankAccountID    *uuid.UUID
	ProofObjectKey   *string
	ProofUploadedAt  *time.Time
	VerifiedBy       *uuid.UUID
	VerifiedAt       *time.Time
	MismatchAccepted bool
	MismatchReason   *string
	RejectionReason  *string
	RejectionNote    *string
	RejectedBy       *uuid.UUID
	RejectedAt       *time.Time
	RefundedAmount   *int64
	RefundReference  *string
	RefundProofKey   *string
	RefundedBy       *uuid.UUID
	RefundedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (Payment) TableName() string { return "payments" }

type PaymentEvent struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	PaymentID uuid.UUID
	OrderID   uuid.UUID
	EventType string
	ActorID   *uuid.UUID
	ActorRole *string
	Amount    *int64
	Reason    *string
	Metadata  []byte `gorm:"type:jsonb"`
	CreatedAt time.Time
}

func (PaymentEvent) TableName() string { return "payment_events" }

type AuditLog struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	ActorType  string
	ActorID    *uuid.UUID
	ActorEmail *string
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	StoreID    *uuid.UUID
	Before     []byte  `gorm:"type:jsonb"`
	After      []byte  `gorm:"type:jsonb"`
	IP         *string `gorm:"type:inet"`
	UserAgent  *string
	RequestID  *string
	CreatedAt  time.Time
}

func (AuditLog) TableName() string { return "audit_log" }

type Notification struct {
	ID            uuid.UUID `gorm:"primaryKey"`
	OrderID       *uuid.UUID
	CustomerID    *uuid.UUID
	Channel       string
	Provider      string
	Event         string
	Target        string
	TemplateKey   string
	Language      string
	Body          string
	Status        string
	Attempts      int
	LastError     *string
	NextAttemptAt *time.Time
	SentAt        *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Notification) TableName() string { return "notifications" }

type IdempotencyKey struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	Key          string
	SubjectType  string
	SubjectID    uuid.UUID
	Endpoint     string
	RequestHash  string
	ResponseCode *int
	ResponseBody []byte `gorm:"type:jsonb"`
	CreatedAt    time.Time
}

func (IdempotencyKey) TableName() string { return "idempotency_keys" }

type Favourite struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	CustomerID uuid.UUID
	MenuItemID uuid.UUID
	CreatedAt  time.Time
}

func (Favourite) TableName() string { return "favourites" }
