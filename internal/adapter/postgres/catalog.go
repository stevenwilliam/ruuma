package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/catalog"
	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/clock"
)

// CatalogRepo reads the menu and resolves it against one store (BR-2.2.x).
type CatalogRepo struct {
	db *gorm.DB
}

func NewCatalogRepo(db *gorm.DB) *CatalogRepo { return &CatalogRepo{db: db} }

// ResolvedItem is a menu item as one store sells it today.
type ResolvedItem struct {
	Item         MenuItem
	Category     Category
	Price        money.Rupiah
	Availability catalog.Availability
	StockLeft    *int
}

// MenuFilter is the customer-facing menu query (docs/04 §3).
type MenuFilter struct {
	StoreID    uuid.UUID
	Q          string
	CategoryID *uuid.UUID
	Cuisine    string
	Diet       string // halal | vegetarian | no_pork | no_alcohol | no_nuts
	Sort       string // name | price_asc | price_desc
	Limit      int
	Offset     int
}

// Menu returns the store-resolved menu. Sold-out items are returned with their
// state rather than hidden — a menu that silently loses dishes reads as broken
// (docs/10 §4.2).
func (r *CatalogRepo) Menu(ctx context.Context, f MenuFilter, now time.Time) ([]ResolvedItem, error) {
	query := r.db.WithContext(ctx).Model(&MenuItem{}).
		Joins("JOIN categories c ON c.id = menu_items.category_id").
		Where("menu_items.is_active AND c.is_active")

	if f.Q != "" {
		like := "%" + strings.TrimSpace(f.Q) + "%"
		query = query.Where(`menu_items.name_id ILIKE ? OR menu_items.name_en ILIKE ?
			OR menu_items.description_id ILIKE ? OR menu_items.description_en ILIKE ?`,
			like, like, like, like)
	}
	if f.CategoryID != nil {
		query = query.Where("menu_items.category_id = ?", *f.CategoryID)
	}
	if f.Cuisine != "" {
		query = query.Where("c.cuisine = ?", f.Cuisine)
	}
	switch f.Diet {
	case "halal":
		query = query.Where("menu_items.is_halal")
	case "vegetarian":
		query = query.Where("menu_items.is_vegetarian")
	case "no_pork":
		query = query.Where("NOT menu_items.contains_pork")
	case "no_alcohol":
		query = query.Where("NOT menu_items.contains_alcohol")
	case "no_nuts":
		query = query.Where("NOT menu_items.contains_nuts")
	}
	switch f.Sort {
	case "price_asc":
		query = query.Order("menu_items.base_price, menu_items.name_id")
	case "price_desc":
		query = query.Order("menu_items.base_price DESC, menu_items.name_id")
	default:
		query = query.Order("c.sort_order, menu_items.sort_order, menu_items.name_id")
	}
	if f.Limit > 0 {
		query = query.Limit(f.Limit).Offset(f.Offset)
	}

	var items []MenuItem
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return r.resolve(ctx, f.StoreID, items, now, nil)
}

// Item returns one item resolved for a store.
func (r *CatalogRepo) Item(ctx context.Context, storeID, itemID uuid.UUID, now time.Time) (*ResolvedItem, error) {
	var item MenuItem
	err := r.db.WithContext(ctx).First(&item, "id = ?", itemID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Menu item not found.")
	}
	if err != nil {
		return nil, err
	}
	out, err := r.resolve(ctx, storeID, []MenuItem{item}, now, nil)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, apierror.NotFound("Menu item not found.")
	}
	return &out[0], nil
}

// resolve applies store overrides, 86s, daily stock and availability rules to a
// set of items (BR-2.2.1 → BR-2.2.4, BR-2.2.7).
func (r *CatalogRepo) resolve(ctx context.Context, storeID uuid.UUID, items []MenuItem,
	now time.Time, slotStartLocal *time.Time) ([]ResolvedItem, error) {

	if len(items) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(items))
	catIDs := map[uuid.UUID]bool{}
	for _, i := range items {
		ids = append(ids, i.ID)
		catIDs[i.CategoryID] = true
	}

	var categories []Category
	if err := r.db.WithContext(ctx).Find(&categories, "id IN ?", keys(catIDs)).Error; err != nil {
		return nil, err
	}
	catByID := map[uuid.UUID]Category{}
	for _, c := range categories {
		catByID[c.ID] = c
	}

	var overrides []StoreMenuOverride
	if err := r.db.WithContext(ctx).
		Where("store_id = ? AND menu_item_id IN ?", storeID, ids).Find(&overrides).Error; err != nil {
		return nil, err
	}
	overrideByItem := map[uuid.UUID]StoreMenuOverride{}
	for _, o := range overrides {
		overrideByItem[o.MenuItemID] = o
	}

	var eighties []Item86
	if err := r.db.WithContext(ctx).
		Where("store_id = ? AND menu_item_id IN ? AND (ends_at IS NULL OR ends_at > ?)",
			storeID, ids, now).Find(&eighties).Error; err != nil {
		return nil, err
	}
	eightyByItem := map[uuid.UUID][]catalog.EightySix{}
	for _, e := range eighties {
		eightyByItem[e.MenuItemID] = append(eightyByItem[e.MenuItemID],
			catalog.EightySix{StartsAt: e.StartsAt, EndsAt: e.EndsAt})
	}

	businessDate := now.UTC().Truncate(24 * time.Hour)
	if slotStartLocal != nil {
		businessDate = time.Date(slotStartLocal.Year(), slotStartLocal.Month(), slotStartLocal.Day(),
			0, 0, 0, 0, time.UTC)
	}
	var stocks []ItemDailyStock
	if err := r.db.WithContext(ctx).
		Where("store_id = ? AND menu_item_id IN ? AND business_date = ?", storeID, ids, businessDate).
		Find(&stocks).Error; err != nil {
		return nil, err
	}
	stockByItem := map[uuid.UUID]catalog.DailyStock{}
	for _, s := range stocks {
		stockByItem[s.MenuItemID] = catalog.DailyStock{Total: s.StockTotal, Used: s.StockUsed}
	}

	var rules []ItemAvailabilityRule
	if err := r.db.WithContext(ctx).
		Where("menu_item_id IN ? AND (store_id IS NULL OR store_id = ?)", ids, storeID).
		Find(&rules).Error; err != nil {
		return nil, err
	}
	rulesByItem := map[uuid.UUID][]catalog.AvailabilityRule{}
	for _, rule := range rules {
		dr := catalog.AvailabilityRule{WeekdayMask: rule.WeekdayMask}
		if rule.FromTime != nil {
			if t, err := parseTimeOfDay(*rule.FromTime); err == nil {
				dr.FromTime = &t
			}
		}
		if rule.ToTime != nil {
			if t, err := parseTimeOfDay(*rule.ToTime); err == nil {
				dr.ToTime = &t
			}
		}
		rulesByItem[rule.MenuItemID] = append(rulesByItem[rule.MenuItemID], dr)
	}

	out := make([]ResolvedItem, 0, len(items))
	for _, i := range items {
		cat := catByID[i.CategoryID]
		domainItem := catalog.Item{
			ID: i.ID.String(), CategoryID: i.CategoryID.String(),
			CategoryActive: cat.IsActive, IsActive: i.IsActive,
			BasePrice: money.Rupiah(i.BasePrice), KitchenUnits: i.KitchenUnits,
			PrepMinutes: i.PrepMinutes, MinLeadMinutes: i.MinLeadMinutes,
		}

		var domainOverride *catalog.StoreOverride
		if o, ok := overrideByItem[i.ID]; ok {
			domainOverride = &catalog.StoreOverride{IsAvailable: o.IsAvailable}
			if o.PriceOverride != nil {
				p := money.Rupiah(*o.PriceOverride)
				domainOverride.PriceOverride = &p
			}
		}

		var stock *catalog.DailyStock
		var left *int
		if s, ok := stockByItem[i.ID]; ok {
			stock = &s
			remaining := s.Remaining()
			left = &remaining
		}

		q := catalog.Query{
			Item: domainItem, Override: domainOverride,
			EightySixs: eightyByItem[i.ID], Stock: stock, Rules: rulesByItem[i.ID],
			Qty: 1, SlotStartLocal: slotStartLocal, Now: now,
		}

		out = append(out, ResolvedItem{
			Item:         i,
			Category:     cat,
			Price:        catalog.EffectivePrice(domainItem, domainOverride),
			Availability: catalog.Resolve(q),
			StockLeft:    left,
		})
	}
	return out, nil
}

// ResolveForSlot resolves items against a specific slot, so item-level
// availability rules narrow the bookable set (BR-2.2.7).
func (r *CatalogRepo) ResolveForSlot(ctx context.Context, storeID uuid.UUID, itemIDs []uuid.UUID,
	now time.Time, slotStartLocal time.Time) ([]ResolvedItem, error) {

	var items []MenuItem
	if err := r.db.WithContext(ctx).Find(&items, "id IN ?", itemIDs).Error; err != nil {
		return nil, err
	}
	if len(items) != len(itemIDs) {
		return nil, apierror.Unprocessable(apierror.CodeItemUnavailable,
			"One of the items is no longer on the menu.")
	}
	return r.resolve(ctx, storeID, items, now, &slotStartLocal)
}

// OptionsFor loads an item's option groups and choices (BR-2.2.5).
func (r *CatalogRepo) OptionsFor(ctx context.Context, itemID uuid.UUID) ([]catalog.OptionGroup, map[uuid.UUID]OptionGroup, map[uuid.UUID]OptionChoice, error) {
	var groups []OptionGroup
	if err := r.db.WithContext(ctx).Where("menu_item_id = ?", itemID).
		Order("sort_order").Find(&groups).Error; err != nil {
		return nil, nil, nil, err
	}
	if len(groups) == 0 {
		return nil, map[uuid.UUID]OptionGroup{}, map[uuid.UUID]OptionChoice{}, nil
	}

	groupIDs := make([]uuid.UUID, 0, len(groups))
	groupByID := map[uuid.UUID]OptionGroup{}
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
		groupByID[g.ID] = g
	}

	var choices []OptionChoice
	if err := r.db.WithContext(ctx).Where("option_group_id IN ?", groupIDs).
		Order("sort_order").Find(&choices).Error; err != nil {
		return nil, nil, nil, err
	}
	choiceByID := map[uuid.UUID]OptionChoice{}
	byGroup := map[uuid.UUID][]catalog.OptionChoice{}
	for _, c := range choices {
		choiceByID[c.ID] = c
		byGroup[c.OptionGroupID] = append(byGroup[c.OptionGroupID], catalog.OptionChoice{
			ID: c.ID.String(), PriceDelta: money.Rupiah(c.PriceDelta),
			KitchenUnits: c.KitchenUnits, IsAvailable: c.IsAvailable,
		})
	}

	domainGroups := make([]catalog.OptionGroup, 0, len(groups))
	for _, g := range groups {
		domainGroups = append(domainGroups, catalog.OptionGroup{
			ID: g.ID.String(), Selection: catalog.Selection(g.Selection),
			IsRequired: g.IsRequired, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect,
			Choices: byGroup[g.ID],
		})
	}
	return domainGroups, groupByID, choiceByID, nil
}

// Categories lists categories, searchable (BR-1.5.1).
func (r *CatalogRepo) Categories(ctx context.Context, q string, activeOnly bool) ([]Category, error) {
	query := r.db.WithContext(ctx).Model(&Category{}).Order("sort_order, name_id")
	if activeOnly {
		query = query.Where("is_active")
	}
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("name_id ILIKE ? OR name_en ILIKE ? OR slug ILIKE ?", like, like, like)
	}
	var out []Category
	return out, query.Find(&out).Error
}

// ── Admin writes ─────────────────────────────────────────────────────────────

func (r *CatalogRepo) CreateCategory(ctx context.Context, c *Category) error {
	c.ID, c.CreatedAt, c.UpdatedAt = uuid.New(), time.Now(), time.Now()
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *CatalogRepo) UpdateCategory(ctx context.Context, c *Category) error {
	c.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *CatalogRepo) CreateItem(ctx context.Context, i *MenuItem) error {
	i.ID, i.CreatedAt, i.UpdatedAt = uuid.New(), time.Now(), time.Now()
	return r.db.WithContext(ctx).Create(i).Error
}

func (r *CatalogRepo) UpdateItem(ctx context.Context, i *MenuItem) error {
	i.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(i).Error
}

func (r *CatalogRepo) GetItem(ctx context.Context, id uuid.UUID) (*MenuItem, error) {
	var i MenuItem
	err := r.db.WithContext(ctx).First(&i, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Menu item not found.")
	}
	return &i, err
}

// SetStoreOverride writes a store's price/availability opinion (BR-2.2.1).
func (r *CatalogRepo) SetStoreOverride(ctx context.Context, storeID, itemID uuid.UUID,
	available *bool, price *int64, actor uuid.UUID) error {

	return r.db.WithContext(ctx).Exec(`
		INSERT INTO store_menu_overrides
			(id, store_id, menu_item_id, is_available, price_override, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6, now(), now())
		ON CONFLICT (store_id, menu_item_id) DO UPDATE
		SET is_available = EXCLUDED.is_available, price_override = EXCLUDED.price_override,
		    updated_by = EXCLUDED.updated_by, updated_at = now()`,
		uuid.New(), storeID, itemID, available, price, actor).Error
}

// Add86 marks an item out of stock at a store (BR-2.2.3).
func (r *CatalogRepo) Add86(ctx context.Context, storeID, itemID uuid.UUID, until *time.Time, reason string, actor uuid.UUID) error {
	row := Item86{
		ID: uuid.New(), StoreID: storeID, MenuItemID: itemID,
		StartsAt: time.Now(), EndsAt: until, CreatedBy: &actor, CreatedAt: time.Now(),
	}
	if reason != "" {
		row.Reason = &reason
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

// Lift86 ends every active 86 for an item at a store.
func (r *CatalogRepo) Lift86(ctx context.Context, storeID, itemID uuid.UUID) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE item_86s SET ends_at = now()
		 WHERE store_id = $1 AND menu_item_id = $2 AND (ends_at IS NULL OR ends_at > now())`,
		storeID, itemID).Error
}

// SetDailyStock sets today's countdown for an item at a store (BR-2.2.4).
func (r *CatalogRepo) SetDailyStock(ctx context.Context, storeID, itemID uuid.UUID, date time.Time, total int) error {
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO item_daily_stock (id, store_id, menu_item_id, business_date, stock_total, stock_used, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,0, now(), now())
		ON CONFLICT (store_id, menu_item_id, business_date) DO UPDATE
		SET stock_total = GREATEST(EXCLUDED.stock_total, item_daily_stock.stock_used), updated_at = now()`,
		uuid.New(), storeID, itemID, date, total).Error
}

// MaxItemLead returns the strictest item lead time in a cart (BR-2.2.7).
func (r *CatalogRepo) MaxItemLead(ctx context.Context, itemIDs []uuid.UUID) (int, error) {
	var lead *int
	err := r.db.WithContext(ctx).Model(&MenuItem{}).
		Where("id IN ?", itemIDs).Select("MAX(min_lead_minutes)").Scan(&lead).Error
	if err != nil || lead == nil {
		return 0, err
	}
	return *lead, nil
}

// LocalTime converts an instant into a store's local wall-clock time, which is
// what availability rules are written against (BR-1.3.2).
func LocalTime(t time.Time, tz string) time.Time { return t.In(clock.Location(tz)) }

func keys(m map[uuid.UUID]bool) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
