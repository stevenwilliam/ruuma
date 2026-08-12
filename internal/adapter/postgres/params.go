package postgres

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
)

// ParamRepo resolves configuration: store override → group default → compiled
// fallback (BR-1.4.4). Nothing operational is hard-coded anywhere else in the
// service; this is the only place a default may appear, and only so a missing
// row cannot take the service down.
type ParamRepo struct {
	db *gorm.DB

	mu     sync.RWMutex
	group  map[string]string
	stores map[uuid.UUID]map[string]string
	loaded time.Time
	ttl    time.Duration
}

func NewParamRepo(db *gorm.DB) *ParamRepo {
	return &ParamRepo{db: db, ttl: 30 * time.Second}
}

// fallbacks mirror the defaults seeded by migration 0012. They exist only for
// the case where a row has been deleted; changing a value here changes nothing
// in a running system — the database wins (BR-1.4.1).
var fallbacks = map[string]string{
	"scheduling.slot_length_minutes":        "30",
	"scheduling.max_orders_per_slot":        "12",
	"scheduling.max_kitchen_units_per_slot": "60",
	"scheduling.lead_time_minutes":          "90",
	"scheduling.cutoff_minutes":             "60",
	"scheduling.max_advance_days":           "14",
	"scheduling.cancel_cutoff_minutes":      "120",
	"orders.auto_cancel_minutes":            "0",
	"orders.max_unpaid_per_customer":        "2",
	"pricing.tax_bps":                       "1000",
	"pricing.service_charge_bps":            "0",
	"pricing.tax_inclusive":                 "false",
	"pricing.quote_ttl_minutes":             "15",
	"fulfilment.delivery_enabled":           "false",
	"auth.otp_ttl_minutes":                  "5",
	"auth.otp_max_attempts":                 "5",
	"auth.access_token_minutes":             "15",
	"auth.refresh_token_days":               "30",
	"auth.provider_google_enabled":          "false",
	"auth.provider_instagram_enabled":       "false",
	"notify.provider":                       "log",
	"notify.slot_reminder_minutes":          "60",
	"finance.verification_sla_minutes":      "60",
	// The button hides itself when the number is missing, so the fallback for
	// the number is deliberately empty: a deleted row must not resurrect a
	// hard-coded number that nobody answers.
	"company.whatsapp_enabled": "true",
	"company.whatsapp_number":  "",
}

// Reload refreshes the cache. Parameter changes take effect without a restart
// (BR-2.9.2); the short TTL bounds how stale a value can be.
func (r *ParamRepo) Reload(ctx context.Context) error {
	var group []SysParameter
	if err := r.db.WithContext(ctx).Find(&group).Error; err != nil {
		return fmt.Errorf("params: load group: %w", err)
	}
	var perStore []StoreParameter
	if err := r.db.WithContext(ctx).Find(&perStore).Error; err != nil {
		return fmt.Errorf("params: load store overrides: %w", err)
	}

	g := make(map[string]string, len(group))
	for _, p := range group {
		g[p.Key] = p.Value
	}
	s := map[uuid.UUID]map[string]string{}
	for _, p := range perStore {
		if s[p.StoreID] == nil {
			s[p.StoreID] = map[string]string{}
		}
		s[p.StoreID][p.Key] = p.Value
	}

	r.mu.Lock()
	r.group, r.stores, r.loaded = g, s, time.Now()
	r.mu.Unlock()
	return nil
}

func (r *ParamRepo) ensure(ctx context.Context) {
	r.mu.RLock()
	fresh := r.group != nil && time.Since(r.loaded) < r.ttl
	r.mu.RUnlock()
	if !fresh {
		_ = r.Reload(ctx) // a stale cache still serves; a failed reload must not 500
	}
}

// Invalidate forces the next read to reload — called after an admin writes a
// parameter so the change is visible immediately.
func (r *ParamRepo) Invalidate() {
	r.mu.Lock()
	r.loaded = time.Time{}
	r.mu.Unlock()
}

// raw resolves one key for an optional store.
func (r *ParamRepo) raw(ctx context.Context, storeID *uuid.UUID, key string) string {
	r.ensure(ctx)
	r.mu.RLock()
	defer r.mu.RUnlock()

	if storeID != nil {
		if byStore, ok := r.stores[*storeID]; ok {
			if v, ok := byStore[key]; ok {
				return v
			}
		}
	}
	if v, ok := r.group[key]; ok {
		return v
	}
	return fallbacks[key]
}

// String resolves a string parameter.
func (r *ParamRepo) String(ctx context.Context, storeID *uuid.UUID, key string) string {
	return r.raw(ctx, storeID, key)
}

// Int resolves an integer parameter, falling back on a malformed value rather
// than failing a customer's request.
func (r *ParamRepo) Int(ctx context.Context, storeID *uuid.UUID, key string) int {
	if n, err := strconv.Atoi(r.raw(ctx, storeID, key)); err == nil {
		return n
	}
	if n, err := strconv.Atoi(fallbacks[key]); err == nil {
		return n
	}
	return 0
}

// Bool resolves a boolean parameter.
func (r *ParamRepo) Bool(ctx context.Context, storeID *uuid.UUID, key string) bool {
	if b, err := strconv.ParseBool(r.raw(ctx, storeID, key)); err == nil {
		return b
	}
	return false
}

// Bps resolves a basis-points parameter (BR-1.1.3).
func (r *ParamRepo) Bps(ctx context.Context, storeID *uuid.UUID, key string) money.Bps {
	return money.Bps(r.Int(ctx, storeID, key))
}

// Source reports where a value came from, for the admin UI (BR-1.4.4).
func (r *ParamRepo) Source(ctx context.Context, storeID *uuid.UUID, key string) string {
	r.ensure(ctx)
	r.mu.RLock()
	defer r.mu.RUnlock()

	if storeID != nil {
		if byStore, ok := r.stores[*storeID]; ok {
			if _, ok := byStore[key]; ok {
				return "store"
			}
		}
	}
	if _, ok := r.group[key]; ok {
		return "group"
	}
	return "fallback"
}

// ListGroup returns every group parameter, with secrets masked (BR-1.4.3).
func (r *ParamRepo) ListGroup(ctx context.Context, q string) ([]SysParameter, error) {
	query := r.db.WithContext(ctx).Model(&SysParameter{}).Order("key")
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("key ILIKE ? OR description ILIKE ?", like, like)
	}
	var out []SysParameter
	if err := query.Find(&out).Error; err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].IsSecret {
			out[i].Value = "••••••••"
		}
	}
	return out, nil
}

// UpsertGroup writes a group parameter and invalidates the cache. It changes
// only what happens next: order lines keep their snapshots and booked slots
// keep their capacity (BR-2.9.3, BR-2.5.1, BR-2.3.16).
func (r *ParamRepo) UpsertGroup(ctx context.Context, key, value string, actor uuid.UUID) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO sys_parameters (id, key, value, data_type, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, 'string', $4, now(), now())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		uuid.New(), key, value, actor).Error
	if err != nil {
		return err
	}
	r.Invalidate()
	return nil
}

// UpsertStore writes a per-store override and invalidates the cache.
func (r *ParamRepo) UpsertStore(ctx context.Context, storeID uuid.UUID, key, value string, actor uuid.UUID) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO store_parameters (id, store_id, key, value, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now(), now())
		ON CONFLICT (store_id, key) DO UPDATE
		SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		uuid.New(), storeID, key, value, actor).Error
	if err != nil {
		return err
	}
	r.Invalidate()
	return nil
}

// DeleteGroup removes a group parameter (the fallback then applies).
func (r *ParamRepo) DeleteGroup(ctx context.Context, key string) error {
	if err := r.db.WithContext(ctx).Where("key = ?", key).Delete(&SysParameter{}).Error; err != nil {
		return err
	}
	r.Invalidate()
	return nil
}

// DeleteStore removes a per-store override so the group default applies again.
func (r *ParamRepo) DeleteStore(ctx context.Context, storeID uuid.UUID, key string) error {
	if err := r.db.WithContext(ctx).
		Where("store_id = ? AND key = ?", storeID, key).Delete(&StoreParameter{}).Error; err != nil {
		return err
	}
	r.Invalidate()
	return nil
}

// ListStore returns a store's overrides.
func (r *ParamRepo) ListStore(ctx context.Context, storeID uuid.UUID) ([]StoreParameter, error) {
	var out []StoreParameter
	err := r.db.WithContext(ctx).Where("store_id = ?", storeID).Order("key").Find(&out).Error
	return out, err
}
