// Package schedule resolves what a store is open for on a given date and turns
// that into bookable slots.
//
// It is pure: no database, no clock of its own — the caller passes `now`
// explicitly (docs/05 §3.3) so cutoff and lead-time rules are testable to the
// minute. Every local↔UTC conversion goes through the store's *time.Location
// (BR-1.3.2); nothing here reads the server's local time.
package schedule

import (
	"sort"
	"time"
)

// FulfilmentType is how the customer receives the order (BR-2.3.1).
type FulfilmentType string

const (
	Pickup   FulfilmentType = "pickup"
	Delivery FulfilmentType = "delivery"
)

// Reason explains why a date or slot is not bookable. Every unbookable slot
// returned to a client carries one (BR-2.3.6) — a greyed-out box with no
// explanation is a support call.
type Reason string

const (
	ReasonOK              Reason = ""
	ReasonPast            Reason = "PAST"
	ReasonClosed          Reason = "CLOSED"
	ReasonBlackout        Reason = "BLACKOUT"
	ReasonLeadTime        Reason = "LEAD_TIME"
	ReasonCutoff          Reason = "CUTOFF"
	ReasonFull            Reason = "FULL"
	ReasonModeUnsupported Reason = "MODE_UNSUPPORTED"
	ReasonModeDisabled    Reason = "MODE_DISABLED"
	ReasonStoreInactive   Reason = "STORE_INACTIVE"
	ReasonBeyondAdvance   Reason = "BEYOND_ADVANCE"
	ReasonItemConstraint  Reason = "ITEM_CONSTRAINT"
	ReasonSlotLocked      Reason = "SLOT_LOCKED"
)

// TimeOfDay is a store-local wall-clock time. It is deliberately not a
// time.Time: opening hours have no date and no zone until they are applied to
// one.
type TimeOfDay struct {
	Hour   int
	Minute int
}

func (t TimeOfDay) Minutes() int { return t.Hour*60 + t.Minute }

func (t TimeOfDay) Before(o TimeOfDay) bool { return t.Minutes() < o.Minutes() }

// Date is a calendar date in the store's timezone (BR-1.3.3).
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the business date of an instant in loc.
func DateOf(t time.Time, loc *time.Location) Date {
	l := t.In(loc)
	return Date{Year: l.Year(), Month: l.Month(), Day: l.Day()}
}

// Time converts a date + wall-clock time into an instant in loc.
func (d Date) Time(tod TimeOfDay, loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, tod.Hour, tod.Minute, 0, 0, loc)
}

// Weekday reports the weekday of the date in loc.
func (d Date) Weekday(loc *time.Location) time.Weekday {
	return d.Time(TimeOfDay{}, loc).Weekday()
}

// AddDays returns the date n days later.
func (d Date) AddDays(n int, loc *time.Location) Date {
	t := d.Time(TimeOfDay{}, loc).AddDate(0, 0, n)
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

func (d Date) String() string {
	return d.Time(TimeOfDay{}, time.UTC).Format("2006-01-02")
}

// Equal compares two dates.
func (d Date) Equal(o Date) bool { return d == o }

// Before reports whether d falls before o.
func (d Date) Before(o Date) bool {
	if d.Year != o.Year {
		return d.Year < o.Year
	}
	if d.Month != o.Month {
		return d.Month < o.Month
	}
	return d.Day < o.Day
}

// Block is one opening block, e.g. lunch 10:00–14:00 (BR-2.1.4).
type Block struct {
	Index  int
	Opens  TimeOfDay
	Closes TimeOfDay
}

// WeekdayHours is a store's standing pattern for one weekday and mode.
type WeekdayHours struct {
	Weekday  time.Weekday
	Mode     FulfilmentType
	IsClosed bool
	Blocks   []Block
}

// DateOverride replaces the weekday pattern for one specific date (BR-2.1.6).
type DateOverride struct {
	Date     Date
	Mode     FulfilmentType
	IsClosed bool
	Blocks   []Block
}

// Params are the store's scheduling values, each resolved store → group default
// → compiled fallback (BR-1.4.4, BR-2.9.1).
type Params struct {
	SlotLengthMinutes   int
	LeadTimeMinutes     int
	CutoffMinutes       int
	MaxAdvanceDays      int
	MaxOrdersPerSlot    int
	MaxKitchenUnitsSlot int
	CancelCutoffMinutes int
}

// Store is everything the resolver needs about one store. It is a value, built
// by the app layer from master data — the domain never queries.
type Store struct {
	Location       *time.Location
	IsActive       bool
	SupportedModes map[FulfilmentType]bool
	Weekly         []WeekdayHours
	Overrides      []DateOverride
	Blackouts      []Date
	Params         Params
}

// DeliveryEnabled is the group-wide phase-2 switch (BR-2.1.3, D16). It is not a
// store field because it is a group decision, and passing it explicitly keeps
// the rule visible at the call site.
type Group struct {
	DeliveryEnabled bool
}

// ModeAllowed applies the store's declared modes and the group switch
// (BR-2.1.2, BR-2.1.3).
func ModeAllowed(s Store, g Group, mode FulfilmentType) Reason {
	if mode == Delivery && !g.DeliveryEnabled {
		return ReasonModeDisabled
	}
	if !s.SupportedModes[mode] {
		return ReasonModeUnsupported
	}
	return ReasonOK
}

// EffectiveBlocks resolves the opening blocks for one date and mode, applying
// the precedence in BR-2.1.8: per-date override → blackout → weekday schedule.
//
// A per-date override therefore deliberately outranks a blackout: it is the only
// way to reopen a blacked-out date on purpose.
func EffectiveBlocks(s Store, d Date, mode FulfilmentType) ([]Block, Reason) {
	// 1. per-date override (highest)
	var overrides []Block
	found := false
	for _, o := range s.Overrides {
		if o.Mode != mode || !o.Date.Equal(d) {
			continue
		}
		found = true
		if o.IsClosed {
			return nil, ReasonClosed
		}
		overrides = append(overrides, o.Blocks...)
	}
	if found {
		if len(overrides) == 0 {
			return nil, ReasonClosed
		}
		return sortBlocks(overrides), ReasonOK
	}

	// 2. blackout
	for _, b := range s.Blackouts {
		if b.Equal(d) {
			return nil, ReasonBlackout
		}
	}

	// 3. weekday pattern
	weekday := d.Weekday(s.Location)
	var blocks []Block
	matched := false
	for _, h := range s.Weekly {
		if h.Weekday != weekday || h.Mode != mode {
			continue
		}
		matched = true
		if h.IsClosed {
			return nil, ReasonClosed
		}
		blocks = append(blocks, h.Blocks...)
	}
	if !matched || len(blocks) == 0 {
		return nil, ReasonClosed
	}
	return sortBlocks(blocks), ReasonOK
}

func sortBlocks(in []Block) []Block {
	out := make([]Block, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool { return out[i].Opens.Minutes() < out[j].Opens.Minutes() })
	return out
}

// DateBookable reports whether a date may be offered at all (BR-2.3.2).
func DateBookable(s Store, g Group, d Date, mode FulfilmentType, now time.Time) Reason {
	if !s.IsActive {
		return ReasonStoreInactive
	}
	if r := ModeAllowed(s, g, mode); r != ReasonOK {
		return r
	}
	today := DateOf(now, s.Location)
	if d.Before(today) {
		return ReasonPast
	}
	last := today.AddDays(s.Params.MaxAdvanceDays, s.Location)
	if last.Before(d) {
		return ReasonBeyondAdvance
	}
	if _, r := EffectiveBlocks(s, d, mode); r != ReasonOK {
		return r
	}
	return ReasonOK
}

// Slot is a generated fulfilment window. Times are instants; the caller stores
// them in UTC and the business date alongside (BR-1.3.1, BR-2.3.4).
type Slot struct {
	Date     Date
	Mode     FulfilmentType
	StartsAt time.Time
	EndsAt   time.Time
}

// Generate cuts the effective blocks into slots of SlotLengthMinutes, aligned to
// each block's opening time. A trailing interval shorter than a whole slot is
// discarded — half a slot cannot be cooked in half the time (BR-2.3.3).
func Generate(s Store, d Date, mode FulfilmentType) ([]Slot, Reason) {
	blocks, reason := EffectiveBlocks(s, d, mode)
	if reason != ReasonOK {
		return nil, reason
	}
	length := s.Params.SlotLengthMinutes
	if length <= 0 {
		return nil, ReasonClosed
	}

	var slots []Slot
	for _, b := range blocks {
		start := d.Time(b.Opens, s.Location)
		end := d.Time(b.Closes, s.Location)
		for cur := start; !cur.Add(time.Duration(length) * time.Minute).After(end); {
			next := cur.Add(time.Duration(length) * time.Minute)
			slots = append(slots, Slot{
				Date:     d,
				Mode:     mode,
				StartsAt: cur.UTC(),
				EndsAt:   next.UTC(),
			})
			cur = next
		}
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].StartsAt.Before(slots[j].StartsAt) })
	return slots, ReasonOK
}

// SlotState is a materialised slot's live counters, as the app layer read them.
type SlotState struct {
	StartsAt             time.Time
	EndsAt               time.Time
	Mode                 FulfilmentType
	Date                 Date
	MaxOrders            int
	MaxKitchenUnits      int
	ReservedOrders       int
	ReservedKitchenUnits int
	IsLocked             bool
}

// RemainingOrders is the free capacity on axis 1 (BR-2.3.7).
func (s SlotState) RemainingOrders() int { return max0(s.MaxOrders - s.ReservedOrders) }

// RemainingKitchenUnits is the free capacity on axis 2 (BR-2.3.7).
func (s SlotState) RemainingKitchenUnits() int {
	return max0(s.MaxKitchenUnits - s.ReservedKitchenUnits)
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// Request is one attempt to book a slot.
type Request struct {
	KitchenUnits int // what this order would consume on axis 2
	// ItemLeadMinutes is the largest min_lead_minutes across the cart's items;
	// item constraints intersect with slot availability (BR-2.2.7).
	ItemLeadMinutes int
	// ItemBlocked is set when an item's availability rule excludes this slot.
	ItemBlocked bool
}

// Bookable applies every gate in BR-2.3.5 in the order a human would explain
// them, and returns the first failing reason.
func Bookable(s Store, g Group, slot SlotState, req Request, now time.Time) Reason {
	if !s.IsActive {
		return ReasonStoreInactive
	}
	if r := ModeAllowed(s, g, slot.Mode); r != ReasonOK {
		return r
	}
	if _, r := EffectiveBlocks(s, slot.Date, slot.Mode); r != ReasonOK {
		return r // covers CLOSED and BLACKOUT, including a blackout added today
	}
	if slot.IsLocked {
		return ReasonSlotLocked
	}
	if !slot.StartsAt.After(now) {
		return ReasonPast
	}

	lead := s.Params.LeadTimeMinutes
	if req.ItemLeadMinutes > lead {
		lead = req.ItemLeadMinutes // the stricter of store and item wins
	}
	if now.Add(time.Duration(lead) * time.Minute).After(slot.StartsAt) {
		return ReasonLeadTime
	}
	if now.After(slot.StartsAt.Add(-time.Duration(s.Params.CutoffMinutes) * time.Minute)) {
		return ReasonCutoff
	}
	if req.ItemBlocked {
		return ReasonItemConstraint
	}

	if slot.RemainingOrders() < 1 {
		return ReasonFull
	}
	if req.KitchenUnits > slot.RemainingKitchenUnits() {
		return ReasonFull
	}
	// Beyond the advance window the slot should not exist, but a stale client
	// can still ask for one — check it here too rather than trusting the caller.
	today := DateOf(now, s.Location)
	if today.AddDays(s.Params.MaxAdvanceDays, s.Location).Before(slot.Date) {
		return ReasonBeyondAdvance
	}
	return ReasonOK
}

// AlmostFull reports whether a slot should be shown as scarce. Honest scarcity
// is a product requirement, not a growth trick (docs/01 §3.1).
func AlmostFull(s SlotState) bool {
	if s.MaxOrders <= 0 {
		return false
	}
	remaining := s.RemainingOrders()
	return remaining > 0 && remaining*4 <= s.MaxOrders
}

// CancellableByCustomer reports whether the customer may still cancel
// (BR-2.3.13). After the cutoff, only staff may.
func CancellableByCustomer(s Store, slotStartsAt time.Time, now time.Time) bool {
	cutoff := slotStartsAt.Add(-time.Duration(s.Params.CancelCutoffMinutes) * time.Minute)
	return now.Before(cutoff)
}
