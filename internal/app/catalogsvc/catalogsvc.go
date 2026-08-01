// Package catalogsvc serves stores, the store-resolved menu, and slot
// availability (docs/04 §3).
package catalogsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/catalog"
	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/clock"
)

type Service struct {
	stores    ports.Stores
	catalogue ports.Catalogue
	slots     ports.Slots
	params    ports.Params
	clock     ports.Clock
}

func New(stores ports.Stores, catalogue ports.Catalogue, slots ports.Slots,
	params ports.Params, clk ports.Clock) *Service {
	return &Service{stores: stores, catalogue: catalogue, slots: slots, params: params, clock: clk}
}

// StoreSummary is a store card with its honest opening state (docs/01 §3.1).
type StoreSummary struct {
	Store        ports.StoreView
	OpenToday    bool
	TodayReason  schedule.Reason
	TodayBlocks  []string
	NextOpenDate *time.Time
}

// Stores lists active stores and tells the truth about today: open or closed,
// why, and when they next open (BR-2.1.4, docs/01 §3.1).
func (s *Service) Stores(ctx context.Context, q string) ([]StoreSummary, error) {
	rows, err := s.stores.ListActive(ctx, q)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()

	out := make([]StoreSummary, 0, len(rows))
	for _, st := range rows {
		summary := StoreSummary{Store: st}
		loc := clock.Location(st.Timezone)
		today := schedule.DateOf(now, loc)

		horizon := today.AddDays(14, loc)
		sched, err := s.stores.LoadSchedule(ctx, st.ID, today, horizon)
		if err != nil {
			return nil, err
		}

		mode := schedule.Pickup
		group := s.group(ctx)
		blocks, reason := schedule.EffectiveBlocks(sched, today, mode)
		summary.OpenToday = reason == schedule.ReasonOK
		summary.TodayReason = reason
		for _, b := range blocks {
			summary.TodayBlocks = append(summary.TodayBlocks, formatBlock(b))
		}
		if !summary.OpenToday {
			// Look ahead for the next open date so the card can say "opens
			// Monday" instead of just "closed".
			for i := 1; i <= 14; i++ {
				d := today.AddDays(i, loc)
				if r := schedule.DateBookable(sched, group, d, mode, now); r == schedule.ReasonOK {
					t := d.Time(schedule.TimeOfDay{}, loc)
					summary.NextOpenDate = &t
					break
				}
			}
		}
		out = append(out, summary)
	}
	return out, nil
}

// StoreDetail is one store with its full schedule.
type StoreDetail struct {
	Store    ports.StoreView
	Schedule schedule.Store
}

// Store returns a store and the schedule a customer needs to choose a date.
func (s *Service) Store(ctx context.Context, id uuid.UUID) (*StoreDetail, error) {
	st, err := s.stores.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	loc := clock.Location(st.Timezone)
	today := schedule.DateOf(s.clock.Now(), loc)
	sched, err := s.stores.LoadSchedule(ctx, id, today, today.AddDays(60, loc))
	if err != nil {
		return nil, err
	}
	return &StoreDetail{Store: *st, Schedule: sched}, nil
}

// Menu returns the store-resolved menu (BR-2.2.1, BR-1.5.1).
func (s *Service) Menu(ctx context.Context, q ports.MenuQuery) ([]ports.MenuItemView, error) {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 50
	}
	return s.catalogue.Menu(ctx, q, s.clock.Now())
}

// ItemDetail is an item with its option groups.
type ItemDetail struct {
	Item    ports.MenuItemView
	Options []ports.OptionGroupView
}

// Item returns one item with its options (BR-2.2.5).
func (s *Service) Item(ctx context.Context, storeID, itemID uuid.UUID) (*ItemDetail, error) {
	item, err := s.catalogue.Item(ctx, storeID, itemID, s.clock.Now())
	if err != nil {
		return nil, err
	}
	options, err := s.catalogue.Options(ctx, itemID)
	if err != nil {
		return nil, err
	}
	return &ItemDetail{Item: *item, Options: options}, nil
}

// Categories lists menu categories.
func (s *Service) Categories(ctx context.Context, q string) ([]ports.CategoryView, error) {
	return s.catalogue.Categories(ctx, q, true)
}

// DateAvailability is one bookable (or not) date.
type DateAvailability struct {
	Date       time.Time
	IsBookable bool
	Reason     schedule.Reason
}

// Dates returns the bookable dates for a store and mode, each unbookable one
// carrying its reason (BR-2.3.2, BR-2.3.6).
func (s *Service) Dates(ctx context.Context, storeID uuid.UUID, mode schedule.FulfilmentType, from time.Time, days int) ([]DateAvailability, error) {
	st, err := s.stores.Get(ctx, storeID)
	if err != nil {
		return nil, err
	}
	loc := clock.Location(st.Timezone)
	now := s.clock.Now()
	start := schedule.DateOf(from, loc)
	if start.Before(schedule.DateOf(now, loc)) {
		start = schedule.DateOf(now, loc)
	}
	if days <= 0 || days > 60 {
		days = 31
	}

	sched, err := s.stores.LoadSchedule(ctx, storeID, start, start.AddDays(days, loc))
	if err != nil {
		return nil, err
	}
	group := s.group(ctx)

	out := make([]DateAvailability, 0, days)
	for i := 0; i < days; i++ {
		d := start.AddDays(i, loc)
		reason := schedule.DateBookable(sched, group, d, mode, now)
		out = append(out, DateAvailability{
			Date:       d.Time(schedule.TimeOfDay{}, loc),
			IsBookable: reason == schedule.ReasonOK,
			Reason:     reason,
		})
	}
	return out, nil
}

// SlotQuery asks for a date's slots, optionally for a specific cart.
type SlotQuery struct {
	StoreID uuid.UUID
	Mode    schedule.FulfilmentType
	Date    time.Time
	ItemIDs []uuid.UUID
	Qty     int
}

// Slots materialises and returns a date's slots with remaining capacity and a
// reason on every unbookable one (BR-2.3.3 → BR-2.3.6).
func (s *Service) Slots(ctx context.Context, q SlotQuery) ([]ports.SlotView, error) {
	st, err := s.stores.Get(ctx, q.StoreID)
	if err != nil {
		return nil, err
	}
	loc := clock.Location(st.Timezone)
	now := s.clock.Now()
	date := schedule.DateOf(q.Date, loc)

	sched, err := s.stores.LoadSchedule(ctx, q.StoreID, date, date)
	if err != nil {
		return nil, err
	}
	group := s.group(ctx)

	if r := schedule.DateBookable(sched, group, date, q.Mode, now); r != schedule.ReasonOK {
		// The date itself is unavailable — return nothing with the reason
		// rather than a list of individually-refused slots.
		return nil, apierror.Unprocessable(apierror.CodeDateNotBookable,
			reasonMessage(r)).WithDetails(map[string]any{"reason": string(r)})
	}

	generated, reason := schedule.Generate(sched, date, q.Mode)
	if reason != schedule.ReasonOK {
		return nil, apierror.Unprocessable(apierror.CodeDateNotBookable, reasonMessage(reason))
	}
	if _, err := s.slots.Materialise(ctx, q.StoreID, generated, sched.Params); err != nil {
		return nil, err
	}

	states, ids, err := s.slots.ListForDate(ctx, q.StoreID,
		date.Time(schedule.TimeOfDay{}, time.UTC), string(q.Mode))
	if err != nil {
		return nil, err
	}

	// Item constraints narrow the bookable set for this cart (BR-2.2.7).
	itemLead := 0
	blockedBySlot := map[int]bool{}
	if len(q.ItemIDs) > 0 {
		for i, state := range states {
			local := state.StartsAt.In(loc)
			resolved, err := s.catalogue.ResolveForSlot(ctx, q.StoreID, q.ItemIDs, now, local)
			if err != nil {
				return nil, err
			}
			for _, item := range resolved {
				if item.Availability != catalog.Available {
					blockedBySlot[i] = true
				}
				if item.MinLeadMinutes > itemLead {
					itemLead = item.MinLeadMinutes
				}
			}
		}
	}

	qty := q.Qty
	if qty <= 0 {
		qty = 1
	}

	out := make([]ports.SlotView, 0, len(states))
	for i, state := range states {
		req := schedule.Request{
			KitchenUnits:    qty,
			ItemLeadMinutes: itemLead,
			ItemBlocked:     blockedBySlot[i],
		}
		reason := schedule.Bookable(sched, group, state, req, now)
		out = append(out, ports.SlotView{
			ID:              ids[i],
			StartsAt:        state.StartsAt,
			EndsAt:          state.EndsAt,
			IsBookable:      reason == schedule.ReasonOK,
			Reason:          reason,
			RemainingOrders: state.RemainingOrders(),
			RemainingUnits:  state.RemainingKitchenUnits(),
			AlmostFull:      schedule.AlmostFull(state),
		})
	}
	return out, nil
}

func (s *Service) group(ctx context.Context) schedule.Group {
	return schedule.Group{DeliveryEnabled: s.params.Bool(ctx, nil, "fulfilment.delivery_enabled")}
}

// formatBlock renders an opening block as the customer reads it on a store
// card: "10:00–21:00".
func formatBlock(b schedule.Block) string {
	return fmt.Sprintf("%02d:%02d–%02d:%02d",
		b.Opens.Hour, b.Opens.Minute, b.Closes.Hour, b.Closes.Minute)
}

// reasonMessage renders a machine reason as customer-facing copy. Every
// disabled state explains itself (docs/10 §4).
func reasonMessage(r schedule.Reason) string {
	switch r {
	case schedule.ReasonClosed:
		return "This store is closed on that date."
	case schedule.ReasonBlackout:
		return "This store is closed on that date."
	case schedule.ReasonPast:
		return "That date has already passed."
	case schedule.ReasonBeyondAdvance:
		return "That date is too far ahead to book yet."
	case schedule.ReasonModeUnsupported:
		return "This store does not offer that fulfilment method."
	case schedule.ReasonModeDisabled:
		return "That fulfilment method is not available yet."
	case schedule.ReasonStoreInactive:
		return "This store is not accepting orders."
	default:
		return "That date is not available."
	}
}
