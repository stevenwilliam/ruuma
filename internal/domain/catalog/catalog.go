// Package catalog resolves what a store can actually sell right now, and
// validates option selections (BR-2.2.x).
//
// The menu is group-level master data; a store overrides availability and price
// (BR-2.2.1). Everything here is pure — the app layer loads the rows and hands
// them over as values.
package catalog

import (
	"errors"
	"time"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
)

// Availability is why an item can or cannot be ordered.
type Availability string

const (
	Available        Availability = ""
	ItemInactive     Availability = "ITEM_INACTIVE"
	CategoryInactive Availability = "CATEGORY_INACTIVE"
	StoreUnavailable Availability = "STORE_UNAVAILABLE"
	EightySixed      Availability = "OUT_OF_STOCK_86"
	SoldOutToday     Availability = "SOLD_OUT_TODAY"
	RuleExcluded     Availability = "NOT_AVAILABLE_AT_THIS_TIME"
)

// Item is a menu item with its group-level attributes.
type Item struct {
	ID             string
	CategoryID     string
	CategoryActive bool
	IsActive       bool
	BasePrice      money.Rupiah
	KitchenUnits   int
	PrepMinutes    int
	MinLeadMinutes int
}

// StoreOverride is a store's opinion about one item (BR-2.2.1). Nil fields
// inherit the group default — that is the whole point of the table.
type StoreOverride struct {
	IsAvailable   *bool
	PriceOverride *money.Rupiah
}

// EightySix marks an item out of stock for a window at one store (BR-2.2.3).
type EightySix struct {
	StartsAt time.Time
	EndsAt   *time.Time // nil = until lifted
}

// Active reports whether the 86 covers an instant.
func (e EightySix) Active(at time.Time) bool {
	if at.Before(e.StartsAt) {
		return false
	}
	return e.EndsAt == nil || at.Before(*e.EndsAt)
}

// DailyStock is the per-date countdown (BR-2.2.4).
type DailyStock struct {
	Total int
	Used  int
}

// Remaining is what is left today.
func (d DailyStock) Remaining() int {
	if d.Total-d.Used < 0 {
		return 0
	}
	return d.Total - d.Used
}

// AvailabilityRule restricts an item to weekdays and/or a time window
// (BR-2.2.7). WeekdayMask uses bit 0 = Sunday … bit 6 = Saturday.
type AvailabilityRule struct {
	WeekdayMask int
	FromTime    *schedule.TimeOfDay
	ToTime      *schedule.TimeOfDay
}

// Admits reports whether the rule allows a slot starting at a store-local time.
func (r AvailabilityRule) Admits(local time.Time) bool {
	if r.WeekdayMask != 0 && r.WeekdayMask != 127 {
		if r.WeekdayMask&(1<<uint(local.Weekday())) == 0 {
			return false
		}
	}
	minutes := local.Hour()*60 + local.Minute()
	if r.FromTime != nil && minutes < r.FromTime.Minutes() {
		return false
	}
	if r.ToTime != nil && minutes >= r.ToTime.Minutes() {
		return false
	}
	return true
}

// Query is one availability question: this item, at this store, for this slot.
type Query struct {
	Item       Item
	Override   *StoreOverride
	EightySixs []EightySix
	Stock      *DailyStock
	Rules      []AvailabilityRule
	Qty        int
	// SlotStartLocal is the slot's start time in the store's timezone; nil when
	// asking about the menu in general rather than a specific slot.
	SlotStartLocal *time.Time
	Now            time.Time
}

// Resolve reports whether an item may be ordered, and why not if it may not
// (BR-2.2.2). The checks run cheapest-first and in the order a human explains
// them.
func Resolve(q Query) Availability {
	if !q.Item.IsActive {
		return ItemInactive
	}
	if !q.Item.CategoryActive {
		return CategoryInactive
	}
	if q.Override != nil && q.Override.IsAvailable != nil && !*q.Override.IsAvailable {
		return StoreUnavailable
	}
	for _, e := range q.EightySixs {
		if e.Active(q.Now) {
			return EightySixed
		}
	}
	if q.Stock != nil {
		qty := q.Qty
		if qty <= 0 {
			qty = 1
		}
		if q.Stock.Remaining() < qty {
			return SoldOutToday
		}
	}
	if q.SlotStartLocal != nil && len(q.Rules) > 0 {
		admitted := false
		for _, r := range q.Rules {
			if r.Admits(*q.SlotStartLocal) {
				admitted = true
				break
			}
		}
		if !admitted {
			return RuleExcluded
		}
	}
	return Available
}

// EffectivePrice is the store override if present, otherwise the group price
// (BR-2.2.1).
func EffectivePrice(item Item, override *StoreOverride) money.Rupiah {
	if override != nil && override.PriceOverride != nil {
		return *override.PriceOverride
	}
	return item.BasePrice
}

// ── Option groups ────────────────────────────────────────────────────────────

// Selection is how many choices a group takes (BR-2.2.5).
type Selection string

const (
	Single Selection = "single"
	Multi  Selection = "multi"
)

// OptionGroup is a set of choices attached to an item.
type OptionGroup struct {
	ID         string
	Selection  Selection
	IsRequired bool
	MinSelect  int
	MaxSelect  int
	Choices    []OptionChoice
}

// OptionChoice is one selectable option with its own price delta and its own
// availability (BR-2.2.6).
type OptionChoice struct {
	ID           string
	PriceDelta   money.Rupiah
	KitchenUnits int
	IsAvailable  bool
}

var (
	ErrRequiredGroupMissing = errors.New("catalog: a required option group has no selection")
	ErrTooFewSelected       = errors.New("catalog: fewer options selected than the group allows")
	ErrTooManySelected      = errors.New("catalog: more options selected than the group allows")
	ErrUnknownChoice        = errors.New("catalog: option choice does not belong to this item")
	ErrChoiceUnavailable    = errors.New("catalog: option choice is unavailable")
	ErrDuplicateChoice      = errors.New("catalog: the same choice was selected twice")
)

// OptionResult is the priced outcome of a valid selection.
type OptionResult struct {
	Delta        money.Rupiah
	KitchenUnits int
}

// ValidateOptions checks a selection against an item's groups and returns the
// price delta and kitchen-unit weight it adds (BR-2.2.5, BR-2.2.6).
func ValidateOptions(groups []OptionGroup, selectedIDs []string) (OptionResult, error) {
	var out OptionResult

	// Index the choices, and refuse anything that is not on this item — a
	// client sending another item's choice id is not a valid order.
	choiceGroup := map[string]OptionGroup{}
	choice := map[string]OptionChoice{}
	for _, g := range groups {
		for _, c := range g.Choices {
			choiceGroup[c.ID] = g
			choice[c.ID] = c
		}
	}

	seen := map[string]bool{}
	perGroup := map[string]int{}
	for _, id := range selectedIDs {
		if seen[id] {
			return out, ErrDuplicateChoice
		}
		seen[id] = true

		c, ok := choice[id]
		if !ok {
			return out, ErrUnknownChoice
		}
		if !c.IsAvailable {
			return out, ErrChoiceUnavailable
		}
		perGroup[choiceGroup[id].ID]++
		out.Delta += c.PriceDelta
		out.KitchenUnits += c.KitchenUnits
	}

	for _, g := range groups {
		n := perGroup[g.ID]
		if g.IsRequired && n == 0 {
			return OptionResult{}, ErrRequiredGroupMissing
		}
		if n == 0 {
			continue // an optional group left untouched is fine
		}
		if g.Selection == Single && n > 1 {
			return OptionResult{}, ErrTooManySelected
		}
		if n < g.MinSelect {
			return OptionResult{}, ErrTooFewSelected
		}
		if g.MaxSelect > 0 && n > g.MaxSelect {
			return OptionResult{}, ErrTooManySelected
		}
	}
	return out, nil
}

// MaxItemLeadMinutes is the strictest item lead time in a cart; it intersects
// with the store's lead time when deciding which slots are bookable
// (BR-2.2.7).
func MaxItemLeadMinutes(items []Item) int {
	worst := 0
	for _, i := range items {
		if i.MinLeadMinutes > worst {
			worst = i.MinLeadMinutes
		}
	}
	return worst
}
