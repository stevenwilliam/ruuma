// Package ordersvc prices carts and creates orders (docs/04 §5).
//
// Pricing is always server-side: the client's total is compared and never
// trusted (BR-2.5.13), and every price on an order is a snapshot taken at
// creation (BR-2.5.1).
package ordersvc

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/notifysvc"
	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/catalog"
	"github.com/stevenwilliam/ruuma/internal/domain/identity"
	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/domain/order"
	"github.com/stevenwilliam/ruuma/internal/domain/pricing"
	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/clock"
)

type Service struct {
	composer   *notifysvc.Composer
	stores     ports.Stores
	catalogue  ports.Catalogue
	slots      ports.Slots
	orders     ports.Orders
	payments   ports.Payments
	promotions ports.Promotions
	customers  ports.Customers
	params     ports.Params
	audit      ports.Auditor
	clock      ports.Clock
}

func New(stores ports.Stores, catalogue ports.Catalogue, slots ports.Slots, orders ports.Orders,
	payments ports.Payments, promotions ports.Promotions, customers ports.Customers,
	params ports.Params, audit ports.Auditor, notifier ports.Notifier, clk ports.Clock) *Service {
	return &Service{
		stores: stores, catalogue: catalogue, slots: slots, orders: orders,
		payments: payments, promotions: promotions, customers: customers,
		params: params, audit: audit, clock: clk,
		composer: notifysvc.NewComposer(params, notifier),
	}
}

// notifyFailed records a message that could not be queued. It is written to the
// audit log rather than dropped, because "the customer was never told" is an
// operational fact somebody has to be able to find.
func (s *Service) notifyFailed(ctx context.Context, orderID uuid.UUID, cause error) {
	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "system", Action: "notify.queue.failed",
		EntityType: "order", EntityID: &orderID,
		After: map[string]any{"error": cause.Error()},
	})
}

// CartLine is one line a customer asked for.
type CartLine struct {
	MenuItemID      uuid.UUID
	Qty             int
	Notes           string
	OptionChoiceIDs []uuid.UUID
}

// CartRequest is a cart to price or to order.
type CartRequest struct {
	StoreID        uuid.UUID
	CustomerID     uuid.UUID
	FulfilmentType schedule.FulfilmentType
	SlotID         *uuid.UUID
	PromoCode      string
	Lines          []CartLine
}

// Quote is the server's pricing of a cart (BR-2.5.x).
type Quote struct {
	Totals           pricing.Totals
	TaxBps           money.Bps
	ServiceChargeBps money.Bps
	KitchenUnits     int
	Lines            []ports.NewOrderLineInput
	PromotionID      *uuid.UUID
	PromoReason      pricing.PromoReason
	Warnings         []Warning
	ExpiresAt        time.Time
}

// Warning tells the customer what changed under them — an item that sold out
// while they browsed, for instance.
type Warning struct {
	Code       string
	MenuItemID uuid.UUID
	Message    string
}

// Quote prices a cart from master data. It never trusts a client-sent price.
func (s *Service) Quote(ctx context.Context, req CartRequest) (*Quote, error) {
	if len(req.Lines) == 0 {
		return nil, apierror.Validation("Your cart is empty.", nil)
	}
	now := s.clock.Now()

	st, err := s.stores.Get(ctx, req.StoreID)
	if err != nil {
		return nil, err
	}
	loc := clock.Location(st.Timezone)

	slotStartLocal := now.In(loc)
	if req.SlotID != nil {
		slot, err := s.slots.Get(ctx, *req.SlotID)
		if err != nil {
			return nil, err
		}
		if slot.StoreID != req.StoreID {
			return nil, apierror.Unprocessable(apierror.CodeValidation,
				"That time slot belongs to a different store.")
		}
		slotStartLocal = slot.State.StartsAt.In(loc)
	}

	itemIDs := make([]uuid.UUID, 0, len(req.Lines))
	for _, l := range req.Lines {
		itemIDs = append(itemIDs, l.MenuItemID)
	}
	resolved, err := s.catalogue.ResolveForSlot(ctx, req.StoreID, itemIDs, now, slotStartLocal)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]ports.MenuItemView{}
	for _, r := range resolved {
		byID[r.ID] = r
	}

	q := &Quote{}
	domainLines := make([]pricing.Line, 0, len(req.Lines))

	for _, l := range req.Lines {
		if l.Qty <= 0 {
			return nil, apierror.Validation("Quantity must be at least one.",
				map[string]any{"menu_item_id": l.MenuItemID})
		}
		item, ok := byID[l.MenuItemID]
		if !ok {
			return nil, apierror.Unprocessable(apierror.CodeItemUnavailable,
				"One of the items is no longer on the menu.")
		}
		if item.Availability != catalog.Available {
			return nil, apierror.Unprocessable(apierror.CodeItemUnavailable,
				availabilityMessage(item.Availability)).
				WithDetails(map[string]any{
					"menu_item_id": l.MenuItemID, "reason": string(item.Availability),
				})
		}

		groups, err := s.catalogue.Options(ctx, l.MenuItemID)
		if err != nil {
			return nil, err
		}
		domainGroups, groupName, choiceName := toDomainOptions(groups)

		selected := make([]string, 0, len(l.OptionChoiceIDs))
		for _, id := range l.OptionChoiceIDs {
			selected = append(selected, id.String())
		}
		result, err := catalog.ValidateOptions(domainGroups, selected)
		if err != nil {
			return nil, apierror.Unprocessable(apierror.CodeOptionInvalid, err.Error()).
				WithDetails(map[string]any{"menu_item_id": l.MenuItemID})
		}

		domainLines = append(domainLines, pricing.Line{
			MenuItemID: item.ID.String(), CategoryID: item.CategoryID.String(),
			UnitPrice: item.Price, OptionsDelta: result.Delta, Qty: l.Qty,
			KitchenUnits: item.KitchenUnits + result.KitchenUnits,
		})

		lineTotal, err := (pricing.Line{
			UnitPrice: item.Price, OptionsDelta: result.Delta, Qty: l.Qty,
		}).LineTotal()
		if err != nil {
			return nil, apierror.Unprocessable(apierror.CodeValidation, "That combination cannot be priced.")
		}

		line := ports.NewOrderLineInput{
			MenuItemID: item.ID, ItemNameID: item.NameID, ItemNameEN: item.NameEN,
			UnitPrice: item.Price, Qty: l.Qty, OptionsDelta: result.Delta,
			LineTotal: lineTotal, KitchenUnits: (item.KitchenUnits + result.KitchenUnits) * l.Qty,
			Notes: l.Notes,
		}
		for _, id := range l.OptionChoiceIDs {
			g, c := groupName[id], choiceName[id]
			line.Options = append(line.Options, ports.NewOrderLineOptionInput{
				OptionGroupID: g.ID, OptionChoiceID: id,
				GroupNameID: g.NameID, ChoiceNameID: c.NameID, ChoiceNameEN: c.NameEN,
				PriceDelta: c.PriceDelta,
			})
		}
		q.Lines = append(q.Lines, line)
	}

	// Promotion (BR-2.5.9).
	var discount money.Rupiah
	if req.PromoCode != "" {
		promoID, promo, err := s.promotions.ByCode(ctx, req.PromoCode, req.CustomerID)
		if err != nil {
			return nil, err
		}
		d, reason := pricing.EvaluatePromotion(promo, pricing.PromoContext{
			StoreID: req.StoreID.String(), Lines: domainLines, Now: now,
		})
		q.PromoReason = reason
		if reason != pricing.PromoOK {
			return nil, apierror.Unprocessable(apierror.CodePromoInvalid, promoMessage(reason)).
				WithDetails(map[string]any{"reason": string(reason)})
		}
		discount = d
		q.PromotionID = &promoID
	}

	cfg := pricing.Config{
		TaxBps:           s.params.Bps(ctx, &req.StoreID, "pricing.tax_bps"),
		ServiceChargeBps: s.params.Bps(ctx, &req.StoreID, "pricing.service_charge_bps"),
		TaxInclusive:     s.params.Bool(ctx, &req.StoreID, "pricing.tax_inclusive"),
	}
	totals, err := pricing.Compute(domainLines, discount, 0, cfg)
	if err != nil {
		return nil, apierror.Unprocessable(apierror.CodeValidation, "That cart cannot be priced.")
	}

	q.Totals = totals
	q.TaxBps, q.ServiceChargeBps = cfg.TaxBps, cfg.ServiceChargeBps
	q.KitchenUnits = pricing.KitchenUnits(domainLines)
	// A quote is valid for pricing.quote_ttl_minutes; an order created against a
	// stale one is re-priced and re-checked at creation (BR-2.5.14, BR-2.5.13).
	q.ExpiresAt = now.Add(time.Duration(s.params.Int(ctx, nil, "pricing.quote_ttl_minutes")) * time.Minute)
	return q, nil
}

// CreateRequest is a checkout.
type CreateRequest struct {
	CartRequest
	SlotID        uuid.UUID
	ContactName   string
	ContactPhone  string
	Notes         string
	ExpectedTotal *money.Rupiah
}

// Create books the slot and writes the order (docs/04 §5). Everything the
// customer sent is re-derived server-side first.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*ports.OrderView, error) {
	now := s.clock.Now()

	// BR-2.7.1/4: registered customers only, and a verified phone before the
	// counter can be expected to reach them.
	cust, err := s.customers.Get(ctx, req.CustomerID)
	if err != nil {
		return nil, err
	}
	if err := identity.CanPlaceOrder(cust.PhoneVerifiedAt); err != nil {
		return nil, apierror.Unprocessable(apierror.CodePhoneVerificationRequired,
			"Please verify your phone number before ordering.")
	}

	st, err := s.stores.Get(ctx, req.StoreID)
	if err != nil {
		return nil, err
	}
	loc := clock.Location(st.Timezone)

	slot, err := s.slots.Get(ctx, req.SlotID)
	if err != nil {
		return nil, err
	}
	if slot.StoreID != req.StoreID {
		return nil, apierror.Unprocessable(apierror.CodeValidation,
			"That time slot belongs to a different store.")
	}

	sched, err := s.stores.LoadSchedule(ctx, req.StoreID, slot.State.Date, slot.State.Date)
	if err != nil {
		return nil, err
	}
	group := schedule.Group{DeliveryEnabled: s.params.Bool(ctx, nil, "fulfilment.delivery_enabled")}

	cart := req.CartRequest
	cart.SlotID = &req.SlotID
	quote, err := s.Quote(ctx, cart)
	if err != nil {
		return nil, err
	}

	// BR-2.5.13: the client's total is compared, never trusted.
	if req.ExpectedTotal != nil && *req.ExpectedTotal != quote.Totals.Total {
		return nil, apierror.Unprocessable(apierror.CodeTotalMismatch,
			"The price has changed since you opened checkout. Please review your order.").
			WithDetails(map[string]any{
				"expected": int64(*req.ExpectedTotal), "actual": int64(quote.Totals.Total),
			})
	}

	// BR-2.3.5: re-check bookability at the moment of ordering, with this
	// cart's weight and its items' lead times.
	itemIDs := make([]uuid.UUID, 0, len(req.Lines))
	for _, l := range req.Lines {
		itemIDs = append(itemIDs, l.MenuItemID)
	}
	resolved, err := s.catalogue.ResolveForSlot(ctx, req.StoreID, itemIDs, now, slot.State.StartsAt.In(loc))
	if err != nil {
		return nil, err
	}
	itemLead, blocked := 0, false
	for _, item := range resolved {
		if item.MinLeadMinutes > itemLead {
			itemLead = item.MinLeadMinutes
		}
		if item.Availability != catalog.Available {
			blocked = true
		}
	}

	reason := schedule.Bookable(sched, group, slot.State, schedule.Request{
		KitchenUnits: quote.KitchenUnits, ItemLeadMinutes: itemLead, ItemBlocked: blocked,
	}, now)
	if reason != schedule.ReasonOK {
		return nil, slotError(reason)
	}

	bank, err := s.stores.PrimaryBankAccount(ctx, req.StoreID)
	if err != nil {
		return nil, err
	}

	in := ports.NewOrderInput{
		StoreID: req.StoreID, CustomerID: req.CustomerID, SlotID: req.SlotID,
		FulfilmentType: req.FulfilmentType,
		BusinessDate:   slot.State.Date.Time(schedule.TimeOfDay{}, time.UTC),
		SlotStartsAt:   slot.State.StartsAt, SlotEndsAt: slot.State.EndsAt,
		ContactName: req.ContactName, ContactPhone: req.ContactPhone, Notes: req.Notes,
		Totals: quote.Totals, TaxBps: quote.TaxBps, ServiceChargeBps: quote.ServiceChargeBps,
		KitchenUnits: quote.KitchenUnits, PromotionID: quote.PromotionID,
		PromoCode: req.PromoCode, BankAccountID: &bank.ID,
		MaxUnpaid: s.params.Int(ctx, nil, "orders.max_unpaid_per_customer"),
		Lines:     quote.Lines,
	}

	created, err := s.orders.Create(ctx, in)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "customer", ActorID: &req.CustomerID, Action: "order.create",
		EntityType: "order", EntityID: &created.ID, StoreID: &req.StoreID,
		After: map[string]any{"order_code": created.OrderCode, "total": int64(created.Total)},
	})

	// The customer is told what to transfer, to which account, and that the
	// amount includes their kode unik (BR-2.10.3, BR-2.6.2). A failure here is
	// logged by the caller, not fatal: the order exists and the same details
	// are on the order page.
	if err := s.composer.Queue(ctx, notifysvc.EventOrderReceived, *created,
		st.Name, st.AddressLine, notifysvc.Bank{
			Name: bank.BankName, AccountName: bank.AccountName, AccountNumber: bank.AccountNumber,
		}); err != nil {
		s.notifyFailed(ctx, created.ID, err)
	}

	return created, nil
}

// Cancel lets a customer cancel inside their window (BR-2.3.13).
func (s *Service) Cancel(ctx context.Context, orderID, customerID uuid.UUID, reason string) error {
	o, err := s.orders.GetForCustomer(ctx, orderID, customerID)
	if err != nil {
		return err
	}
	if !order.CustomerCancellable(o.Status) {
		return apierror.Conflict(apierror.CodeIllegalTransition,
			"This order can no longer be cancelled online. Please call the store.")
	}

	st, err := s.stores.Get(ctx, o.StoreID)
	if err != nil {
		return err
	}
	sched, err := s.stores.LoadSchedule(ctx, o.StoreID,
		schedule.DateOf(o.SlotStartsAt, clock.Location(st.Timezone)),
		schedule.DateOf(o.SlotStartsAt, clock.Location(st.Timezone)))
	if err != nil {
		return err
	}
	if !schedule.CancellableByCustomer(sched, o.SlotStartsAt, s.clock.Now()) {
		return apierror.Conflict(apierror.CodeIllegalTransition,
			"It is too close to your pickup time to cancel online. Please call the store.")
	}

	if err := s.orders.Transition(ctx, orderID, order.Cancelled,
		order.ActorCustomer, &customerID, reason, nil); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "customer", ActorID: &customerID, Action: "order.cancel",
		EntityType: "order", EntityID: &orderID, StoreID: &o.StoreID,
		After: map[string]any{"reason": reason},
	})
}

// History returns a customer's past orders (BR-1.5.1).
func (s *Service) History(ctx context.Context, customerID uuid.UUID, q string, limit int) ([]ports.OrderView, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.orders.ListForCustomer(ctx, customerID, q, limit, nil)
}

// Get returns one of the customer's own orders with its history and payment.
func (s *Service) Get(ctx context.Context, orderID, customerID uuid.UUID) (*ports.OrderView, []ports.OrderEventView, *ports.PaymentView, error) {
	o, err := s.orders.GetForCustomer(ctx, orderID, customerID)
	if err != nil {
		return nil, nil, nil, err
	}
	events, err := s.orders.Events(ctx, orderID)
	if err != nil {
		return nil, nil, nil, err
	}
	pay, err := s.payments.ForOrder(ctx, orderID)
	if err != nil {
		return nil, nil, nil, err
	}
	return o, events, pay, nil
}

// Track resolves an order by code for its owner. The code alone is never enough
// (BR-2.7.11).
func (s *Service) Track(ctx context.Context, code string, customerID uuid.UUID) (*ports.OrderView, []ports.OrderEventView, *ports.PaymentView, error) {
	o, err := s.orders.GetByCodeForCustomer(ctx, code, customerID)
	if err != nil {
		return nil, nil, nil, err
	}
	return s.Get(ctx, o.ID, customerID)
}

// ReorderResult is a past order revalidated against today's menu.
type ReorderResult struct {
	Lines    []CartLine
	Warnings []Warning
}

// Reorder rebuilds a cart from a past order, dropping what is no longer
// available and saying so (docs/01 §3.1).
func (s *Service) Reorder(ctx context.Context, orderID, customerID uuid.UUID) (*ReorderResult, error) {
	o, err := s.orders.GetForCustomer(ctx, orderID, customerID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()

	// The whole store menu is resolved once: a reorder has to know both what
	// is still on the menu and what has quietly become unavailable.
	resolved, err := s.catalogue.Menu(ctx, ports.MenuQuery{StoreID: o.StoreID, Limit: 500}, now)
	if err != nil {
		return nil, err
	}
	available := map[uuid.UUID]ports.MenuItemView{}
	for _, r := range resolved {
		available[r.ID] = r
	}

	out := &ReorderResult{}
	for _, l := range o.Lines {
		item, ok := available[l.MenuItemID]
		if !ok {
			out.Warnings = append(out.Warnings, Warning{
				Code: "ITEM_REMOVED", MenuItemID: l.MenuItemID,
				Message: l.ItemNameID + " is no longer on the menu.",
			})
			continue
		}
		if item.Availability != catalog.Available {
			out.Warnings = append(out.Warnings, Warning{
				Code: "ITEM_UNAVAILABLE", MenuItemID: l.MenuItemID,
				Message: l.ItemNameID + ": " + availabilityMessage(item.Availability),
			})
			continue
		}
		if item.Price != l.UnitPrice {
			out.Warnings = append(out.Warnings, Warning{
				Code: "PRICE_CHANGED", MenuItemID: l.MenuItemID,
				Message: l.ItemNameID + " has a new price.",
			})
		}
		choices := make([]uuid.UUID, 0, len(l.Options))
		for _, opt := range l.Options {
			choices = append(choices, opt.OptionChoiceID)
		}
		out.Lines = append(out.Lines, CartLine{
			MenuItemID: l.MenuItemID, Qty: l.Qty, Notes: l.Notes, OptionChoiceIDs: choices,
		})
	}
	return out, nil
}

func toDomainOptions(groups []ports.OptionGroupView) ([]catalog.OptionGroup,
	map[uuid.UUID]ports.OptionGroupView, map[uuid.UUID]ports.OptionChoiceView) {

	domain := make([]catalog.OptionGroup, 0, len(groups))
	groupByChoice := map[uuid.UUID]ports.OptionGroupView{}
	choiceByID := map[uuid.UUID]ports.OptionChoiceView{}

	for _, g := range groups {
		dg := catalog.OptionGroup{
			ID: g.ID.String(), Selection: g.Selection, IsRequired: g.IsRequired,
			MinSelect: g.MinSelect, MaxSelect: g.MaxSelect,
		}
		for _, c := range g.Choices {
			dg.Choices = append(dg.Choices, catalog.OptionChoice{
				ID: c.ID.String(), PriceDelta: c.PriceDelta,
				KitchenUnits: c.KitchenUnits, IsAvailable: c.IsAvailable,
			})
			groupByChoice[c.ID] = g
			choiceByID[c.ID] = c
		}
		domain = append(domain, dg)
	}
	return domain, groupByChoice, choiceByID
}

func availabilityMessage(a catalog.Availability) string {
	switch a {
	case catalog.EightySixed:
		return "This dish is out of stock at the moment."
	case catalog.SoldOutToday:
		return "This dish has sold out for that date."
	case catalog.RuleExcluded:
		return "This dish is not served at that time."
	case catalog.StoreUnavailable:
		return "This dish is not available at this store."
	default:
		return "This dish is not available."
	}
}

func promoMessage(r pricing.PromoReason) string {
	switch r {
	case pricing.PromoExpired:
		return "That promo code has expired."
	case pricing.PromoNotStarted:
		return "That promo code is not active yet."
	case pricing.PromoStoreNotEligible:
		return "That promo code cannot be used at this store."
	case pricing.PromoMinSpend:
		return "Your order is below the minimum spend for that promo code."
	case pricing.PromoCapReached, pricing.PromoCustomerCap:
		return "That promo code has reached its usage limit."
	case pricing.PromoNoEligibleItems:
		return "That promo code does not apply to the items in your cart."
	default:
		return "That promo code is not valid."
	}
}

func slotError(r schedule.Reason) error {
	switch r {
	case schedule.ReasonFull:
		return apierror.Conflict(apierror.CodeSlotFull, "That time slot has just filled up.")
	case schedule.ReasonPast:
		return apierror.Unprocessable(apierror.CodeSlotPast, "That time slot has already passed.")
	case schedule.ReasonLeadTime:
		return apierror.Unprocessable(apierror.CodeSlotLeadTime,
			"That time slot is too soon — please choose a later one.")
	case schedule.ReasonCutoff:
		return apierror.Unprocessable(apierror.CodeSlotCutoff,
			"Ordering for that time slot has closed.")
	case schedule.ReasonBlackout:
		return apierror.Unprocessable(apierror.CodeBlackout, "The store is closed on that date.")
	case schedule.ReasonItemConstraint:
		return apierror.Unprocessable(apierror.CodeItemUnavailable,
			"One of your items is not available in that time slot.")
	default:
		return apierror.Unprocessable(apierror.CodeSlotNotBookable,
			"That time slot is not available.").WithDetails(map[string]any{"reason": string(r)})
	}
}
