package schedule

import (
	"testing"
	"time"
)

var jakarta = mustLoad("Asia/Jakarta")

func mustLoad(n string) *time.Location {
	l, err := time.LoadLocation(n)
	if err != nil {
		panic(err)
	}
	return l
}

func lunchDinner() []Block {
	return []Block{
		{Index: 0, Opens: TimeOfDay{10, 0}, Closes: TimeOfDay{14, 0}},
		{Index: 1, Opens: TimeOfDay{17, 0}, Closes: TimeOfDay{21, 0}},
	}
}

// baseStore: open every weekday 10:00–14:00 and 17:00–21:00 for pickup,
// delivery closing an hour earlier (BR-2.1.5).
func baseStore() Store {
	s := Store{
		Location:       jakarta,
		IsActive:       true,
		SupportedModes: map[FulfilmentType]bool{Pickup: true, Delivery: true},
		Params: Params{
			SlotLengthMinutes:   30,
			LeadTimeMinutes:     90,
			CutoffMinutes:       60,
			MaxAdvanceDays:      14,
			MaxOrdersPerSlot:    12,
			MaxKitchenUnitsSlot: 60,
			CancelCutoffMinutes: 120,
		},
	}
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		s.Weekly = append(s.Weekly,
			WeekdayHours{Weekday: wd, Mode: Pickup, Blocks: lunchDinner()},
			WeekdayHours{Weekday: wd, Mode: Delivery, Blocks: []Block{
				{Index: 0, Opens: TimeOfDay{10, 0}, Closes: TimeOfDay{14, 0}},
				{Index: 1, Opens: TimeOfDay{17, 0}, Closes: TimeOfDay{20, 0}},
			}},
		)
	}
	return s
}

var (
	group    = Group{DeliveryEnabled: true}
	pickOnly = Group{DeliveryEnabled: false}
	// 2026-08-03 is a Monday.
	monday = Date{2026, time.August, 3}
	sunday = Date{2026, time.August, 2}
	sat    = Date{2026, time.August, 1}
)

// BR-2.1.4: a closed weekday generates no slots at all, for any mode.
func TestClosedWeekdayGeneratesNoSlots_BR_2_1_4(t *testing.T) {
	s := baseStore()
	for i := range s.Weekly {
		if s.Weekly[i].Weekday == time.Sunday {
			s.Weekly[i].IsClosed = true
			s.Weekly[i].Blocks = nil
		}
	}
	for _, mode := range []FulfilmentType{Pickup, Delivery} {
		slots, reason := Generate(s, sunday, mode)
		if len(slots) != 0 || reason != ReasonClosed {
			t.Fatalf("%s on a closed Sunday: got %d slots, reason %q; want 0, CLOSED",
				mode, len(slots), reason)
		}
	}
}

// BR-2.1.5: per-mode windows differ; delivery stops earlier than pickup.
func TestPerModeWindows_BR_2_1_5(t *testing.T) {
	s := baseStore()
	pickup, _ := Generate(s, monday, Pickup)
	delivery, _ := Generate(s, monday, Delivery)

	lastPickup := pickup[len(pickup)-1].EndsAt.In(jakarta)
	lastDelivery := delivery[len(delivery)-1].EndsAt.In(jakarta)

	if lastPickup.Hour() != 21 {
		t.Fatalf("pickup should end at 21:00, ended %s", lastPickup.Format("15:04"))
	}
	if lastDelivery.Hour() != 20 {
		t.Fatalf("delivery should end at 20:00, ended %s", lastDelivery.Format("15:04"))
	}
}

// BR-2.1.6 / BR-2.1.8: a per-date override replaces the weekday pattern and
// outranks a blackout.
func TestDateOverridePrecedence_BR_2_1_6_BR_2_1_8(t *testing.T) {
	s := baseStore()
	s.Blackouts = []Date{monday}

	// blackout alone closes the date
	if _, r := EffectiveBlocks(s, monday, Pickup); r != ReasonBlackout {
		t.Fatalf("blackout: got %q, want BLACKOUT", r)
	}

	// an override on the same date wins and reopens it (extended to 22:00)
	s.Overrides = []DateOverride{{
		Date: monday, Mode: Pickup,
		Blocks: []Block{{Index: 0, Opens: TimeOfDay{11, 0}, Closes: TimeOfDay{22, 0}}},
	}}
	blocks, r := EffectiveBlocks(s, monday, Pickup)
	if r != ReasonOK || len(blocks) != 1 {
		t.Fatalf("override should win over blackout: reason %q, blocks %d", r, len(blocks))
	}
	if blocks[0].Closes.Hour != 22 {
		t.Fatalf("override closes at %d, want 22", blocks[0].Closes.Hour)
	}

	// a closing override closes a normally-open date
	s.Blackouts = nil
	s.Overrides = []DateOverride{{Date: monday, Mode: Pickup, IsClosed: true}}
	if _, r := EffectiveBlocks(s, monday, Pickup); r != ReasonClosed {
		t.Fatalf("closing override: got %q, want CLOSED", r)
	}
}

// BR-2.1.7 / D27: a blackout may target today and blocks new bookings from that
// instant. Existing orders are untouched — that is the app layer's job, but the
// domain must stop offering the slot.
func TestSameDayBlackout_BR_2_1_7(t *testing.T) {
	s := baseStore()
	now := monday.Time(TimeOfDay{9, 0}, jakarta)
	slot := SlotState{
		Date: monday, Mode: Pickup,
		StartsAt:  monday.Time(TimeOfDay{18, 0}, jakarta).UTC(),
		EndsAt:    monday.Time(TimeOfDay{18, 30}, jakarta).UTC(),
		MaxOrders: 12, MaxKitchenUnits: 60,
	}
	if r := Bookable(s, group, slot, Request{KitchenUnits: 1}, now); r != ReasonOK {
		t.Fatalf("before blackout: got %q, want OK", r)
	}
	s.Blackouts = []Date{monday} // manager closes the store mid-morning
	if r := Bookable(s, group, slot, Request{KitchenUnits: 1}, now); r != ReasonBlackout {
		t.Fatalf("after same-day blackout: got %q, want BLACKOUT", r)
	}
}

// BR-2.3.3: slots align to block starts and a short trailing interval is
// discarded.
func TestSlotGenerationAlignment_BR_2_3_3(t *testing.T) {
	s := baseStore()
	s.Params.SlotLengthMinutes = 45
	s.Weekly = []WeekdayHours{{
		Weekday: monday.Weekday(jakarta), Mode: Pickup,
		Blocks: []Block{{Index: 0, Opens: TimeOfDay{10, 0}, Closes: TimeOfDay{12, 0}}},
	}}
	slots, r := Generate(s, monday, Pickup)
	if r != ReasonOK {
		t.Fatalf("reason %q", r)
	}
	// 10:00–12:00 with 45-minute slots yields 10:00, 10:45 — 11:30–12:15 does not
	// fit and is dropped.
	if len(slots) != 2 {
		t.Fatalf("got %d slots, want 2", len(slots))
	}
	first := slots[0].StartsAt.In(jakarta)
	if first.Hour() != 10 || first.Minute() != 0 {
		t.Fatalf("first slot %s, want 10:00", first.Format("15:04"))
	}
	last := slots[1].EndsAt.In(jakarta)
	if last.Hour() != 11 || last.Minute() != 30 {
		t.Fatalf("last slot ends %s, want 11:30", last.Format("15:04"))
	}
}

// BR-1.3.2: slot instants are stored in UTC and read back correctly whatever
// the server's timezone. 12:00 Jakarta is 05:00Z.
func TestTimezoneIsExplicit_BR_1_3_2(t *testing.T) {
	s := baseStore()
	slots, _ := Generate(s, monday, Pickup)
	var noon *Slot
	for i := range slots {
		if slots[i].StartsAt.In(jakarta).Hour() == 12 && slots[i].StartsAt.In(jakarta).Minute() == 0 {
			noon = &slots[i]
			break
		}
	}
	if noon == nil {
		t.Fatal("no 12:00 slot generated")
	}
	if got := noon.StartsAt.UTC().Format("15:04"); got != "05:00" {
		t.Fatalf("12:00 Jakarta stored as %sZ, want 05:00Z", got)
	}
}

// BR-2.3.5: lead time, cutoff, past and capacity each produce their own reason.
func TestBookabilityGates_BR_2_3_5_BR_2_3_6(t *testing.T) {
	s := baseStore()
	slotAt := func(h, m int) SlotState {
		return SlotState{
			Date: monday, Mode: Pickup,
			StartsAt:  monday.Time(TimeOfDay{h, m}, jakarta).UTC(),
			EndsAt:    monday.Time(TimeOfDay{h, m + 30}, jakarta).UTC(),
			MaxOrders: 12, MaxKitchenUnits: 60,
		}
	}
	cases := []struct {
		name string
		now  time.Time
		slot SlotState
		req  Request
		want Reason
	}{
		{"comfortably ahead", monday.Time(TimeOfDay{8, 0}, jakarta), slotAt(12, 0), Request{KitchenUnits: 2}, ReasonOK},
		{"slot already started", monday.Time(TimeOfDay{12, 1}, jakarta), slotAt(12, 0), Request{KitchenUnits: 1}, ReasonPast},
		{"inside lead time", monday.Time(TimeOfDay{11, 0}, jakarta), slotAt(12, 0), Request{KitchenUnits: 1}, ReasonLeadTime},
		{"past cutoff but outside lead", func() time.Time {
			// lead 30, cutoff 60: 11:15 is 45 min before start — lead OK, cutoff not
			return monday.Time(TimeOfDay{11, 15}, jakarta)
		}(), slotAt(12, 0), Request{KitchenUnits: 1}, ReasonCutoff},
		{"orders exhausted", monday.Time(TimeOfDay{8, 0}, jakarta), func() SlotState {
			sl := slotAt(12, 0)
			sl.ReservedOrders = sl.MaxOrders
			return sl
		}(), Request{KitchenUnits: 1}, ReasonFull},
		{"kitchen units exhausted", monday.Time(TimeOfDay{8, 0}, jakarta), func() SlotState {
			sl := slotAt(12, 0)
			sl.ReservedKitchenUnits = 59
			return sl
		}(), Request{KitchenUnits: 2}, ReasonFull},
		{"slot locked by manager", monday.Time(TimeOfDay{8, 0}, jakarta), func() SlotState {
			sl := slotAt(12, 0)
			sl.IsLocked = true
			return sl
		}(), Request{KitchenUnits: 1}, ReasonSlotLocked},
		{"item constraint", monday.Time(TimeOfDay{8, 0}, jakarta), slotAt(12, 0),
			Request{KitchenUnits: 1, ItemBlocked: true}, ReasonItemConstraint},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := s
			if tc.name == "past cutoff but outside lead" {
				st.Params.LeadTimeMinutes = 30
			}
			if got := Bookable(st, group, tc.slot, tc.req, tc.now); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// BR-2.2.7: an item's own lead time is stricter than the store's and narrows
// the bookable set.
func TestItemLeadTimeIntersects_BR_2_2_7(t *testing.T) {
	s := baseStore()
	now := monday.Time(TimeOfDay{8, 0}, jakarta)
	slot := SlotState{
		Date: monday, Mode: Pickup,
		StartsAt:  monday.Time(TimeOfDay{10, 0}, jakarta).UTC(),
		EndsAt:    monday.Time(TimeOfDay{10, 30}, jakarta).UTC(),
		MaxOrders: 12, MaxKitchenUnits: 60,
	}
	if r := Bookable(s, group, slot, Request{KitchenUnits: 1}, now); r != ReasonOK {
		t.Fatalf("store lead 90 min, slot 2h away: got %q, want OK", r)
	}
	// a dish needing four hours' notice cannot make a 10:00 slot at 08:00
	if r := Bookable(s, group, slot, Request{KitchenUnits: 1, ItemLeadMinutes: 240}, now); r != ReasonLeadTime {
		t.Fatalf("item lead 240 min: got %q, want LEAD_TIME", r)
	}
}

// BR-2.1.2 / BR-2.1.3: unsupported mode and the group-wide phase-2 switch.
func TestModeGates_BR_2_1_2_BR_2_1_3(t *testing.T) {
	s := baseStore()
	s.SupportedModes = map[FulfilmentType]bool{Pickup: true}

	if r := ModeAllowed(s, group, Delivery); r != ReasonModeUnsupported {
		t.Fatalf("pickup-only store: got %q, want MODE_UNSUPPORTED", r)
	}
	full := baseStore()
	if r := ModeAllowed(full, pickOnly, Delivery); r != ReasonModeDisabled {
		t.Fatalf("delivery disabled group-wide: got %q, want MODE_DISABLED", r)
	}
	if r := ModeAllowed(full, pickOnly, Pickup); r != ReasonOK {
		t.Fatalf("pickup in phase 1: got %q, want OK", r)
	}
}

// BR-2.3.2: max advance days and past dates.
func TestDateBookable_BR_2_3_2(t *testing.T) {
	s := baseStore()
	now := sat.Time(TimeOfDay{9, 0}, jakarta)

	if r := DateBookable(s, group, sat.AddDays(-1, jakarta), Pickup, now); r != ReasonPast {
		t.Fatalf("yesterday: got %q, want PAST", r)
	}
	if r := DateBookable(s, group, sat.AddDays(15, jakarta), Pickup, now); r != ReasonBeyondAdvance {
		t.Fatalf("day 15 with max 14: got %q, want BEYOND_ADVANCE", r)
	}
	if r := DateBookable(s, group, sat.AddDays(14, jakarta), Pickup, now); r != ReasonOK {
		t.Fatalf("day 14 with max 14: got %q, want OK", r)
	}
	s.IsActive = false
	if r := DateBookable(s, group, sat, Pickup, now); r != ReasonStoreInactive {
		t.Fatalf("inactive store: got %q, want STORE_INACTIVE", r)
	}
}

// BR-2.3.13: the customer's self-cancel window.
func TestCancellableByCustomer_BR_2_3_13(t *testing.T) {
	s := baseStore() // cancel cutoff 120 minutes
	start := monday.Time(TimeOfDay{12, 0}, jakarta)

	if !CancellableByCustomer(s, start, monday.Time(TimeOfDay{9, 0}, jakarta)) {
		t.Fatal("3 hours before: customer should be able to cancel")
	}
	if CancellableByCustomer(s, start, monday.Time(TimeOfDay{10, 30}, jakarta)) {
		t.Fatal("90 minutes before: customer should not be able to cancel")
	}
}

// Honest scarcity: "almost full" at a quarter of capacity or less remaining.
func TestAlmostFull(t *testing.T) {
	if !AlmostFull(SlotState{MaxOrders: 12, ReservedOrders: 9}) {
		t.Fatal("3 of 12 remaining should read as almost full")
	}
	if AlmostFull(SlotState{MaxOrders: 12, ReservedOrders: 4}) {
		t.Fatal("8 of 12 remaining is not almost full")
	}
	if AlmostFull(SlotState{MaxOrders: 12, ReservedOrders: 12}) {
		t.Fatal("a full slot is FULL, not almost full")
	}
}

// The three seeded stores in docs/01 differ deliberately; this asserts the
// resolver handles the awkward one (closed Saturday and Sunday).
func TestWeekendClosedStore_BR_2_1_4(t *testing.T) {
	s := baseStore()
	for i := range s.Weekly {
		if s.Weekly[i].Weekday == time.Saturday || s.Weekly[i].Weekday == time.Sunday {
			s.Weekly[i].IsClosed = true
			s.Weekly[i].Blocks = nil
		}
	}
	for _, d := range []Date{sat, sunday} {
		if _, r := EffectiveBlocks(s, d, Pickup); r != ReasonClosed {
			t.Fatalf("%s: got %q, want CLOSED", d, r)
		}
	}
	if _, r := EffectiveBlocks(s, monday, Pickup); r != ReasonOK {
		t.Fatalf("Monday: got %q, want OK", r)
	}
}
