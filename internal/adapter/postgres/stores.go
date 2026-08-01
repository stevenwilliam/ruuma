package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/clock"
)

// StoreRepo reads and writes store master data, and assembles the pure
// schedule.Store value the domain needs (BR-2.1.x).
type StoreRepo struct {
	db     *gorm.DB
	params *ParamRepo
}

func NewStoreRepo(db *gorm.DB, params *ParamRepo) *StoreRepo {
	return &StoreRepo{db: db, params: params}
}

// scoped applies the caller's store scope. nil scope means every store and is
// only produced for admin, owner or group-scoped finance (BR-2.7.8).
func scoped(q *gorm.DB, column string, scope []uuid.UUID) *gorm.DB {
	if scope == nil {
		return q
	}
	if len(scope) == 0 {
		// An empty (non-nil) scope is a staff member with no assignment: they
		// see nothing, rather than everything.
		return q.Where("1 = 0")
	}
	return q.Where(column+" IN ?", scope)
}

// ListActive returns stores a customer may see, filtered by an optional search
// term (BR-1.5.1).
func (r *StoreRepo) ListActive(ctx context.Context, q string) ([]Store, error) {
	query := r.db.WithContext(ctx).Model(&Store{}).
		Where("is_active").Order("sort_order, name")
	if q != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		query = query.Where("name ILIKE ? OR code ILIKE ? OR address_line ILIKE ? OR city ILIKE ?",
			like, like, like, like)
	}
	var out []Store
	return out, query.Find(&out).Error
}

// ListAll returns every store in the caller's scope, for admin lists.
func (r *StoreRepo) ListAll(ctx context.Context, q string, scope []uuid.UUID) ([]Store, error) {
	query := scoped(r.db.WithContext(ctx).Model(&Store{}), "id", scope).Order("sort_order, name")
	if q != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		query = query.Where("name ILIKE ? OR code ILIKE ? OR address_line ILIKE ?", like, like, like)
	}
	var out []Store
	return out, query.Find(&out).Error
}

// Get returns one store, or a 404-shaped error.
func (r *StoreRepo) Get(ctx context.Context, id uuid.UUID) (*Store, error) {
	var s Store
	err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Store not found.")
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetBySlug resolves a customer-facing slug.
func (r *StoreRepo) GetBySlug(ctx context.Context, slug string) (*Store, error) {
	var s Store
	err := r.db.WithContext(ctx).First(&s, "slug = ?", slug).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Store not found.")
	}
	return &s, err
}

func (r *StoreRepo) Create(ctx context.Context, s *Store) error {
	s.CreatedAt, s.UpdatedAt = time.Now(), time.Now()
	return r.db.WithContext(ctx).Create(s).Error
}

// Update writes store master data. The code is deliberately not treated as
// editable once orders reference the store (BR-1.2.3); the admin UI keeps it
// read-only after creation and the audit trail records any change.
func (r *StoreRepo) Update(ctx context.Context, s *Store) error {
	s.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(s).Error
}

// SetActive hides or shows a store. Deactivation never touches historical
// orders (BR-2.1.11).
func (r *StoreRepo) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	return r.db.WithContext(ctx).Model(&Store{}).Where("id = ?", id).
		Updates(map[string]any{"is_active": active, "updated_at": time.Now()}).Error
}

// ── Schedule assembly ────────────────────────────────────────────────────────

// LoadSchedule builds the pure schedule.Store value for a date window. The
// domain never queries; it receives master data as a value (docs/05 §2).
func (r *StoreRepo) LoadSchedule(ctx context.Context, storeID uuid.UUID, from, to schedule.Date) (schedule.Store, *Store, error) {
	store, err := r.Get(ctx, storeID)
	if err != nil {
		return schedule.Store{}, nil, err
	}
	loc := clock.Location(store.Timezone)

	var modes []StoreFulfilmentMode
	if err := r.db.WithContext(ctx).Where("store_id = ?", storeID).Find(&modes).Error; err != nil {
		return schedule.Store{}, nil, err
	}
	supported := map[schedule.FulfilmentType]bool{}
	for _, m := range modes {
		if m.IsEnabled {
			supported[schedule.FulfilmentType(m.FulfilmentType)] = true
		}
	}

	var hours []StoreHour
	if err := r.db.WithContext(ctx).Where("store_id = ?", storeID).
		Order("weekday, fulfilment_type, block_index").Find(&hours).Error; err != nil {
		return schedule.Store{}, nil, err
	}

	fromT := from.Time(schedule.TimeOfDay{}, loc)
	toT := to.Time(schedule.TimeOfDay{}, loc)

	var overrides []StoreDateOverride
	if err := r.db.WithContext(ctx).
		Where("store_id = ? AND business_date BETWEEN ? AND ?", storeID, fromT, toT).
		Find(&overrides).Error; err != nil {
		return schedule.Store{}, nil, err
	}

	var blackouts []StoreBlackoutDate
	if err := r.db.WithContext(ctx).
		Where("store_id = ? AND business_date BETWEEN ? AND ?", storeID, fromT, toT).
		Find(&blackouts).Error; err != nil {
		return schedule.Store{}, nil, err
	}

	out := schedule.Store{
		Location:       loc,
		IsActive:       store.IsActive,
		SupportedModes: supported,
		Params:         r.Params(ctx, storeID),
	}

	byWeekdayMode := map[string]*schedule.WeekdayHours{}
	for _, h := range hours {
		key := fmt.Sprintf("%d/%s", h.Weekday, h.FulfilmentType)
		wh, ok := byWeekdayMode[key]
		if !ok {
			wh = &schedule.WeekdayHours{
				Weekday: time.Weekday(h.Weekday),
				Mode:    schedule.FulfilmentType(h.FulfilmentType),
			}
			byWeekdayMode[key] = wh
		}
		if h.IsClosed {
			wh.IsClosed = true
			continue
		}
		opens, closes, err := parseBlockTimes(h.OpensAt, h.ClosesAt)
		if err != nil {
			return schedule.Store{}, nil, err
		}
		wh.Blocks = append(wh.Blocks, schedule.Block{Index: h.BlockIndex, Opens: opens, Closes: closes})
	}
	for _, wh := range byWeekdayMode {
		out.Weekly = append(out.Weekly, *wh)
	}

	for _, o := range overrides {
		d := schedule.DateOf(o.BusinessDate, time.UTC) // DATE columns come back at midnight UTC
		ov := schedule.DateOverride{
			Date:     d,
			Mode:     schedule.FulfilmentType(o.FulfilmentType),
			IsClosed: o.IsClosed,
		}
		if !o.IsClosed {
			opens, closes, err := parseBlockTimes(o.OpensAt, o.ClosesAt)
			if err != nil {
				return schedule.Store{}, nil, err
			}
			ov.Blocks = []schedule.Block{{Index: o.BlockIndex, Opens: opens, Closes: closes}}
		}
		out.Overrides = append(out.Overrides, ov)
	}

	for _, b := range blackouts {
		out.Blackouts = append(out.Blackouts, schedule.DateOf(b.BusinessDate, time.UTC))
	}

	return out, store, nil
}

// Params resolves this store's scheduling values (BR-2.1.12, BR-1.4.4).
func (r *StoreRepo) Params(ctx context.Context, storeID uuid.UUID) schedule.Params {
	id := storeID
	return schedule.Params{
		SlotLengthMinutes:   r.params.Int(ctx, &id, "scheduling.slot_length_minutes"),
		LeadTimeMinutes:     r.params.Int(ctx, &id, "scheduling.lead_time_minutes"),
		CutoffMinutes:       r.params.Int(ctx, &id, "scheduling.cutoff_minutes"),
		MaxAdvanceDays:      r.params.Int(ctx, &id, "scheduling.max_advance_days"),
		MaxOrdersPerSlot:    r.params.Int(ctx, &id, "scheduling.max_orders_per_slot"),
		MaxKitchenUnitsSlot: r.params.Int(ctx, &id, "scheduling.max_kitchen_units_per_slot"),
		CancelCutoffMinutes: r.params.Int(ctx, &id, "scheduling.cancel_cutoff_minutes"),
	}
}

func parseBlockTimes(opens, closes *string) (schedule.TimeOfDay, schedule.TimeOfDay, error) {
	if opens == nil || closes == nil {
		return schedule.TimeOfDay{}, schedule.TimeOfDay{}, fmt.Errorf("stores: open block missing its times")
	}
	o, err := parseTimeOfDay(*opens)
	if err != nil {
		return schedule.TimeOfDay{}, schedule.TimeOfDay{}, err
	}
	c, err := parseTimeOfDay(*closes)
	if err != nil {
		return schedule.TimeOfDay{}, schedule.TimeOfDay{}, err
	}
	return o, c, nil
}

func parseTimeOfDay(v string) (schedule.TimeOfDay, error) {
	for _, layout := range []string{"15:04:05", "15:04", "15:04:05.999999"} {
		if t, err := time.Parse(layout, v); err == nil {
			return schedule.TimeOfDay{Hour: t.Hour(), Minute: t.Minute()}, nil
		}
	}
	return schedule.TimeOfDay{}, fmt.Errorf("stores: cannot parse time %q", v)
}

// ── Hours, overrides, blackouts, bank accounts ───────────────────────────────

func (r *StoreRepo) Hours(ctx context.Context, storeID uuid.UUID) ([]StoreHour, error) {
	var out []StoreHour
	return out, r.db.WithContext(ctx).Where("store_id = ?", storeID).
		Order("weekday, fulfilment_type, block_index").Find(&out).Error
}

// ReplaceHours swaps a store's whole weekly pattern in one transaction, so a
// half-applied schedule can never be served.
func (r *StoreRepo) ReplaceHours(ctx context.Context, storeID uuid.UUID, hours []StoreHour) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("store_id = ?", storeID).Delete(&StoreHour{}).Error; err != nil {
			return err
		}
		for i := range hours {
			hours[i].ID = uuid.New()
			hours[i].StoreID = storeID
			hours[i].CreatedAt, hours[i].UpdatedAt = time.Now(), time.Now()
		}
		if len(hours) == 0 {
			return nil
		}
		return tx.Create(&hours).Error
	})
}

func (r *StoreRepo) DateOverrides(ctx context.Context, storeID uuid.UUID, from, to time.Time) ([]StoreDateOverride, error) {
	var out []StoreDateOverride
	return out, r.db.WithContext(ctx).
		Where("store_id = ? AND business_date BETWEEN ? AND ?", storeID, from, to).
		Order("business_date, fulfilment_type, block_index").Find(&out).Error
}

func (r *StoreRepo) UpsertDateOverride(ctx context.Context, o *StoreDateOverride) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	o.CreatedAt, o.UpdatedAt = time.Now(), time.Now()
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO store_date_overrides
			(id, store_id, business_date, fulfilment_type, block_index, is_closed, opens_at, closes_at, reason, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, now(), now())
		ON CONFLICT (store_id, business_date, fulfilment_type, block_index) DO UPDATE
		SET is_closed = EXCLUDED.is_closed, opens_at = EXCLUDED.opens_at,
		    closes_at = EXCLUDED.closes_at, reason = EXCLUDED.reason, updated_at = now()`,
		o.ID, o.StoreID, o.BusinessDate, o.FulfilmentType, o.BlockIndex, o.IsClosed,
		o.OpensAt, o.ClosesAt, o.Reason, o.CreatedBy).Error
}

func (r *StoreRepo) DeleteDateOverride(ctx context.Context, storeID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND store_id = ?", id, storeID).Delete(&StoreDateOverride{}).Error
}

func (r *StoreRepo) Blackouts(ctx context.Context, storeID uuid.UUID, from, to time.Time) ([]StoreBlackoutDate, error) {
	var out []StoreBlackoutDate
	return out, r.db.WithContext(ctx).
		Where("store_id = ? AND business_date BETWEEN ? AND ?", storeID, from, to).
		Order("business_date").Find(&out).Error
}

// AddBlackout closes a store for a date. The date may be **today** — emergency
// closure is the point (BR-2.1.7, D27) — so there is deliberately no guard
// against past or current dates here.
func (r *StoreRepo) AddBlackout(ctx context.Context, b *StoreBlackoutDate) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	if strings.TrimSpace(b.Reason) == "" {
		return apierror.Validation("A blackout requires a reason.", map[string]any{"reason": "required"})
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO store_blackout_dates (id, store_id, business_date, reason, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5, now(), now())
		ON CONFLICT (store_id, business_date) DO UPDATE
		SET reason = EXCLUDED.reason, created_by = EXCLUDED.created_by, updated_at = now()`,
		b.ID, b.StoreID, b.BusinessDate, b.Reason, b.CreatedBy).Error
}

func (r *StoreRepo) RemoveBlackout(ctx context.Context, storeID uuid.UUID, date time.Time) error {
	return r.db.WithContext(ctx).
		Where("store_id = ? AND business_date = ?", storeID, date).
		Delete(&StoreBlackoutDate{}).Error
}

// CountAffectedOrders reports how many live orders a closure would hit. They
// are never auto-cancelled — staff handle them by hand (BR-2.1.9, D27) — but
// the manager must be told the number at the moment they close the store.
func (r *StoreRepo) CountAffectedOrders(ctx context.Context, storeID uuid.UUID, date time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&Order{}).
		Where("store_id = ? AND business_date = ? AND status NOT IN ?",
			storeID, date, []string{"CANCELLED", "REFUNDED", "COMPLETED", "PICKED_UP", "DELIVERED"}).
		Count(&n).Error
	return n, err
}

func (r *StoreRepo) BankAccounts(ctx context.Context, storeID uuid.UUID) ([]StoreBankAccount, error) {
	var out []StoreBankAccount
	return out, r.db.WithContext(ctx).Where("store_id = ?", storeID).
		Order("is_primary DESC, bank_name").Find(&out).Error
}

// PrimaryBankAccount is the account shown at checkout for this store
// (BR-2.1.13).
func (r *StoreRepo) PrimaryBankAccount(ctx context.Context, storeID uuid.UUID) (*StoreBankAccount, error) {
	var a StoreBankAccount
	err := r.db.WithContext(ctx).
		Where("store_id = ? AND is_primary AND is_active", storeID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.Unprocessable(apierror.CodeValidation,
			"This store has no bank account configured for transfers.")
	}
	return &a, err
}

// SetPrimaryBankAccount moves the primary flag atomically; the partial unique
// index would otherwise refuse two primaries mid-update.
func (r *StoreRepo) SetPrimaryBankAccount(ctx context.Context, storeID, accountID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&StoreBankAccount{}).
			Where("store_id = ?", storeID).Update("is_primary", false).Error; err != nil {
			return err
		}
		return tx.Model(&StoreBankAccount{}).
			Where("id = ? AND store_id = ?", accountID, storeID).Update("is_primary", true).Error
	})
}

func (r *StoreRepo) Modes(ctx context.Context, storeID uuid.UUID) ([]StoreFulfilmentMode, error) {
	var out []StoreFulfilmentMode
	return out, r.db.WithContext(ctx).Where("store_id = ?", storeID).Find(&out).Error
}

// ReplaceModes sets which fulfilment modes a store supports (BR-2.1.2).
func (r *StoreRepo) ReplaceModes(ctx context.Context, storeID uuid.UUID, modes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("store_id = ?", storeID).Delete(&StoreFulfilmentMode{}).Error; err != nil {
			return err
		}
		for _, m := range modes {
			row := StoreFulfilmentMode{
				ID: uuid.New(), StoreID: storeID, FulfilmentType: m, IsEnabled: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// AssignedStores returns a staff member's store ids (BR-2.7.7). This is what
// becomes Principal.Stores on every request — never a client claim.
func (r *StoreRepo) AssignedStores(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Model(&StaffStoreAssignment{}).
		Where("user_id = ?", userID).Pluck("store_id", &ids).Error
	return ids, err
}

// ReplaceAssignments sets a staff member's stores.
func (r *StoreRepo) ReplaceAssignments(ctx context.Context, userID uuid.UUID, storeIDs []uuid.UUID, actor uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&StaffStoreAssignment{}).Error; err != nil {
			return err
		}
		for _, sid := range storeIDs {
			row := StaffStoreAssignment{
				ID: uuid.New(), UserID: userID, StoreID: sid,
				CreatedBy: &actor, CreatedAt: time.Now(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
