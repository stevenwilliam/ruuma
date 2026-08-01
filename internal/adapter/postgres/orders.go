package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/order"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/id"
)

// OrderRepo owns the order-creation transaction — the one place where money,
// capacity, stock and a promo redemption all move together (docs/04 §5).
type OrderRepo struct {
	db    *gorm.DB
	slots *SlotRepo
}

func NewOrderRepo(db *gorm.DB, slots *SlotRepo) *OrderRepo {
	return &OrderRepo{db: db, slots: slots}
}

// NewOrderLineOption is one chosen option, already priced and name-snapshotted.
type NewOrderLineOption struct {
	OptionGroupID  uuid.UUID
	OptionChoiceID uuid.UUID
	GroupNameID    string
	ChoiceNameID   string
	ChoiceNameEN   string
	PriceDelta     int64
}

// NewOrderLine is one cart line with its prices already snapshotted by the app
// layer from master data (BR-2.5.1).
type NewOrderLine struct {
	MenuItemID   uuid.UUID
	ItemNameID   string
	ItemNameEN   string
	UnitPrice    int64
	Qty          int
	OptionsDelta int64
	LineTotal    int64
	KitchenUnits int
	Notes        *string
	Options      []NewOrderLineOption
}

// NewOrder is a fully-priced, fully-validated order ready to be written.
type NewOrder struct {
	StoreID          uuid.UUID
	CustomerID       uuid.UUID
	SlotID           uuid.UUID
	FulfilmentType   string
	BusinessDate     time.Time
	SlotStartsAt     time.Time
	SlotEndsAt       time.Time
	ContactName      string
	ContactPhone     string
	Notes            *string
	Subtotal         int64
	Discount         int64
	ServiceCharge    int64
	Tax              int64
	DeliveryFee      int64
	Total            int64
	TaxBps           int
	ServiceChargeBps int
	KitchenUnits     int
	PromotionID      *uuid.UUID
	PromoCode        *string
	BankAccountID    *uuid.UUID
	MaxUnpaid        int // 0 = unlimited (BR-2.3.15)
	Lines            []NewOrderLine
}

// Create writes an order in a single transaction (docs/04 §5):
//
//	unpaid cap → lock slot → reserve capacity → decrement daily stock →
//	allocate kode unik → insert order, lines, options, payment, first event →
//	record the promo redemption.
//
// Any failure rolls back the lot: a customer never loses capacity to a
// half-written order.
func (r *OrderRepo) Create(ctx context.Context, in NewOrder) (*Order, error) {
	var created *Order

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// BR-2.3.15: cap concurrent unpaid orders. With no auto-cancel in phase 1
		// (D25) this is the control that stops slot squatting.
		if in.MaxUnpaid > 0 {
			var unpaid int64
			if err := tx.Model(&Order{}).
				Where("customer_id = ? AND status IN ?", in.CustomerID,
					[]string{string(order.PendingPayment), string(order.AwaitingVerification)}).
				Count(&unpaid).Error; err != nil {
				return err
			}
			if unpaid >= int64(in.MaxUnpaid) {
				return apierror.Unprocessable(apierror.CodeUnpaidLimitReached,
					"You already have unpaid orders waiting. Please complete or cancel them first.").
					WithDetails(map[string]any{"unpaid_orders": unpaid, "limit": in.MaxUnpaid})
			}
		}

		// BR-2.3.8: hold the slot row before touching its counters.
		if _, err := r.slots.LockForUpdate(ctx, tx, in.SlotID); err != nil {
			return err
		}
		if err := r.slots.Reserve(ctx, tx, in.SlotID, in.KitchenUnits); err != nil {
			return err
		}

		// BR-2.2.4: the daily countdown moves inside the same transaction, so a
		// dish cannot be sold twice at the last portion.
		for _, l := range in.Lines {
			res := tx.Exec(`
				UPDATE item_daily_stock
				   SET stock_used = stock_used + $1, updated_at = now()
				 WHERE store_id = $2 AND menu_item_id = $3 AND business_date = $4
				   AND stock_used + $1 <= stock_total`,
				l.Qty, in.StoreID, l.MenuItemID, in.BusinessDate)
			if res.Error != nil {
				if isCheckViolation(res.Error, "item_daily_stock_no_oversell") {
					return apierror.Unprocessable(apierror.CodeItemUnavailable,
						"One of the items has just sold out for that date.")
				}
				return res.Error
			}
			if res.RowsAffected == 0 {
				// No stock row means the item is not counted today; but if a row
				// exists and the update matched nothing, it is sold out.
				var exists int64
				if err := tx.Model(&ItemDailyStock{}).
					Where("store_id = ? AND menu_item_id = ? AND business_date = ?",
						in.StoreID, l.MenuItemID, in.BusinessDate).
					Count(&exists).Error; err != nil {
					return err
				}
				if exists > 0 {
					return apierror.Unprocessable(apierror.CodeItemUnavailable,
						"One of the items has just sold out for that date.").
						WithDetails(map[string]any{"menu_item_id": l.MenuItemID})
				}
			}
		}

		// BR-1.2.2 / BR-2.6.2: order code and kode unik are both CSPRNG values
		// with a uniqueness constraint behind them; retry on the rare collision.
		o := &Order{
			ID:               uuid.New(),
			StoreID:          in.StoreID,
			CustomerID:       in.CustomerID,
			SlotID:           in.SlotID,
			FulfilmentType:   in.FulfilmentType,
			BusinessDate:     in.BusinessDate,
			SlotStartsAt:     in.SlotStartsAt,
			SlotEndsAt:       in.SlotEndsAt,
			Status:           string(order.PendingPayment),
			ContactName:      in.ContactName,
			ContactPhone:     in.ContactPhone,
			Notes:            in.Notes,
			Subtotal:         in.Subtotal,
			Discount:         in.Discount,
			ServiceCharge:    in.ServiceCharge,
			Tax:              in.Tax,
			DeliveryFee:      in.DeliveryFee,
			Total:            in.Total,
			TaxBps:           in.TaxBps,
			ServiceChargeBps: in.ServiceChargeBps,
			PromotionID:      in.PromotionID,
			PromoCode:        in.PromoCode,
			KitchenUnits:     in.KitchenUnits,
			PlacedAt:         ptrTime(time.Now()),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		if err := r.insertWithCodes(ctx, tx, o); err != nil {
			return err
		}

		for _, l := range in.Lines {
			line := OrderLine{
				ID: uuid.New(), OrderID: o.ID, MenuItemID: l.MenuItemID,
				ItemNameID: l.ItemNameID, ItemNameEN: l.ItemNameEN,
				UnitPrice: l.UnitPrice, Qty: l.Qty, OptionsDelta: l.OptionsDelta,
				LineTotal: l.LineTotal, KitchenUnits: l.KitchenUnits, Notes: l.Notes,
				CreatedAt: time.Now(),
			}
			if err := tx.Create(&line).Error; err != nil {
				return err
			}
			for _, opt := range l.Options {
				row := OrderLineOption{
					ID: uuid.New(), OrderLineID: line.ID,
					OptionGroupID: opt.OptionGroupID, OptionChoiceID: opt.OptionChoiceID,
					GroupNameID: opt.GroupNameID, ChoiceNameID: opt.ChoiceNameID,
					ChoiceNameEN: opt.ChoiceNameEN, PriceDelta: opt.PriceDelta,
					CreatedAt: time.Now(),
				}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			}
		}

		// The payment row exists from the start so the finance queue and the
		// customer's upload have somewhere to land (BR-2.6.4).
		pay := Payment{
			ID: uuid.New(), OrderID: o.ID, StoreID: o.StoreID,
			Method: "manual_transfer", Status: "PENDING",
			AmountDue: o.AmountDue, BankAccountID: in.BankAccountID,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := tx.Create(&pay).Error; err != nil {
			return err
		}

		if err := appendOrderEvent(tx, o.ID, nil, order.PendingPayment,
			order.ActorCustomer, &in.CustomerID, "order created", nil); err != nil {
			return err
		}

		// BR-2.5.11: the redemption row and the counter move with the order, so
		// parallel checkouts cannot both spend the last use of a code.
		if in.PromotionID != nil {
			red := PromotionRedemption{
				ID: uuid.New(), PromotionID: *in.PromotionID, OrderID: o.ID,
				CustomerID: o.CustomerID, StoreID: o.StoreID, Discount: o.Discount,
				CreatedAt: time.Now(),
			}
			if err := tx.Create(&red).Error; err != nil {
				return err
			}
			res := tx.Exec(`
				UPDATE promotions
				   SET used_count = used_count + 1, updated_at = now()
				 WHERE id = $1
				   AND (usage_cap_total IS NULL OR used_count + 1 <= usage_cap_total)`,
				*in.PromotionID)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return apierror.Unprocessable(apierror.CodePromoExhausted,
					"That promo code has just reached its usage limit.")
			}
		}

		created = o
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// insertWithCodes assigns an order code and a kode unik, retrying on the unique
// constraints rather than pre-checking — the constraint is the authority.
func (r *OrderRepo) insertWithCodes(ctx context.Context, tx *gorm.DB, o *Order) error {
	const attempts = 8
	var lastErr error
	for i := 0; i < attempts; i++ {
		code, err := id.OrderCode()
		if err != nil {
			return err
		}
		unique, err := id.UniqueCode()
		if err != nil {
			return err
		}
		o.OrderCode = code
		o.UniqueCode = unique
		o.AmountDue = o.Total + int64(unique)

		// A savepoint keeps the outer transaction alive across a retry.
		err = tx.WithContext(ctx).SavePoint(fmt.Sprintf("order_code_%d", i)).Error
		if err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Create(o).Error; err == nil {
			return nil
		} else if isUniqueViolation(err) {
			lastErr = err
			if rbErr := tx.RollbackTo(fmt.Sprintf("order_code_%d", i)).Error; rbErr != nil {
				return rbErr
			}
			continue
		} else {
			return err
		}
	}
	return fmt.Errorf("orders: could not allocate a unique order code or kode unik: %w", lastErr)
}

// Transition moves an order and appends its event, refusing anything the state
// machine does not allow (BR-2.4.2/3/4).
func (r *OrderRepo) Transition(ctx context.Context, orderID uuid.UUID, to order.Status,
	actorType order.ActorType, actorID *uuid.UUID, reason string, scope []uuid.UUID) error {

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var o Order
		if err := scoped(tx.Model(&Order{}), "store_id", scope).
			Where("id = ?", orderID).First(&o).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apierror.NotFound("Order not found.")
			}
			return err
		}

		from := order.Status(o.Status)
		if err := order.Transition(from, to); err != nil {
			return apierror.Conflict(apierror.CodeIllegalTransition,
				"That change is not allowed from the order's current state.").
				WithDetails(map[string]any{"from": string(from), "to": string(to)}).
				WithCause(err)
		}

		updates := map[string]any{"status": string(to), "updated_at": time.Now()}
		if to == order.Cancelled {
			updates["cancelled_reason"] = reason
			updates["cancelled_by"] = actorID
		}
		if err := tx.Model(&Order{}).Where("id = ?", o.ID).Updates(updates).Error; err != nil {
			return err
		}

		// BR-2.3.12: cancellation releases capacity exactly once. The
		// capacity_released_at stamp is what makes a double cancel harmless.
		if to == order.Cancelled && o.CapacityReleasedAt == nil {
			if err := r.slots.Release(ctx, tx, o.SlotID, o.KitchenUnits); err != nil {
				return err
			}
			if err := tx.Model(&Order{}).Where("id = ? AND capacity_released_at IS NULL", o.ID).
				Update("capacity_released_at", time.Now()).Error; err != nil {
				return err
			}
			// BR-2.5.12: a released order returns its promo use.
			if o.PromotionID != nil {
				if err := tx.Exec(`
					UPDATE promotion_redemptions SET released_at = now()
					 WHERE order_id = $1 AND released_at IS NULL`, o.ID).Error; err != nil {
					return err
				}
				if err := tx.Exec(`
					UPDATE promotions SET used_count = GREATEST(used_count - 1, 0), updated_at = now()
					 WHERE id = $1`, *o.PromotionID).Error; err != nil {
					return err
				}
			}
		}

		return appendOrderEvent(tx, o.ID, &from, to, actorType, actorID, reason, nil)
	})
}

// appendOrderEvent writes the append-only history row (BR-2.4.4).
func appendOrderEvent(tx *gorm.DB, orderID uuid.UUID, from *order.Status, to order.Status,
	actorType order.ActorType, actorID *uuid.UUID, reason string, meta map[string]any) error {

	var fromStr *string
	if from != nil {
		s := string(*from)
		fromStr = &s
	}
	var reasonPtr *string
	if strings.TrimSpace(reason) != "" {
		reasonPtr = &reason
	}
	var metaJSON []byte
	if meta != nil {
		b, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		metaJSON = b
	}
	return tx.Create(&OrderEvent{
		ID: uuid.New(), OrderID: orderID, FromStatus: fromStr, ToStatus: string(to),
		ActorType: string(actorType), ActorID: actorID, Reason: reasonPtr,
		Metadata: metaJSON, CreatedAt: time.Now(),
	}).Error
}

// ── Reads ────────────────────────────────────────────────────────────────────

// GetForCustomer reads an order the customer owns. A customer asking for
// someone else's order gets 404, not 403 — existence is not disclosed
// (BR-2.7.10).
func (r *OrderRepo) GetForCustomer(ctx context.Context, orderID, customerID uuid.UUID) (*Order, error) {
	var o Order
	err := r.db.WithContext(ctx).
		Where("id = ? AND customer_id = ?", orderID, customerID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Order not found.")
	}
	return &o, err
}

// GetByCodeForCustomer backs order tracking: the code alone is never enough
// (BR-2.7.11).
func (r *OrderRepo) GetByCodeForCustomer(ctx context.Context, code string, customerID uuid.UUID) (*Order, error) {
	var o Order
	err := r.db.WithContext(ctx).
		Where("order_code = ? AND customer_id = ?", id.NormalizeOrderCode(code), customerID).
		First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Order not found.")
	}
	return &o, err
}

// GetInScope reads an order for staff, bounded by the caller's stores
// (BR-2.7.8).
func (r *OrderRepo) GetInScope(ctx context.Context, orderID uuid.UUID, scope []uuid.UUID) (*Order, error) {
	var o Order
	err := scoped(r.db.WithContext(ctx).Model(&Order{}), "store_id", scope).
		Where("id = ?", orderID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Order not found.")
	}
	return &o, err
}

// ListForCustomer returns a customer's order history, newest first, searchable
// by code (BR-1.5.1).
func (r *OrderRepo) ListForCustomer(ctx context.Context, customerID uuid.UUID, q string, limit int, before *time.Time) ([]Order, error) {
	query := r.db.WithContext(ctx).Model(&Order{}).
		Where("customer_id = ?", customerID).Order("created_at DESC").Limit(limit)
	if q != "" {
		query = query.Where("order_code ILIKE ?", "%"+strings.ToUpper(q)+"%")
	}
	if before != nil {
		query = query.Where("created_at < ?", *before)
	}
	var out []Order
	return out, query.Find(&out).Error
}

// BoardFilter selects orders for the operations board.
type BoardFilter struct {
	StoreID  *uuid.UUID
	Date     *time.Time
	Statuses []string
	Q        string
	Limit    int
}

// Board returns orders grouped-ready for the ops screen, always store-scoped
// (BR-2.8.1).
func (r *OrderRepo) Board(ctx context.Context, f BoardFilter, scope []uuid.UUID) ([]Order, error) {
	query := scoped(r.db.WithContext(ctx).Model(&Order{}), "store_id", scope).
		Order("slot_starts_at, created_at")
	if f.StoreID != nil {
		query = query.Where("store_id = ?", *f.StoreID)
	}
	if f.Date != nil {
		query = query.Where("business_date = ?", *f.Date)
	}
	if len(f.Statuses) > 0 {
		query = query.Where("status IN ?", f.Statuses)
	}
	if f.Q != "" {
		like := "%" + f.Q + "%"
		query = query.Where("order_code ILIKE ? OR contact_name ILIKE ? OR contact_phone ILIKE ?",
			like, like, like)
	}
	if f.Limit > 0 {
		query = query.Limit(f.Limit)
	}
	var out []Order
	return out, query.Find(&out).Error
}

// Unpaid lists ageing unpaid orders oldest-first — the phase-1 control against
// slot squatting (BR-2.3.15, D25).
func (r *OrderRepo) Unpaid(ctx context.Context, storeID *uuid.UUID, scope []uuid.UUID) ([]Order, error) {
	query := scoped(r.db.WithContext(ctx).Model(&Order{}), "store_id", scope).
		Where("status IN ?", []string{string(order.PendingPayment), string(order.AwaitingVerification)}).
		Order("created_at")
	if storeID != nil {
		query = query.Where("store_id = ?", *storeID)
	}
	var out []Order
	return out, query.Find(&out).Error
}

// AffectedByClosure lists live orders on a closed date, for staff to handle by
// hand (BR-2.1.9, D27).
func (r *OrderRepo) AffectedByClosure(ctx context.Context, storeID uuid.UUID, date time.Time, scope []uuid.UUID) ([]Order, error) {
	var out []Order
	return out, scoped(r.db.WithContext(ctx).Model(&Order{}), "store_id", scope).
		Where("store_id = ? AND business_date = ? AND status NOT IN ?",
			storeID, date, []string{"CANCELLED", "REFUNDED", "COMPLETED", "PICKED_UP", "DELIVERED"}).
		Order("slot_starts_at").Find(&out).Error
}

// Lines returns an order's lines with their options.
func (r *OrderRepo) Lines(ctx context.Context, orderID uuid.UUID) ([]OrderLine, map[uuid.UUID][]OrderLineOption, error) {
	var lines []OrderLine
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).
		Order("created_at").Find(&lines).Error; err != nil {
		return nil, nil, err
	}
	ids := make([]uuid.UUID, 0, len(lines))
	for _, l := range lines {
		ids = append(ids, l.ID)
	}
	options := map[uuid.UUID][]OrderLineOption{}
	if len(ids) > 0 {
		var opts []OrderLineOption
		if err := r.db.WithContext(ctx).Where("order_line_id IN ?", ids).Find(&opts).Error; err != nil {
			return nil, nil, err
		}
		for _, o := range opts {
			options[o.OrderLineID] = append(options[o.OrderLineID], o)
		}
	}
	return lines, options, nil
}

// Events returns an order's append-only history (BR-2.4.4).
func (r *OrderRepo) Events(ctx context.Context, orderID uuid.UUID) ([]OrderEvent, error) {
	var out []OrderEvent
	return out, r.db.WithContext(ctx).Where("order_id = ?", orderID).
		Order("created_at").Find(&out).Error
}

// ProductionLine is one aggregated row of the kitchen summary (BR-2.8.2).
type ProductionLine struct {
	MenuItemID  uuid.UUID `gorm:"column:menu_item_id"`
	ItemName    string    `gorm:"column:item_name"`
	OptionName  *string   `gorm:"column:option_name"`
	Qty         int       `gorm:"column:qty"`
	PrepMinutes int       `gorm:"column:prep_minutes"`
}

// ProductionSummary aggregates quantities per item and per selected option for
// one slot, ordered longest-prep-first so the kitchen starts the right dish
// (BR-2.8.2, BR-2.8.3). Cancelled orders are excluded.
func (r *OrderRepo) ProductionSummary(ctx context.Context, slotID uuid.UUID, scope []uuid.UUID) ([]ProductionLine, error) {
	var scopeArg any = nil
	if scope != nil {
		scopeArg = uuidList(scope)
	}
	var out []ProductionLine
	err := r.db.WithContext(ctx).Raw(`
		SELECT ol.menu_item_id,
		       ol.item_name_id                AS item_name,
		       olo.choice_name_id             AS option_name,
		       SUM(ol.qty)::int               AS qty,
		       MAX(mi.prep_minutes)::int      AS prep_minutes
		  FROM order_lines ol
		  JOIN orders o      ON o.id = ol.order_id
		  JOIN menu_items mi ON mi.id = ol.menu_item_id
		  LEFT JOIN order_line_options olo ON olo.order_line_id = ol.id
		 WHERE o.slot_id = $1
		   AND o.status NOT IN ('CANCELLED','REFUNDED','DRAFT','PENDING_PAYMENT','AWAITING_VERIFICATION')
		   AND ($2::uuid[] IS NULL OR o.store_id = ANY($2::uuid[]))
		 GROUP BY ol.menu_item_id, ol.item_name_id, olo.choice_name_id
		 ORDER BY prep_minutes DESC, item_name`, slotID, scopeArg).Scan(&out).Error
	return out, err
}

func uuidList(in []uuid.UUID) string {
	parts := make([]string, len(in))
	for i, u := range in {
		parts[i] = u.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func ptrTime(t time.Time) *time.Time { return &t }

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "SQLSTATE 23505")
}
