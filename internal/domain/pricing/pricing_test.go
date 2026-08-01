package pricing

import (
	"testing"
	"time"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
)

var (
	now      = time.Date(2026, 8, 2, 5, 0, 0, 0, time.UTC)
	storeKG  = "store-kg"
	storeBSD = "store-bsd"
	catIndo  = "cat-indonesian"
	catWest  = "cat-western"
)

func line(price, delta money.Rupiah, qty int, cat string) Line {
	return Line{CategoryID: cat, UnitPrice: price, OptionsDelta: delta, Qty: qty, KitchenUnits: 1}
}

// BR-2.5.2: line_total = (unit + options) × qty, integers only.
func TestLineTotal_BR_2_5_2(t *testing.T) {
	cases := []struct {
		name string
		line Line
		want money.Rupiah
	}{
		{"plain", line(65000, 0, 1, catIndo), 65000},
		{"with options", line(65000, 10000, 2, catIndo), 150000},
		{"negative option delta", line(65000, -5000, 3, catIndo), 180000},
		{"qty zero", line(65000, 0, 0, catIndo), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.line.LineTotal()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}

	// BR-2.2.6: an option delta can never push a line below zero.
	if _, err := (Line{UnitPrice: 5000, OptionsDelta: -6000, Qty: 1}).LineTotal(); err != ErrNegativeLine {
		t.Fatalf("negative line: got %v, want ErrNegativeLine", err)
	}
}

// BR-2.5.3 → BR-2.5.7: the full total, computed by hand in the test so the
// expected figures are independent of the implementation.
func TestComputeTotals_BR_2_5_5_BR_2_5_7(t *testing.T) {
	lines := []Line{
		line(65000, 10000, 2, catIndo), // 150.000
		line(45000, 0, 1, catWest),     //  45.000
	}
	cfg := Config{TaxBps: 1000} // PB1 10%, no service charge (D17 defaults)

	got, err := Compute(lines, 0, 0, cfg)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if got.Subtotal != 195000 {
		t.Fatalf("subtotal %d, want 195000", got.Subtotal)
	}
	if got.Tax != 19500 {
		t.Fatalf("tax %d, want 19500", got.Tax)
	}
	if got.Total != 214500 {
		t.Fatalf("total %d, want 214500", got.Total)
	}

	// With a discount and a service charge: 195.000 − 15.000 = 180.000;
	// service 5% = 9.000; tax 10% of 189.000 = 18.900; total 207.900.
	cfg2 := Config{TaxBps: 1000, ServiceChargeBps: 500}
	got2, err := Compute(lines, 15000, 0, cfg2)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if got2.ServiceCharge != 9000 || got2.Tax != 18900 || got2.Total != 207900 {
		t.Fatalf("service %d tax %d total %d; want 9000 18900 207900",
			got2.ServiceCharge, got2.Tax, got2.Total)
	}
}

// BR-2.5.4: a discount may never exceed the subtotal.
func TestDiscountCannotExceedSubtotal_BR_2_5_4(t *testing.T) {
	lines := []Line{line(50000, 0, 1, catIndo)}
	if _, err := Compute(lines, 60000, 0, Config{TaxBps: 1000}); err != ErrDiscountRange {
		t.Fatalf("got %v, want ErrDiscountRange", err)
	}
	// exactly equal is allowed and lands on zero
	got, err := Compute(lines, 50000, 0, Config{TaxBps: 1000})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if got.Total != 0 {
		t.Fatalf("total %d, want 0", got.Total)
	}
}

// D17: tax-inclusive pricing derives the tax instead of adding it, and the
// customer pays the same total either way for the same shelf price.
func TestTaxInclusive_D17(t *testing.T) {
	lines := []Line{line(110000, 0, 1, catIndo)}

	inclusive, err := Compute(lines, 0, 0, Config{TaxBps: 1000, TaxInclusive: true})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if inclusive.Total != 110000 {
		t.Fatalf("inclusive total %d, want 110000 (price unchanged)", inclusive.Total)
	}
	// 110.000 contains 10.000 of PB1 (100.000 × 10%).
	if inclusive.Tax != 10000 {
		t.Fatalf("derived tax %d, want 10000", inclusive.Tax)
	}

	exclusive, err := Compute([]Line{line(100000, 0, 1, catIndo)}, 0, 0, Config{TaxBps: 1000})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if exclusive.Total != inclusive.Total {
		t.Fatalf("exclusive total %d != inclusive total %d", exclusive.Total, inclusive.Total)
	}
}

func TestKitchenUnits_BR_2_3_7(t *testing.T) {
	lines := []Line{
		{Qty: 2, KitchenUnits: 1},
		{Qty: 1, KitchenUnits: 5}, // a banquet dish weighs more than a drink
	}
	if got := KitchenUnits(lines); got != 7 {
		t.Fatalf("kitchen units %d, want 7", got)
	}
}

func basePromo() Promotion {
	return Promotion{
		Code:     "RUUMA10",
		Type:     Percent,
		ValueBps: 1000,
		IsActive: true,
		StartsAt: now.Add(-time.Hour),
		EndsAt:   now.Add(time.Hour),
		StoreIDs: []string{storeKG},
		MinSpend: 100000,
	}
}

// BR-2.5.9: every gate refuses with its own reason.
func TestPromotionGates_BR_2_5_9(t *testing.T) {
	lines := []Line{line(150000, 0, 1, catIndo)}
	ctx := PromoContext{StoreID: storeKG, Lines: lines, Now: now}

	cases := []struct {
		name   string
		mutate func(*Promotion)
		ctx    PromoContext
		want   PromoReason
	}{
		{"valid", func(*Promotion) {}, ctx, PromoOK},
		{"inactive", func(p *Promotion) { p.IsActive = false }, ctx, PromoInactive},
		{"not started", func(p *Promotion) { p.StartsAt = now.Add(time.Hour) }, ctx, PromoNotStarted},
		{"expired", func(p *Promotion) { p.EndsAt = now.Add(-time.Minute) }, ctx, PromoExpired},
		{"wrong store", func(*Promotion) {}, PromoContext{StoreID: storeBSD, Lines: lines, Now: now}, PromoStoreNotEligible},
		{"below minimum spend", func(p *Promotion) { p.MinSpend = 200000 }, ctx, PromoMinSpend},
		{"total cap reached", func(p *Promotion) { p.UsageCapTotal = 5; p.UsedCount = 5 }, ctx, PromoCapReached},
		{"customer cap reached", func(p *Promotion) { p.UsageCapPerCustomer = 1; p.CustomerUsedCount = 1 }, ctx, PromoCustomerCap},
		{"no eligible category", func(p *Promotion) { p.CategoryIDs = []string{catWest} }, ctx, PromoNoEligibleItems},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := basePromo()
			tc.mutate(&p)
			_, reason := EvaluatePromotion(p, tc.ctx)
			if reason != tc.want {
				t.Fatalf("got %q, want %q", reason, tc.want)
			}
		})
	}
}

// D15: a promotion covers several selected stores, and only those.
func TestPromotionMultiStoreScope_D15(t *testing.T) {
	p := basePromo()
	p.StoreIDs = []string{storeKG, storeBSD}
	lines := []Line{line(150000, 0, 1, catIndo)}

	for _, store := range []string{storeKG, storeBSD} {
		if _, r := EvaluatePromotion(p, PromoContext{StoreID: store, Lines: lines, Now: now}); r != PromoOK {
			t.Fatalf("store %s: got %q, want OK", store, r)
		}
	}
	if _, r := EvaluatePromotion(p, PromoContext{StoreID: "store-senayan", Lines: lines, Now: now}); r != PromoStoreNotEligible {
		t.Fatalf("unlisted store: got %q, want PROMO_STORE_NOT_ELIGIBLE", r)
	}
}

// BR-2.5.10: percent caps, fixed caps, and never below zero.
func TestPromotionAmounts_BR_2_5_10(t *testing.T) {
	lines := []Line{line(150000, 0, 1, catIndo)}
	ctx := PromoContext{StoreID: storeKG, Lines: lines, Now: now}

	p := basePromo() // 10% of 150.000 = 15.000
	got, r := EvaluatePromotion(p, ctx)
	if r != PromoOK || got != 15000 {
		t.Fatalf("percent: got %d/%q, want 15000/OK", got, r)
	}

	p.MaxDiscount = 10000
	got, _ = EvaluatePromotion(p, ctx)
	if got != 10000 {
		t.Fatalf("capped percent: got %d, want 10000", got)
	}

	fixed := basePromo()
	fixed.Type = Fixed
	fixed.ValueAmount = 500000 // more than the cart is worth
	got, _ = EvaluatePromotion(fixed, ctx)
	if got != 150000 {
		t.Fatalf("fixed over subtotal: got %d, want 150000 (capped at eligible)", got)
	}

	// The capped discount fed back into Compute must land on exactly zero.
	totals, err := Compute(lines, got, 0, Config{TaxBps: 1000})
	if err != nil || totals.Total != 0 {
		t.Fatalf("total %d err %v; want 0, nil", totals.Total, err)
	}
}

// A category-restricted promotion discounts only the eligible lines, while the
// minimum spend is judged on the whole cart (BR-2.5.9).
func TestPromotionCategoryRestriction_BR_2_5_9(t *testing.T) {
	p := basePromo()
	p.CategoryIDs = []string{catIndo}
	p.MinSpend = 100000

	lines := []Line{
		line(60000, 0, 1, catIndo), // eligible
		line(50000, 0, 1, catWest), // not eligible
	}
	got, r := EvaluatePromotion(p, PromoContext{StoreID: storeKG, Lines: lines, Now: now})
	if r != PromoOK {
		t.Fatalf("reason %q, want OK (cart total 110.000 meets min spend)", r)
	}
	if got != 6000 { // 10% of the eligible 60.000 only
		t.Fatalf("discount %d, want 6000", got)
	}
}
