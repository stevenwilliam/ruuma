package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/adapter/postgres"
	"github.com/stevenwilliam/ruuma/internal/platform/config"
	"github.com/stevenwilliam/ruuma/internal/platform/id"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// runSeed loads demo data: three deliberately different stores, a menu across
// all three cuisines, and one staff account per role.
//
// This is a command, not a migration, so a production deployment never receives
// fake stores (docs/03 §4). It refuses to run against APP_ENV=production.
func runSeed(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	if cfg.App.IsProduction() {
		return fmt.Errorf("seed: refusing to load demo data into production")
	}

	a, err := build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer a.close()

	if err := seedStores(ctx, a.db, log); err != nil {
		return err
	}
	if err := seedMenu(ctx, a.db, log); err != nil {
		return err
	}
	if err := seedStaff(ctx, a.db, log); err != nil {
		return err
	}
	log.Info("seed complete")
	return nil
}

type storeSeed struct {
	code, name, slug, address, city, phone string
	modes                                  []string
	closedWeekdays                         []time.Weekday
	pickupOpen, pickupClose                string
	deliveryOpen, deliveryClose            string
	bank                                   [3]string // bank, account name, account number
}

// The three stores differ on purpose: one open all week, one closed Sunday, one
// closed Saturday and Sunday. That is what makes the closed-weekday rules
// (BR-2.1.4) visible in a demo and meaningful in the tests.
var storeSeeds = []storeSeed{
	{
		code: "RMA-KG", name: "Ruuma Kelapa Gading", slug: "kelapa-gading",
		address: "Jl. Boulevard Raya Blok A No. 1, Kelapa Gading — SEED, replace",
		city:    "Jakarta Utara", phone: "+622145551001",
		modes:      []string{"pickup", "delivery"},
		pickupOpen: "10:00:00", pickupClose: "21:00:00",
		deliveryOpen: "10:00:00", deliveryClose: "20:00:00",
		bank: [3]string{"BCA", "PT Ruuma Eatery — SEED", "1234567890"},
	},
	{
		code: "RMA-SDR", name: "Ruuma Senayan", slug: "senayan",
		address: "Jl. Asia Afrika No. 8, Senayan — SEED, replace",
		city:    "Jakarta Pusat", phone: "+622145551002",
		modes:          []string{"pickup"},
		closedWeekdays: []time.Weekday{time.Sunday},
		pickupOpen:     "10:00:00", pickupClose: "21:00:00",
		bank: [3]string{"BCA", "PT Ruuma Eatery — SEED", "1234567891"},
	},
	{
		code: "RMA-BSD", name: "Ruuma BSD", slug: "bsd",
		address: "Jl. Grand Boulevard No. 12, BSD City — SEED, replace",
		city:    "Tangerang Selatan", phone: "+622145551003",
		modes:          []string{"pickup", "delivery"},
		closedWeekdays: []time.Weekday{time.Saturday, time.Sunday},
		pickupOpen:     "11:00:00", pickupClose: "20:00:00",
		deliveryOpen: "11:00:00", deliveryClose: "19:00:00",
		bank: [3]string{"Mandiri", "PT Ruuma Eatery — SEED", "9876543210"},
	},
}

func seedStores(ctx context.Context, db *gorm.DB, log *slog.Logger) error {
	for _, s := range storeSeeds {
		var existing postgres.Store
		if err := db.WithContext(ctx).Where("code = ?", s.code).First(&existing).Error; err == nil {
			log.Info("store already seeded", "code", s.code)
			continue
		}

		store := postgres.Store{
			ID: uuid.New(), Code: s.code, Name: s.name, Slug: s.slug,
			AddressLine: s.address, City: s.city, Phone: s.phone,
			Timezone: "Asia/Jakarta", IsActive: true,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := db.WithContext(ctx).Create(&store).Error; err != nil {
			return err
		}

		for _, m := range s.modes {
			if err := db.WithContext(ctx).Create(&postgres.StoreFulfilmentMode{
				ID: uuid.New(), StoreID: store.ID, FulfilmentType: m, IsEnabled: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}).Error; err != nil {
				return err
			}
		}

		closed := map[time.Weekday]bool{}
		for _, w := range s.closedWeekdays {
			closed[w] = true
		}

		for wd := time.Sunday; wd <= time.Saturday; wd++ {
			for _, mode := range s.modes {
				row := postgres.StoreHour{
					ID: uuid.New(), StoreID: store.ID, Weekday: int(wd),
					FulfilmentType: mode, BlockIndex: 0, IsClosed: closed[wd],
					CreatedAt: time.Now(), UpdatedAt: time.Now(),
				}
				if !closed[wd] {
					open, close := s.pickupOpen, s.pickupClose
					if mode == "delivery" {
						open, close = s.deliveryOpen, s.deliveryClose
					}
					row.OpensAt, row.ClosesAt = &open, &close
				}
				if err := db.WithContext(ctx).Create(&row).Error; err != nil {
					return err
				}
			}
		}

		if err := db.WithContext(ctx).Create(&postgres.StoreBankAccount{
			ID: uuid.New(), StoreID: store.ID, BankName: s.bank[0],
			AccountName: s.bank[1], AccountNumber: s.bank[2],
			IsPrimary: true, IsActive: true,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
		log.Info("store seeded", "code", s.code)
	}

	// A per-date override on the BSD store, which is otherwise closed at the
	// weekend — the worked example of D18 / BR-2.1.6.
	var bsd postgres.Store
	if err := db.WithContext(ctx).Where("code = ?", "RMA-BSD").First(&bsd).Error; err == nil {
		next := nextWeekday(time.Now(), time.Sunday)
		open, close := "11:00:00", "22:00:00"
		row := postgres.StoreDateOverride{
			ID: uuid.New(), StoreID: bsd.ID, BusinessDate: next,
			FulfilmentType: "pickup", BlockIndex: 0,
			OpensAt: &open, ClosesAt: &close,
			Reason:    strp("Special Sunday opening — seeded example of a per-date override"),
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		_ = db.WithContext(ctx).Exec(`
			INSERT INTO store_date_overrides
				(id, store_id, business_date, fulfilment_type, block_index, is_closed,
				 opens_at, closes_at, reason, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,false,$6,$7,$8, now(), now())
			ON CONFLICT DO NOTHING`,
			row.ID, row.StoreID, row.BusinessDate, row.FulfilmentType, row.BlockIndex,
			row.OpensAt, row.ClosesAt, row.Reason).Error
	}
	return nil
}

type itemSeed struct {
	sku, nameID, nameEN, descID, descEN string
	price                               int64
	kitchenUnits, prep, minLead, spice  int
	halal, vegetarian                   bool
	pork, alcohol, nuts                 bool
}

type categorySeed struct {
	nameID, nameEN, slug, cuisine string
	items                         []itemSeed
}

// A menu that actually spans the three cuisines the group serves, with the
// awkward cases represented: a dish needing four hours' notice, a pork dish, a
// vegetarian dish, and drinks that weigh almost nothing in the kitchen.
var menuSeeds = []categorySeed{
	{
		nameID: "Indonesia", nameEN: "Indonesian", slug: "indonesian", cuisine: "indonesian",
		items: []itemSeed{
			{sku: "IDN-001", nameID: "Nasi Goreng Kampung", nameEN: "Village Fried Rice",
				descID: "Nasi goreng dengan teri, telur, dan sambal terasi",
				descEN: "Fried rice with anchovies, egg and shrimp-paste sambal",
				price:  48000, kitchenUnits: 1, prep: 12, spice: 2, halal: true},
			{sku: "IDN-002", nameID: "Ayam Bakar Madu", nameEN: "Honey Grilled Chicken",
				descID: "Ayam bakar bumbu madu, lalapan, sambal",
				descEN: "Honey-glazed grilled chicken with fresh vegetables and sambal",
				price:  62000, kitchenUnits: 2, prep: 20, spice: 1, halal: true},
			{sku: "IDN-003", nameID: "Rendang Daging", nameEN: "Beef Rendang",
				descID: "Rendang sapi dimasak perlahan dengan santan dan rempah",
				descEN: "Slow-cooked beef in coconut milk and spices",
				price:  78000, kitchenUnits: 2, prep: 15, spice: 2, halal: true},
			{sku: "IDN-004", nameID: "Gado-Gado", nameEN: "Gado-Gado",
				descID: "Sayuran rebus dengan saus kacang", descEN: "Steamed vegetables in peanut sauce",
				price: 42000, kitchenUnits: 1, prep: 10, halal: true, vegetarian: true, nuts: true},
			{sku: "IDN-005", nameID: "Sate Ayam (10 tusuk)", nameEN: "Chicken Satay (10 skewers)",
				descID: "Sate ayam dengan bumbu kacang dan lontong",
				descEN: "Chicken satay with peanut sauce and rice cake",
				price:  55000, kitchenUnits: 2, prep: 18, spice: 1, halal: true, nuts: true},
			{sku: "IDN-006", nameID: "Soto Betawi", nameEN: "Betawi Beef Soup",
				descID: "Soto daging kuah santan khas Betawi",
				descEN: "Jakarta-style beef soup in coconut broth",
				price:  58000, kitchenUnits: 2, prep: 14, spice: 1, halal: true},
		},
	},
	{
		nameID: "Tionghoa", nameEN: "Chinese", slug: "chinese", cuisine: "chinese",
		items: []itemSeed{
			{sku: "CHN-001", nameID: "Kwetiau Sapi", nameEN: "Beef Flat Noodles",
				descID: "Kwetiau goreng dengan irisan sapi dan sawi",
				descEN: "Wok-fried flat noodles with sliced beef and greens",
				price:  56000, kitchenUnits: 1, prep: 12, spice: 1, halal: true},
			{sku: "CHN-002", nameID: "Ayam Kung Pao", nameEN: "Kung Pao Chicken",
				descID: "Ayam pedas dengan kacang mete dan cabai kering",
				descEN: "Spicy chicken with cashews and dried chilli",
				price:  64000, kitchenUnits: 2, prep: 15, spice: 3, halal: true, nuts: true},
			{sku: "CHN-003", nameID: "Sapo Tahu", nameEN: "Claypot Tofu",
				descID: "Tahu jepang, jamur, dan sayuran dalam sapo",
				descEN: "Japanese tofu, mushrooms and vegetables in a claypot",
				price:  52000, kitchenUnits: 1, prep: 14, halal: true, vegetarian: true},
			{sku: "CHN-004", nameID: "Dimsum Pilihan (6 pcs)", nameEN: "Assorted Dim Sum (6 pcs)",
				descID: "Enam potong dimsum pilihan chef", descEN: "Six pieces of the chef's dim sum selection",
				price: 48000, kitchenUnits: 1, prep: 10, halal: true},
			{sku: "CHN-005", nameID: "Bebek Peking Utuh", nameEN: "Whole Peking Duck",
				descID: "Bebek peking utuh — pesan minimal 4 jam sebelumnya",
				descEN: "Whole Peking duck — order at least four hours ahead",
				price:  385000, kitchenUnits: 8, prep: 45, minLead: 240, halal: true},
			{sku: "CHN-006", nameID: "Babi Kecap", nameEN: "Braised Pork in Sweet Soy",
				descID: "Babi masak kecap manis", descEN: "Pork braised in sweet soy sauce",
				price: 72000, kitchenUnits: 2, prep: 16, pork: true},
		},
	},
	{
		nameID: "Barat", nameEN: "Western", slug: "western", cuisine: "western",
		items: []itemSeed{
			{sku: "WST-001", nameID: "Steak Ayam Panggang", nameEN: "Grilled Chicken Steak",
				descID: "Dada ayam panggang, kentang, saus lada hitam",
				descEN: "Grilled chicken breast, potatoes, black pepper sauce",
				price:  75000, kitchenUnits: 2, prep: 20, halal: true},
			{sku: "WST-002", nameID: "Fish & Chips", nameEN: "Fish & Chips",
				descID: "Dori goreng tepung dengan kentang dan saus tartar",
				descEN: "Battered dory with fries and tartar sauce",
				price:  68000, kitchenUnits: 2, prep: 16, halal: true},
			{sku: "WST-003", nameID: "Spaghetti Carbonara", nameEN: "Spaghetti Carbonara",
				descID: "Spaghetti krim dengan smoked beef", descEN: "Cream spaghetti with smoked beef",
				price: 62000, kitchenUnits: 1, prep: 14, halal: true},
			{sku: "WST-004", nameID: "Caesar Salad", nameEN: "Caesar Salad",
				descID: "Romaine, keju parmesan, crouton, dressing caesar",
				descEN: "Romaine, parmesan, croutons, caesar dressing",
				price:  52000, kitchenUnits: 1, prep: 8, halal: true, vegetarian: true},
			{sku: "WST-005", nameID: "Beef Burger", nameEN: "Beef Burger",
				descID: "Patty sapi 150g, keju, kentang goreng",
				descEN: "150g beef patty, cheese, fries",
				price:  72000, kitchenUnits: 2, prep: 15, halal: true},
		},
	},
	{
		nameID: "Minuman", nameEN: "Drinks", slug: "drinks", cuisine: "other",
		items: []itemSeed{
			{sku: "DRK-001", nameID: "Es Teh Manis", nameEN: "Iced Sweet Tea",
				price: 12000, kitchenUnits: 0, prep: 3, halal: true, vegetarian: true},
			{sku: "DRK-002", nameID: "Es Jeruk", nameEN: "Iced Orange",
				price: 18000, kitchenUnits: 0, prep: 4, halal: true, vegetarian: true},
			{sku: "DRK-003", nameID: "Kopi Susu Gula Aren", nameEN: "Palm Sugar Latte",
				price: 28000, kitchenUnits: 0, prep: 5, halal: true, vegetarian: true},
			{sku: "DRK-004", nameID: "Air Mineral", nameEN: "Mineral Water",
				price: 8000, kitchenUnits: 0, prep: 1, halal: true, vegetarian: true},
		},
	},
}

func seedMenu(ctx context.Context, db *gorm.DB, log *slog.Logger) error {
	for order, cat := range menuSeeds {
		var category postgres.Category
		err := db.WithContext(ctx).Where("slug = ?", cat.slug).First(&category).Error
		if err != nil {
			category = postgres.Category{
				ID: uuid.New(), NameID: cat.nameID, NameEN: cat.nameEN, Slug: cat.slug,
				Cuisine: cat.cuisine, SortOrder: order, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			if err := db.WithContext(ctx).Create(&category).Error; err != nil {
				return err
			}
		}

		for i, it := range cat.items {
			var existing postgres.MenuItem
			if err := db.WithContext(ctx).Where("sku = ?", it.sku).First(&existing).Error; err == nil {
				continue
			}
			item := postgres.MenuItem{
				ID: uuid.New(), CategoryID: category.ID, SKU: it.sku,
				NameID: it.nameID, NameEN: it.nameEN,
				DescriptionID: strp(it.descID), DescriptionEN: strp(it.descEN),
				BasePrice: it.price, KitchenUnits: maxInt(it.kitchenUnits, 1),
				PrepMinutes: it.prep, MinLeadMinutes: it.minLead, SpiceLevel: it.spice,
				IsHalal: it.halal, IsVegetarian: it.vegetarian, ContainsPork: it.pork,
				ContainsAlcohol: it.alcohol, ContainsNuts: it.nuts,
				IsActive: true, SortOrder: i,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			if err := db.WithContext(ctx).Create(&item).Error; err != nil {
				return err
			}

			// Rice and spice are required single choices; add-ons are optional
			// and capped at two (BR-2.2.5).
			if cat.cuisine != "other" {
				riceGroup := postgres.OptionGroup{
					ID: uuid.New(), MenuItemID: item.ID, NameID: "Nasi", NameEN: "Rice",
					Selection: "single", IsRequired: true, MinSelect: 1, MaxSelect: 1, SortOrder: 0,
					CreatedAt: time.Now(), UpdatedAt: time.Now(),
				}
				if err := db.WithContext(ctx).Create(&riceGroup).Error; err != nil {
					return err
				}
				for j, c := range []struct {
					id, en string
					delta  int64
				}{
					{"Nasi Putih", "White Rice", 0},
					{"Nasi Merah", "Brown Rice", 5000},
					{"Tanpa Nasi", "No Rice", -5000},
				} {
					if err := db.WithContext(ctx).Create(&postgres.OptionChoice{
						ID: uuid.New(), OptionGroupID: riceGroup.ID, NameID: c.id, NameEN: c.en,
						PriceDelta: c.delta, IsAvailable: true, SortOrder: j,
						CreatedAt: time.Now(), UpdatedAt: time.Now(),
					}).Error; err != nil {
						return err
					}
				}

				addonGroup := postgres.OptionGroup{
					ID: uuid.New(), MenuItemID: item.ID, NameID: "Tambahan", NameEN: "Add-ons",
					Selection: "multi", IsRequired: false, MinSelect: 0, MaxSelect: 2, SortOrder: 1,
					CreatedAt: time.Now(), UpdatedAt: time.Now(),
				}
				if err := db.WithContext(ctx).Create(&addonGroup).Error; err != nil {
					return err
				}
				for j, c := range []struct {
					id, en string
					delta  int64
					units  int
				}{
					{"Telur Mata Sapi", "Fried Egg", 8000, 1},
					{"Sambal Ekstra", "Extra Sambal", 3000, 0},
					{"Kerupuk", "Prawn Crackers", 5000, 0},
				} {
					if err := db.WithContext(ctx).Create(&postgres.OptionChoice{
						ID: uuid.New(), OptionGroupID: addonGroup.ID, NameID: c.id, NameEN: c.en,
						PriceDelta: c.delta, KitchenUnits: c.units, IsAvailable: true, SortOrder: j,
						CreatedAt: time.Now(), UpdatedAt: time.Now(),
					}).Error; err != nil {
						return err
					}
				}
			}
		}
		log.Info("menu category seeded", "slug", cat.slug, "items", len(cat.items))
	}
	return nil
}

type staffSeed struct {
	email, name, role string
	groupScope        bool
	storeCodes        []string
}

// One account per role, each scoped to a different store — which is exactly
// what makes the cross-store tests meaningful (docs/07 §4).
var staffSeeds = []staffSeed{
	{email: "owner@ruuma.id", name: "Owner", role: "owner"},
	{email: "admin@ruuma.id", name: "Admin", role: "admin"},
	{email: "finance@ruuma.id", name: "Finance (group)", role: "finance", groupScope: true},
	{email: "finance.kg@ruuma.id", name: "Finance Kelapa Gading", role: "finance", storeCodes: []string{"RMA-KG"}},
	{email: "manager.kg@ruuma.id", name: "Manager Kelapa Gading", role: "store_manager", storeCodes: []string{"RMA-KG"}},
	{email: "manager.bsd@ruuma.id", name: "Manager BSD", role: "store_manager", storeCodes: []string{"RMA-BSD"}},
	{email: "kitchen.kg@ruuma.id", name: "Kitchen Kelapa Gading", role: "kitchen", storeCodes: []string{"RMA-KG"}},
	{email: "kitchen.sdr@ruuma.id", name: "Kitchen Senayan", role: "kitchen", storeCodes: []string{"RMA-SDR"}},
	{email: "counter.kg@ruuma.id", name: "Counter Kelapa Gading", role: "counter", storeCodes: []string{"RMA-KG"}},
}

// seedStaffPassword returns the password the demo accounts get. It is read
// from SEED_PASSWORD when set and otherwise generated fresh, so no credential
// is ever compiled into the binary and two developers never share one
// (docs/12, A05).
func seedStaffPassword() (string, error) {
	if v := os.Getenv("SEED_PASSWORD"); v != "" {
		return v, nil
	}
	generated, err := id.Token(20)
	if err != nil {
		return "", err
	}
	return "ruuma-" + generated, nil
}

func seedStaff(ctx context.Context, db *gorm.DB, log *slog.Logger) error {
	seedPassword, err := seedStaffPassword()
	if err != nil {
		return err
	}
	hash, err := security.HashPassword(seedPassword)
	if err != nil {
		return err
	}

	for _, s := range staffSeeds {
		var existing postgres.User
		if err := db.WithContext(ctx).Where("email = ?", s.email).First(&existing).Error; err == nil {
			continue
		}
		user := postgres.User{
			ID: uuid.New(), Email: s.email, PasswordHash: hash, FullName: s.name,
			Role: s.role, IsGroupScope: s.groupScope, IsActive: true,
			MustChangePassword: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := db.WithContext(ctx).Create(&user).Error; err != nil {
			return err
		}

		for _, code := range s.storeCodes {
			var store postgres.Store
			if err := db.WithContext(ctx).Where("code = ?", code).First(&store).Error; err != nil {
				continue
			}
			if err := db.WithContext(ctx).Create(&postgres.StaffStoreAssignment{
				ID: uuid.New(), UserID: user.ID, StoreID: store.ID, CreatedAt: time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		log.Info("staff seeded", "email", s.email, "role", s.role)
	}

	log.Info("seed staff password (development only)", "password", seedPassword)
	return nil
}

func nextWeekday(from time.Time, target time.Weekday) time.Time {
	d := from
	for i := 0; i < 8; i++ {
		d = d.AddDate(0, 0, 1)
		if d.Weekday() == target {
			break
		}
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func strp(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
