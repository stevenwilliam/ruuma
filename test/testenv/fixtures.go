package testenv

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/adapter/postgres"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// Fixtures are the ids a test needs to talk about the world it was given.
//
// Two stores with different staff is not decoration: it is what makes the
// cross-store tests meaningful (docs/07 §4).
type Fixtures struct {
	StoreA    uuid.UUID // open every day, pickup + delivery
	StoreB    uuid.UUID // closed Saturday and Sunday
	BankA     uuid.UUID
	BankB     uuid.UUID
	Category  uuid.UUID
	ItemMain  uuid.UUID // 1 kitchen unit
	ItemBig   uuid.UUID // 8 kitchen units, 240-minute lead time
	ItemDrink uuid.UUID
	RiceGroup uuid.UUID
	RiceWhite uuid.UUID
	RiceBrown uuid.UUID

	Customer      uuid.UUID // verified phone — may order
	CustomerOther uuid.UUID // a second customer, for IDOR tests
	Unverified    uuid.UUID // no verified phone — may not order

	Owner        uuid.UUID
	Admin        uuid.UUID
	FinanceA     uuid.UUID // scoped to store A
	FinanceGroup uuid.UUID // group-wide
	KitchenA     uuid.UUID
	KitchenB     uuid.UUID
	CounterA     uuid.UUID
	ManagerA     uuid.UUID
}

// F holds the fixture ids for the running environment.
var _ = time.Now

func (e *Env) seed(ctx context.Context) {
	f := &Fixtures{}
	db := e.DB
	now := time.Now()

	mustCreate := func(v any) {
		if err := db.WithContext(ctx).Create(v).Error; err != nil {
			e.T.Fatalf("fixture: %v", err)
		}
	}

	// ── Stores ───────────────────────────────────────────────────────────────
	storeA := postgres.Store{
		ID: uuid.New(), Code: "TST-A", Name: "Test Store A", Slug: "test-a",
		AddressLine: "Jl. Test 1", City: "Jakarta", Phone: "+622100000001",
		Timezone: "Asia/Jakarta", IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	storeB := postgres.Store{
		ID: uuid.New(), Code: "TST-B", Name: "Test Store B", Slug: "test-b",
		AddressLine: "Jl. Test 2", City: "Tangerang", Phone: "+622100000002",
		Timezone: "Asia/Jakarta", IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	mustCreate(&storeA)
	mustCreate(&storeB)
	f.StoreA, f.StoreB = storeA.ID, storeB.ID

	for _, s := range []postgres.Store{storeA, storeB} {
		for _, mode := range []string{"pickup", "delivery"} {
			mustCreate(&postgres.StoreFulfilmentMode{
				ID: uuid.New(), StoreID: s.ID, FulfilmentType: mode, IsEnabled: true,
				CreatedAt: now, UpdatedAt: now,
			})
		}
	}

	open, close := "10:00:00", "21:00:00"
	for wd := 0; wd <= 6; wd++ {
		for _, mode := range []string{"pickup", "delivery"} {
			mustCreate(&postgres.StoreHour{
				ID: uuid.New(), StoreID: storeA.ID, Weekday: wd, FulfilmentType: mode,
				OpensAt: &open, ClosesAt: &close, CreatedAt: now, UpdatedAt: now,
			})
			// Store B closes at the weekend — the awkward store (BR-2.1.4).
			closed := wd == 0 || wd == 6
			row := postgres.StoreHour{
				ID: uuid.New(), StoreID: storeB.ID, Weekday: wd, FulfilmentType: mode,
				IsClosed: closed, CreatedAt: now, UpdatedAt: now,
			}
			if !closed {
				row.OpensAt, row.ClosesAt = &open, &close
			}
			mustCreate(&row)
		}
	}

	bankA := postgres.StoreBankAccount{
		ID: uuid.New(), StoreID: storeA.ID, BankName: "BCA", AccountName: "PT Test",
		AccountNumber: "1111111111", IsPrimary: true, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	bankB := postgres.StoreBankAccount{
		ID: uuid.New(), StoreID: storeB.ID, BankName: "BCA", AccountName: "PT Test",
		AccountNumber: "2222222222", IsPrimary: true, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	mustCreate(&bankA)
	mustCreate(&bankB)
	f.BankA, f.BankB = bankA.ID, bankB.ID

	// ── Menu ─────────────────────────────────────────────────────────────────
	category := postgres.Category{
		ID: uuid.New(), NameID: "Indonesia", NameEN: "Indonesian", Slug: "indonesian",
		Cuisine: "indonesian", IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	mustCreate(&category)
	f.Category = category.ID

	main := postgres.MenuItem{
		ID: uuid.New(), CategoryID: category.ID, SKU: "TST-001",
		NameID: "Nasi Goreng Uji", NameEN: "Test Fried Rice",
		BasePrice: 50000, KitchenUnits: 1, PrepMinutes: 10, IsHalal: true,
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	big := postgres.MenuItem{
		ID: uuid.New(), CategoryID: category.ID, SKU: "TST-002",
		NameID: "Bebek Uji", NameEN: "Test Duck",
		BasePrice: 300000, KitchenUnits: 8, PrepMinutes: 45, MinLeadMinutes: 240,
		IsHalal: true, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	drink := postgres.MenuItem{
		ID: uuid.New(), CategoryID: category.ID, SKU: "TST-003",
		NameID: "Es Teh Uji", NameEN: "Test Iced Tea",
		BasePrice: 10000, KitchenUnits: 1, PrepMinutes: 2, IsHalal: true,
		IsVegetarian: true, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	mustCreate(&main)
	mustCreate(&big)
	mustCreate(&drink)
	f.ItemMain, f.ItemBig, f.ItemDrink = main.ID, big.ID, drink.ID

	rice := postgres.OptionGroup{
		ID: uuid.New(), MenuItemID: main.ID, NameID: "Nasi", NameEN: "Rice",
		Selection: "single", IsRequired: true, MinSelect: 1, MaxSelect: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	mustCreate(&rice)
	white := postgres.OptionChoice{
		ID: uuid.New(), OptionGroupID: rice.ID, NameID: "Putih", NameEN: "White",
		PriceDelta: 0, IsAvailable: true, CreatedAt: now, UpdatedAt: now,
	}
	brown := postgres.OptionChoice{
		ID: uuid.New(), OptionGroupID: rice.ID, NameID: "Merah", NameEN: "Brown",
		PriceDelta: 5000, IsAvailable: true, CreatedAt: now, UpdatedAt: now,
	}
	mustCreate(&white)
	mustCreate(&brown)
	f.RiceGroup, f.RiceWhite, f.RiceBrown = rice.ID, white.ID, brown.ID

	// ── Customers ────────────────────────────────────────────────────────────
	verifiedAt := now
	hash, err := security.HashPassword("customer-password-2026")
	if err != nil {
		e.T.Fatalf("hash: %v", err)
	}

	newCustomer := func(name, email, phone string, verified bool) uuid.UUID {
		c := postgres.Customer{
			ID: uuid.New(), FullName: name, PreferredLanguage: "id", IsActive: true,
			CreatedAt: now, UpdatedAt: now,
		}
		c.Email, c.Phone = &email, &phone
		c.PasswordHash = &hash
		c.EmailVerifiedAt = &verifiedAt
		if verified {
			c.PhoneVerifiedAt = &verifiedAt
		}
		mustCreate(&c)
		return c.ID
	}
	f.Customer = newCustomer("Budi Test", "budi@test.local", "+628111111111", true)
	f.CustomerOther = newCustomer("Sari Test", "sari@test.local", "+628222222222", true)
	f.Unverified = newCustomer("Tanpa Verifikasi", "no@test.local", "+628333333333", false)

	// ── Staff ────────────────────────────────────────────────────────────────
	staffHash, err := security.HashPassword("staff-password-2026")
	if err != nil {
		e.T.Fatalf("hash: %v", err)
	}
	newStaff := func(email, role string, groupScope bool, stores ...uuid.UUID) uuid.UUID {
		u := postgres.User{
			ID: uuid.New(), Email: email, PasswordHash: staffHash, FullName: email,
			Role: role, IsGroupScope: groupScope, IsActive: true,
			CreatedAt: now, UpdatedAt: now,
		}
		mustCreate(&u)
		for _, s := range stores {
			mustCreate(&postgres.StaffStoreAssignment{
				ID: uuid.New(), UserID: u.ID, StoreID: s, CreatedAt: now,
			})
		}
		return u.ID
	}
	f.Owner = newStaff("owner@test.local", "owner", false)
	f.Admin = newStaff("admin@test.local", "admin", false)
	f.FinanceA = newStaff("finance.a@test.local", "finance", false, storeA.ID)
	f.FinanceGroup = newStaff("finance.group@test.local", "finance", true)
	f.KitchenA = newStaff("kitchen.a@test.local", "kitchen", false, storeA.ID)
	f.KitchenB = newStaff("kitchen.b@test.local", "kitchen", false, storeB.ID)
	f.CounterA = newStaff("counter.a@test.local", "counter", false, storeA.ID)
	f.ManagerA = newStaff("manager.a@test.local", "store_manager", false, storeA.ID)

	e.Fixtures = f
}

// Token mints an access token for a fixture subject, so a test does not have to
// log in through the API when it is testing something else.
func (e *Env) Token(subject security.SubjectType, id uuid.UUID, role security.Role) string {
	e.T.Helper()
	token, _, err := e.Signer.Issue(subject, id, role)
	if err != nil {
		e.T.Fatalf("issue token: %v", err)
	}
	return token
}

// CustomerToken is shorthand for a customer session.
func (e *Env) CustomerToken(id uuid.UUID) string {
	return e.Token(security.SubjectCustomer, id, security.RoleCustomer)
}

// StaffToken is shorthand for a staff session.
func (e *Env) StaffToken(id uuid.UUID, role security.Role) string {
	return e.Token(security.SubjectStaff, id, role)
}

// ── Helpers the suites build scenarios from ──────────────────────────────────

// MakeSlot materialises one slot at a given hour, with explicit capacity, so a
// test can create the exact scarcity it wants to prove something about.
func (e *Env) MakeSlot(storeID uuid.UUID, mode string, hour, minute, maxOrders, maxUnits int) uuid.UUID {
	e.T.Helper()

	loc := time.FixedZone("WIB", 7*3600)
	local := e.Now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	businessDate := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	slot := postgres.Slot{
		ID: uuid.New(), StoreID: storeID, BusinessDate: businessDate,
		FulfilmentType: mode, StartsAt: start.UTC(), EndsAt: start.Add(30 * time.Minute).UTC(),
		MaxOrders: maxOrders, MaxKitchenUnits: maxUnits,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := e.DB.Create(&slot).Error; err != nil {
		e.T.Fatalf("make slot: %v", err)
	}
	return slot.ID
}

// SlotCounters reads a slot's reserved and maximum order counts.
func (e *Env) SlotCounters(slotID uuid.UUID) (reserved, max int) {
	e.T.Helper()
	var slot postgres.Slot
	if err := e.DB.First(&slot, "id = ?", slotID).Error; err != nil {
		e.T.Fatalf("read slot: %v", err)
	}
	return slot.ReservedOrders, slot.MaxOrders
}

// MakeCustomers creates n order-ready customers (verified phone, BR-2.7.4).
func (e *Env) MakeCustomers(n int) []uuid.UUID {
	e.T.Helper()
	verified := time.Now()
	out := make([]uuid.UUID, 0, n)

	for i := 0; i < n; i++ {
		email := fmt.Sprintf("racer-%s@test.local", uuid.NewString()[:8])
		phone := fmt.Sprintf("+62899%08d", i+randomSuffix())
		c := postgres.Customer{
			ID: uuid.New(), FullName: "Racer", PreferredLanguage: "id", IsActive: true,
			Email: &email, Phone: &phone,
			EmailVerifiedAt: &verified, PhoneVerifiedAt: &verified,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := e.DB.Create(&c).Error; err != nil {
			e.T.Fatalf("make customer: %v", err)
		}
		out = append(out, c.ID)
	}
	return out
}

var suffix = 10000000

func randomSuffix() int {
	suffix++
	return suffix % 89999999
}

// OrderBody builds a one-line order request for the standard test item.
func (e *Env) OrderBody(storeID, slotID uuid.UUID) map[string]any {
	return e.OrderBodyItem(storeID, slotID, e.Fixtures.ItemMain, 1,
		[]uuid.UUID{e.Fixtures.RiceWhite})
}

// OrderBodyItem builds an order request for a specific item and options.
func (e *Env) OrderBodyItem(storeID, slotID, itemID uuid.UUID, qty int, choices []uuid.UUID) map[string]any {
	line := map[string]any{"menu_item_id": itemID, "qty": qty}
	if len(choices) > 0 {
		line["option_choice_ids"] = choices
	}
	return map[string]any{
		"store_id":        storeID,
		"slot_id":         slotID,
		"fulfilment_type": "pickup",
		"contact_name":    "Budi Test",
		"contact_phone":   "+628111111111",
		"lines":           []map[string]any{line},
	}
}

// AttachProof puts a payment into the finance queue without going through the
// multipart upload, so a test about verification is not a test about files.
func (e *Env) AttachProof(orderID string, customerID uuid.UUID, declared int64) {
	e.T.Helper()
	id := uuid.MustParse(orderID)

	if err := e.Payments.AttachProof(context.Background(), id, customerID,
		"proofs/test/"+uuid.NewString(), declared, "Test Sender"); err != nil {
		e.T.Fatalf("attach proof: %v", err)
	}
}

// PaymentIDForOrder finds an order's payment row.
func (e *Env) PaymentIDForOrder(orderID string) string {
	e.T.Helper()
	var p postgres.Payment
	if err := e.DB.Where("order_id = ?", orderID).First(&p).Error; err != nil {
		e.T.Fatalf("payment for order: %v", err)
	}
	return p.ID.String()
}

// SundayISO returns the next Sunday in the store timezone, for the tests that
// need a day the weekend-closed store is shut (BR-2.1.4).
func (e *Env) SundayISO() string {
	loc := time.FixedZone("WIB", 7*3600)
	d := e.Now.In(loc)
	for i := 0; i < 8; i++ {
		if d.Weekday() == time.Sunday && d.After(e.Now.In(loc)) {
			break
		}
		d = d.AddDate(0, 0, 1)
	}
	return d.Format("2006-01-02")
}
