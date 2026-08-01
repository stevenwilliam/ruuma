// Package pricing computes order totals and evaluates promotions.
//
// Every amount is integer rupiah (BR-1.1.1/2) and the server is the only
// authority on a total — the client's figure is compared, never trusted
// (BR-2.5.13).
package pricing

import (
	"errors"
	"time"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
)

// Line is one cart line after option selection. Prices here are snapshots
// resolved from master data, never sent by the client (BR-2.5.1).
type Line struct {
	MenuItemID   string
	CategoryID   string
	UnitPrice    money.Rupiah // store-resolved price (BR-2.2.1)
	OptionsDelta money.Rupiah // sum of the selected choices' deltas
	Qty          int
	KitchenUnits int // per unit; the caller multiplies by Qty for capacity
}

// LineTotal is (unit + options) × qty (BR-2.5.2).
func (l Line) LineTotal() (money.Rupiah, error) {
	unit, err := money.Add(l.UnitPrice, l.OptionsDelta)
	if err != nil {
		return 0, err
	}
	if unit < 0 {
		// An option delta may be negative, but never enough to make a line
		// negative (BR-2.2.6).
		return 0, ErrNegativeLine
	}
	return money.Mul(unit, l.Qty)
}

// Config carries the rates in force for this store, each resolved store → group
// default (BR-1.4.4, BR-2.5.5).
type Config struct {
	TaxBps           money.Bps
	ServiceChargeBps money.Bps
	TaxInclusive     bool // D17: when true, prices already include tax
}

// Totals is the money breakdown of an order (BR-2.5.7).
type Totals struct {
	Subtotal      money.Rupiah
	Discount      money.Rupiah
	ServiceCharge money.Rupiah
	Tax           money.Rupiah
	DeliveryFee   money.Rupiah
	Total         money.Rupiah
}

var (
	ErrNegativeLine  = errors.New("pricing: line total would be negative")
	ErrDiscountRange = errors.New("pricing: discount exceeds subtotal")
)

// Compute builds the totals for a set of lines (BR-2.5.3 → BR-2.5.7).
//
// Order of operations, which is itself the rule: subtotal, then discount, then
// service charge on the discounted amount, then tax on the discounted amount
// plus service charge, then the delivery fee (zero in phase 1).
func Compute(lines []Line, discount money.Rupiah, deliveryFee money.Rupiah, cfg Config) (Totals, error) {
	var t Totals

	for _, l := range lines {
		lt, err := l.LineTotal()
		if err != nil {
			return t, err
		}
		sum, err := money.Add(t.Subtotal, lt)
		if err != nil {
			return t, err
		}
		t.Subtotal = sum
	}

	if discount < 0 {
		discount = 0
	}
	if discount > t.Subtotal {
		return t, ErrDiscountRange // BR-2.5.4
	}
	t.Discount = discount
	t.DeliveryFee = money.ClampNonNegative(deliveryFee)

	discounted, err := money.Sub(t.Subtotal, t.Discount)
	if err != nil {
		return t, err
	}

	t.ServiceCharge = money.ApplyRate(discounted, cfg.ServiceChargeBps)

	base, err := money.Add(discounted, t.ServiceCharge)
	if err != nil {
		return t, err
	}

	if cfg.TaxInclusive {
		// Prices already contain the tax: derive the tax component rather than
		// adding to the price (D17). tax = base − base/(1+rate), computed in
		// integers as base × bps / (10000 + bps), half-up.
		if cfg.TaxBps > 0 {
			denominator := int64(money.FullBps) + int64(cfg.TaxBps)
			t.Tax = money.Rupiah((int64(base)*int64(cfg.TaxBps) + denominator/2) / denominator)
		}
		t.Total, err = money.Add(base, t.DeliveryFee)
		if err != nil {
			return t, err
		}
		return t, nil
	}

	t.Tax = money.ApplyRate(base, cfg.TaxBps)

	total, err := money.Sum(base, t.Tax, t.DeliveryFee)
	if err != nil {
		return t, err
	}
	t.Total = money.ClampNonNegative(total) // BR-2.5.7: never negative
	return t, nil
}

// KitchenUnits totals a cart's weight on capacity axis 2 (BR-2.3.7).
func KitchenUnits(lines []Line) int {
	total := 0
	for _, l := range lines {
		total += l.KitchenUnits * l.Qty
	}
	return total
}

// DiscountType is how a promotion reduces the subtotal.
type DiscountType string

const (
	Percent DiscountType = "percent"
	Fixed   DiscountType = "fixed"
)

// Promotion is a promo code's rules (BR-2.5.8).
type Promotion struct {
	Code                string
	Type                DiscountType
	ValueBps            money.Bps    // percent promotions
	ValueAmount         money.Rupiah // fixed promotions
	MaxDiscount         money.Rupiah // 0 = uncapped
	MinSpend            money.Rupiah
	StartsAt            time.Time
	EndsAt              time.Time
	IsActive            bool
	UsageCapTotal       int // 0 = unlimited
	UsedCount           int
	UsageCapPerCustomer int // 0 = unlimited
	CustomerUsedCount   int
	StoreIDs            []string // explicit store list — no implicit "all" (D15)
	CategoryIDs         []string // empty = every category
}

// PromoReason explains a refusal. Each maps to a 422 with its own code.
type PromoReason string

const (
	PromoOK               PromoReason = ""
	PromoNotFound         PromoReason = "PROMO_NOT_FOUND"
	PromoInactive         PromoReason = "PROMO_INACTIVE"
	PromoNotStarted       PromoReason = "PROMO_NOT_STARTED"
	PromoExpired          PromoReason = "PROMO_EXPIRED"
	PromoStoreNotEligible PromoReason = "PROMO_STORE_NOT_ELIGIBLE"
	PromoMinSpend         PromoReason = "PROMO_MIN_SPEND"
	PromoNoEligibleItems  PromoReason = "PROMO_NO_ELIGIBLE_ITEMS"
	PromoCapReached       PromoReason = "PROMO_CAP_REACHED"
	PromoCustomerCap      PromoReason = "PROMO_CUSTOMER_CAP"
)

// PromoContext is the order being priced.
type PromoContext struct {
	StoreID string
	Lines   []Line
	Now     time.Time
}

// EvaluatePromotion returns the discount a promotion grants, or the reason it
// does not apply (BR-2.5.9, BR-2.5.10). It never returns a discount larger than
// the eligible subtotal, so a total can never go negative.
func EvaluatePromotion(p Promotion, ctx PromoContext) (money.Rupiah, PromoReason) {
	if !p.IsActive {
		return 0, PromoInactive
	}
	if ctx.Now.Before(p.StartsAt) {
		return 0, PromoNotStarted
	}
	if !ctx.Now.Before(p.EndsAt) {
		return 0, PromoExpired
	}
	if !containsString(p.StoreIDs, ctx.StoreID) {
		return 0, PromoStoreNotEligible
	}
	if p.UsageCapTotal > 0 && p.UsedCount >= p.UsageCapTotal {
		return 0, PromoCapReached
	}
	if p.UsageCapPerCustomer > 0 && p.CustomerUsedCount >= p.UsageCapPerCustomer {
		return 0, PromoCustomerCap
	}

	// Full subtotal decides the minimum spend; only eligible categories are
	// discounted.
	var subtotal, eligible money.Rupiah
	for _, l := range ctx.Lines {
		lt, err := l.LineTotal()
		if err != nil {
			return 0, PromoNoEligibleItems
		}
		subtotal += lt
		if len(p.CategoryIDs) == 0 || containsString(p.CategoryIDs, l.CategoryID) {
			eligible += lt
		}
	}
	if subtotal < p.MinSpend {
		return 0, PromoMinSpend
	}
	if eligible <= 0 {
		return 0, PromoNoEligibleItems
	}

	var discount money.Rupiah
	switch p.Type {
	case Percent:
		discount = money.ApplyRate(eligible, p.ValueBps)
	case Fixed:
		discount = p.ValueAmount
	default:
		return 0, PromoNotFound
	}

	if p.MaxDiscount > 0 {
		discount = money.Min(discount, p.MaxDiscount)
	}
	discount = money.Min(discount, eligible) // never exceeds what it discounts
	return money.ClampNonNegative(discount), PromoOK
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
