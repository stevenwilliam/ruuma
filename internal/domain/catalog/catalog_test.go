package catalog

import (
	"errors"
	"testing"
	"time"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
)

var jakarta = func() *time.Location {
	l, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic(err)
	}
	return l
}()

func activeItem() Item {
	return Item{
		ID: "item-1", CategoryID: "cat-1", CategoryActive: true, IsActive: true,
		BasePrice: 65000, KitchenUnits: 1, PrepMinutes: 12,
	}
}

func boolPtr(b bool) *bool { return &b }

// BR-2.2.1: the store override wins for price and availability; absence falls
// back to the group default.
func TestStoreOverrideResolution_BR_2_2_1(t *testing.T) {
	item := activeItem()

	if got := EffectivePrice(item, nil); got != 65000 {
		t.Fatalf("no override: price %d, want 65000", got)
	}
	override := money.Rupiah(72000)
	if got := EffectivePrice(item, &StoreOverride{PriceOverride: &override}); got != 72000 {
		t.Fatalf("with override: price %d, want 72000", got)
	}
	// An override that only touches availability leaves the price alone.
	if got := EffectivePrice(item, &StoreOverride{IsAvailable: boolPtr(false)}); got != 65000 {
		t.Fatalf("availability-only override changed the price to %d", got)
	}

	now := time.Now()
	if got := Resolve(Query{Item: item, Override: &StoreOverride{IsAvailable: boolPtr(false)}, Now: now}); got != StoreUnavailable {
		t.Fatalf("store marked unavailable: got %q, want STORE_UNAVAILABLE", got)
	}
	if got := Resolve(Query{Item: item, Now: now}); got != Available {
		t.Fatalf("plain item: got %q, want available", got)
	}
}

// BR-2.2.2: the ordered gates — inactive item, inactive category, store
// override, 86, stock, rules.
func TestResolveGates_BR_2_2_2(t *testing.T) {
	now := time.Date(2026, 8, 3, 5, 0, 0, 0, time.UTC) // Monday 12:00 Jakarta

	inactive := activeItem()
	inactive.IsActive = false
	if got := Resolve(Query{Item: inactive, Now: now}); got != ItemInactive {
		t.Fatalf("inactive item: got %q", got)
	}

	catOff := activeItem()
	catOff.CategoryActive = false
	if got := Resolve(Query{Item: catOff, Now: now}); got != CategoryInactive {
		t.Fatalf("inactive category: got %q", got)
	}
}

// BR-2.2.3: an active 86 hides the item only for its window.
func TestEightySixWindow_BR_2_2_3(t *testing.T) {
	item := activeItem()
	start := time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC)
	end := start.Add(6 * time.Hour)

	e := EightySix{StartsAt: start, EndsAt: &end}

	if got := Resolve(Query{Item: item, EightySixs: []EightySix{e}, Now: start.Add(-time.Minute)}); got != Available {
		t.Fatalf("before the 86 window: got %q, want available", got)
	}
	if got := Resolve(Query{Item: item, EightySixs: []EightySix{e}, Now: start.Add(time.Hour)}); got != EightySixed {
		t.Fatalf("inside the 86 window: got %q, want OUT_OF_STOCK_86", got)
	}
	if got := Resolve(Query{Item: item, EightySixs: []EightySix{e}, Now: end.Add(time.Minute)}); got != Available {
		t.Fatalf("after the 86 window: got %q, want available", got)
	}

	// An open-ended 86 stays active until lifted.
	open := EightySix{StartsAt: start}
	if got := Resolve(Query{Item: item, EightySixs: []EightySix{open}, Now: start.Add(300 * time.Hour)}); got != EightySixed {
		t.Fatalf("open-ended 86: got %q, want OUT_OF_STOCK_86", got)
	}
}

// BR-2.2.4: the daily countdown blocks at zero, and respects the quantity asked
// for rather than just "is there any left".
func TestDailyStock_BR_2_2_4(t *testing.T) {
	item := activeItem()
	now := time.Now()

	stock := DailyStock{Total: 10, Used: 8}
	if got := Resolve(Query{Item: item, Stock: &stock, Qty: 2, Now: now}); got != Available {
		t.Fatalf("2 of 2 remaining: got %q, want available", got)
	}
	if got := Resolve(Query{Item: item, Stock: &stock, Qty: 3, Now: now}); got != SoldOutToday {
		t.Fatalf("3 of 2 remaining: got %q, want SOLD_OUT_TODAY", got)
	}

	sold := DailyStock{Total: 10, Used: 10}
	if got := Resolve(Query{Item: item, Stock: &sold, Qty: 1, Now: now}); got != SoldOutToday {
		t.Fatalf("nothing left: got %q, want SOLD_OUT_TODAY", got)
	}
	if got := sold.Remaining(); got != 0 {
		t.Fatalf("Remaining() = %d, want 0", got)
	}
}

// BR-2.2.7: a weekend-only dish is excluded on a Monday slot and admitted on a
// Saturday slot.
func TestAvailabilityRules_BR_2_2_7(t *testing.T) {
	item := activeItem()
	weekendOnly := AvailabilityRule{WeekdayMask: (1 << uint(time.Saturday)) | (1 << uint(time.Sunday))}

	monday := time.Date(2026, 8, 3, 12, 0, 0, 0, jakarta)
	saturday := time.Date(2026, 8, 1, 12, 0, 0, 0, jakarta)

	if got := Resolve(Query{Item: item, Rules: []AvailabilityRule{weekendOnly}, SlotStartLocal: &monday, Now: monday}); got != RuleExcluded {
		t.Fatalf("weekend dish on Monday: got %q, want NOT_AVAILABLE_AT_THIS_TIME", got)
	}
	if got := Resolve(Query{Item: item, Rules: []AvailabilityRule{weekendOnly}, SlotStartLocal: &saturday, Now: saturday}); got != Available {
		t.Fatalf("weekend dish on Saturday: got %q, want available", got)
	}

	// A lunch-only rule.
	from := schedule.TimeOfDay{Hour: 10}
	to := schedule.TimeOfDay{Hour: 14}
	lunchOnly := AvailabilityRule{WeekdayMask: 127, FromTime: &from, ToTime: &to}

	dinner := time.Date(2026, 8, 3, 18, 0, 0, 0, jakarta)
	if got := Resolve(Query{Item: item, Rules: []AvailabilityRule{lunchOnly}, SlotStartLocal: &dinner, Now: dinner}); got != RuleExcluded {
		t.Fatalf("lunch dish at dinner: got %q, want NOT_AVAILABLE_AT_THIS_TIME", got)
	}
	if got := Resolve(Query{Item: item, Rules: []AvailabilityRule{lunchOnly}, SlotStartLocal: &monday, Now: monday}); got != Available {
		t.Fatalf("lunch dish at noon: got %q, want available", got)
	}
}

func groups() []OptionGroup {
	return []OptionGroup{
		{ // required single choice: rice type
			ID: "g-rice", Selection: Single, IsRequired: true, MinSelect: 1, MaxSelect: 1,
			Choices: []OptionChoice{
				{ID: "c-white", PriceDelta: 0, IsAvailable: true},
				{ID: "c-brown", PriceDelta: 5000, IsAvailable: true},
				{ID: "c-none", PriceDelta: -3000, IsAvailable: true},
				{ID: "c-red", PriceDelta: 6000, IsAvailable: false}, // out of stock
			},
		},
		{ // optional multi: add-ons, at most two
			ID: "g-addon", Selection: Multi, MinSelect: 0, MaxSelect: 2,
			Choices: []OptionChoice{
				{ID: "c-egg", PriceDelta: 8000, KitchenUnits: 1, IsAvailable: true},
				{ID: "c-sambal", PriceDelta: 3000, IsAvailable: true},
				{ID: "c-krupuk", PriceDelta: 4000, IsAvailable: true},
			},
		},
	}
}

// BR-2.2.5 / BR-2.2.6: required groups, min/max bounds, unavailable choices,
// unknown ids, duplicates, and the resulting price delta.
func TestValidateOptions_BR_2_2_5_BR_2_2_6(t *testing.T) {
	res, err := ValidateOptions(groups(), []string{"c-brown", "c-egg", "c-sambal"})
	if err != nil {
		t.Fatalf("valid selection: %v", err)
	}
	if res.Delta != 16000 { // 5000 + 8000 + 3000
		t.Fatalf("delta %d, want 16000", res.Delta)
	}
	if res.KitchenUnits != 1 {
		t.Fatalf("kitchen units %d, want 1", res.KitchenUnits)
	}

	cases := []struct {
		name     string
		selected []string
		want     error
	}{
		{"required group missing", []string{"c-egg"}, ErrRequiredGroupMissing},
		{"two from a single group", []string{"c-white", "c-brown"}, ErrTooManySelected},
		{"over the multi maximum", []string{"c-white", "c-egg", "c-sambal", "c-krupuk"}, ErrTooManySelected},
		{"unavailable choice", []string{"c-red"}, ErrChoiceUnavailable},
		{"unknown choice", []string{"c-white", "c-from-another-dish"}, ErrUnknownChoice},
		{"duplicate choice", []string{"c-white", "c-egg", "c-egg"}, ErrDuplicateChoice},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateOptions(groups(), tc.selected); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}

	// A negative delta is legal (no rice is cheaper) as long as the line stays
	// non-negative — that check lives in pricing.
	res, err = ValidateOptions(groups(), []string{"c-none"})
	if err != nil || res.Delta != -3000 {
		t.Fatalf("negative delta: got %d, %v", res.Delta, err)
	}
}

// BR-2.2.7: the strictest item lead time in a cart is the one that counts.
func TestMaxItemLeadMinutes_BR_2_2_7(t *testing.T) {
	items := []Item{
		{MinLeadMinutes: 0},
		{MinLeadMinutes: 240}, // a whole roast duck needs four hours
		{MinLeadMinutes: 30},
	}
	if got := MaxItemLeadMinutes(items); got != 240 {
		t.Fatalf("got %d, want 240", got)
	}
	if got := MaxItemLeadMinutes(nil); got != 0 {
		t.Fatalf("empty cart: got %d, want 0", got)
	}
}
