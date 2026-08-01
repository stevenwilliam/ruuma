package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
)

// SlotRepo materialises slots and reserves capacity.
//
// Reservation is the sharpest edge in the system: two customers must never
// take the same last place (BR-2.3.10). The mechanism is SELECT … FOR UPDATE on
// the slot row (BR-2.3.8); the CHECK constraints on the table are the database's
// own refusal if anything ever bypasses this code (BR-2.3.9).
type SlotRepo struct {
	db *gorm.DB
}

func NewSlotRepo(db *gorm.DB) *SlotRepo { return &SlotRepo{db: db} }

// Materialise creates any missing slot rows for a store, date and mode from the
// generated schedule (BR-2.3.3, BR-2.3.4). It is idempotent: the unique index on
// (store, date, mode, start) makes a re-run a no-op.
//
// Existing rows are deliberately left alone — a capacity change must not
// retroactively alter a slot customers have already booked into (BR-2.3.16).
func (r *SlotRepo) Materialise(ctx context.Context, storeID uuid.UUID, generated []schedule.Slot, params schedule.Params) (int, error) {
	created := 0
	for _, s := range generated {
		date := time.Date(s.Date.Year, s.Date.Month, s.Date.Day, 0, 0, 0, 0, time.UTC)
		res := r.db.WithContext(ctx).Exec(`
			INSERT INTO slots (id, store_id, business_date, fulfilment_type, starts_at, ends_at,
			                   max_orders, max_kitchen_units, reserved_orders, reserved_kitchen_units,
			                   is_locked, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,0,0,false, now(), now())
			ON CONFLICT (store_id, business_date, fulfilment_type, starts_at) DO NOTHING`,
			uuid.New(), storeID, date, string(s.Mode), s.StartsAt.UTC(), s.EndsAt.UTC(),
			params.MaxOrdersPerSlot, params.MaxKitchenUnitsSlot)
		if res.Error != nil {
			return created, res.Error
		}
		created += int(res.RowsAffected)
	}
	return created, nil
}

// ListForDate returns the materialised slots for a store, date and mode.
func (r *SlotRepo) ListForDate(ctx context.Context, storeID uuid.UUID, date time.Time, mode string) ([]Slot, error) {
	var out []Slot
	return out, r.db.WithContext(ctx).
		Where("store_id = ? AND business_date = ? AND fulfilment_type = ?", storeID, date, mode).
		Order("starts_at").Find(&out).Error
}

// Get returns one slot.
func (r *SlotRepo) Get(ctx context.Context, id uuid.UUID) (*Slot, error) {
	var s Slot
	err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Slot not found.")
	}
	return &s, err
}

// LockForUpdate reads a slot inside a transaction and holds a row lock until
// commit (BR-2.3.8). Every reservation path must go through this, never a plain
// read-then-write.
func (r *SlotRepo) LockForUpdate(ctx context.Context, tx *gorm.DB, slotID uuid.UUID) (*Slot, error) {
	var s Slot
	err := tx.WithContext(ctx).Raw(
		`SELECT * FROM slots WHERE id = $1 FOR UPDATE`, slotID).Scan(&s).Error
	if err != nil {
		return nil, err
	}
	if s.ID == uuid.Nil {
		return nil, apierror.NotFound("Slot not found.")
	}
	return &s, nil
}

// Reserve consumes one order and n kitchen units from a locked slot
// (BR-2.3.7/8). The WHERE clause repeats the capacity check so that even a
// caller that skipped the domain check cannot oversell, and the UPDATE is
// integer arithmetic in SQL rather than a read-modify-write in Go.
//
// A zero-row result means the slot filled up between the read and the write —
// which cannot happen while the row lock is held, but is still handled as
// SLOT_FULL rather than as a silent success.
func (r *SlotRepo) Reserve(ctx context.Context, tx *gorm.DB, slotID uuid.UUID, kitchenUnits int) error {
	res := tx.WithContext(ctx).Exec(`
		UPDATE slots
		   SET reserved_orders        = reserved_orders + 1,
		       reserved_kitchen_units = reserved_kitchen_units + $2,
		       updated_at             = now()
		 WHERE id = $1
		   AND NOT is_locked
		   AND reserved_orders + 1 <= max_orders
		   AND reserved_kitchen_units + $2 <= max_kitchen_units`,
		slotID, kitchenUnits)
	if res.Error != nil {
		// The CHECK constraints are the last line of defence (BR-2.3.9); map a
		// violation to 409 rather than letting a 500 escape.
		if isCheckViolation(res.Error, "slots_no_oversell") {
			return apierror.Conflict(apierror.CodeSlotFull, "This time slot is fully booked.")
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apierror.Conflict(apierror.CodeSlotFull, "This time slot is fully booked.")
	}
	return nil
}

// Release returns capacity to a slot when an order is cancelled (BR-2.3.12).
// The GREATEST guards against ever driving a counter negative, and the caller
// makes the release idempotent by setting orders.capacity_released_at in the
// same transaction.
func (r *SlotRepo) Release(ctx context.Context, tx *gorm.DB, slotID uuid.UUID, kitchenUnits int) error {
	return tx.WithContext(ctx).Exec(`
		UPDATE slots
		   SET reserved_orders        = GREATEST(reserved_orders - 1, 0),
		       reserved_kitchen_units = GREATEST(reserved_kitchen_units - $2, 0),
		       updated_at             = now()
		 WHERE id = $1`, slotID, kitchenUnits).Error
}

// SetCapacity changes a slot's limits. Lowering capacity below what is already
// reserved is refused unless the manager confirms (BR-2.3.16).
func (r *SlotRepo) SetCapacity(ctx context.Context, slotID uuid.UUID, maxOrders, maxUnits int, confirmOverReserved bool) error {
	slot, err := r.Get(ctx, slotID)
	if err != nil {
		return err
	}
	if !confirmOverReserved && (slot.ReservedOrders > maxOrders || slot.ReservedKitchenUnits > maxUnits) {
		return apierror.Unprocessable(apierror.CodeValidation,
			"That capacity is below what is already booked for this slot. Confirm explicitly to proceed.").
			WithDetails(map[string]any{
				"reserved_orders":        slot.ReservedOrders,
				"reserved_kitchen_units": slot.ReservedKitchenUnits,
			})
	}
	// The table's CHECK constraints refuse max < reserved outright, so an
	// over-reserved confirmation raises the ceiling to what is already booked.
	if maxOrders < slot.ReservedOrders {
		maxOrders = slot.ReservedOrders
	}
	if maxUnits < slot.ReservedKitchenUnits {
		maxUnits = slot.ReservedKitchenUnits
	}
	return r.db.WithContext(ctx).Model(&Slot{}).Where("id = ?", slotID).
		Updates(map[string]any{
			"max_orders": maxOrders, "max_kitchen_units": maxUnits, "updated_at": time.Now(),
		}).Error
}

// SetLocked closes a slot for new orders without changing its capacity.
func (r *SlotRepo) SetLocked(ctx context.Context, slotID uuid.UUID, locked bool) error {
	return r.db.WithContext(ctx).Model(&Slot{}).Where("id = ?", slotID).
		Updates(map[string]any{"is_locked": locked, "updated_at": time.Now()}).Error
}

// State converts a row into the domain's view of a slot.
func (s Slot) State() schedule.SlotState {
	return schedule.SlotState{
		StartsAt:             s.StartsAt,
		EndsAt:               s.EndsAt,
		Mode:                 schedule.FulfilmentType(s.FulfilmentType),
		Date:                 schedule.DateOf(s.BusinessDate, time.UTC),
		MaxOrders:            s.MaxOrders,
		MaxKitchenUnits:      s.MaxKitchenUnits,
		ReservedOrders:       s.ReservedOrders,
		ReservedKitchenUnits: s.ReservedKitchenUnits,
		IsLocked:             s.IsLocked,
	}
}

// isCheckViolation reports whether err is a Postgres CHECK violation naming a
// constraint. Driver text never reaches a client (docs/12, A05) — it is only
// inspected here to choose the right typed error.
func isCheckViolation(err error, constraint string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, constraint) ||
		(strings.Contains(msg, "violates check constraint") && strings.Contains(msg, constraint))
}
