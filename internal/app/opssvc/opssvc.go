// Package opssvc backs the operations screens: the orders board, the kitchen
// production summary, status transitions and staff cancellation (BR-2.8.x).
//
// Every read and write here is bounded by the caller's store scope (BR-2.7.8).
package opssvc

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/order"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

type Service struct {
	orders   ports.Orders
	stores   ports.Stores
	slots    ports.Slots
	notifier ports.Notifier
	params   ports.Params
	audit    ports.Auditor
	clock    ports.Clock
}

func New(orders ports.Orders, stores ports.Stores, slots ports.Slots, notifier ports.Notifier,
	params ports.Params, audit ports.Auditor, clk ports.Clock) *Service {
	return &Service{orders: orders, stores: stores, slots: slots, notifier: notifier,
		params: params, audit: audit, clock: clk}
}

// SlotGroup is one slot's worth of the board (BR-2.8.1).
type SlotGroup struct {
	SlotID   uuid.UUID
	StartsAt time.Time
	EndsAt   time.Time
	Orders   []ports.OrderView
}

// Board returns the store's orders grouped by slot, in slot order.
func (s *Service) Board(ctx context.Context, p security.Principal, storeID *uuid.UUID,
	date *time.Time, statuses []string, q string) ([]SlotGroup, error) {

	if storeID != nil && !p.CanAccessStore(*storeID) {
		return nil, apierror.Forbidden(apierror.CodeStoreOutOfScope, "That store is outside your access.")
	}
	rows, err := s.orders.Board(ctx, storeID, date, statuses, q, 500, p.StoreScope())
	if err != nil {
		return nil, err
	}

	bySlot := map[uuid.UUID]*SlotGroup{}
	for _, o := range rows {
		g, ok := bySlot[o.SlotID]
		if !ok {
			g = &SlotGroup{SlotID: o.SlotID, StartsAt: o.SlotStartsAt, EndsAt: o.SlotEndsAt}
			bySlot[o.SlotID] = g
		}
		g.Orders = append(g.Orders, o)
	}

	out := make([]SlotGroup, 0, len(bySlot))
	for _, g := range bySlot {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out, nil
}

// Production returns the aggregated cook list for a slot, longest-prep first
// (BR-2.8.2/3).
func (s *Service) Production(ctx context.Context, p security.Principal, slotID uuid.UUID) ([]ports.ProductionRow, error) {
	slot, err := s.slots.Get(ctx, slotID)
	if err != nil {
		return nil, err
	}
	if !p.CanAccessStore(slot.StoreID) {
		return nil, apierror.Forbidden(apierror.CodeStoreOutOfScope, "That slot belongs to another store.")
	}
	return s.orders.ProductionSummary(ctx, slotID, p.StoreScope())
}

// Ticket is a printable kitchen ticket. It deliberately carries no payment
// detail and no full contact details (BR-2.8.4).
type Ticket struct {
	OrderCode    string
	SlotStartsAt time.Time
	SlotEndsAt   time.Time
	CustomerName string
	Lines        []ports.OrderLineView
	Notes        string
}

// Ticket builds the printable ticket for one order.
func (s *Service) Ticket(ctx context.Context, p security.Principal, orderID uuid.UUID) (*Ticket, error) {
	o, err := s.orders.GetInScope(ctx, orderID, p.StoreScope())
	if err != nil {
		return nil, err
	}
	return &Ticket{
		OrderCode: o.OrderCode, SlotStartsAt: o.SlotStartsAt, SlotEndsAt: o.SlotEndsAt,
		CustomerName: firstName(o.ContactName), Lines: o.Lines, Notes: o.Notes,
	}, nil
}

// Advance moves an order along the kitchen and counter path (BR-2.4.2).
func (s *Service) Advance(ctx context.Context, p security.Principal, orderID uuid.UUID, to order.Status) error {
	switch to {
	case order.InKitchen, order.Ready:
		if !p.Can(security.PermOrderStatusKitchen) {
			return apierror.Forbidden(apierror.CodeForbidden, "You cannot change that order's state.")
		}
	case order.PickedUp:
		if !p.Can(security.PermOrderStatusHandover) {
			return apierror.Forbidden(apierror.CodeForbidden, "You cannot hand over that order.")
		}
	case order.Completed:
		if !p.Can(security.PermOrderStatusHandover) {
			return apierror.Forbidden(apierror.CodeForbidden, "You cannot complete that order.")
		}
	default:
		return apierror.Unprocessable(apierror.CodeValidation, "That state cannot be set from here.")
	}

	o, err := s.orders.GetInScope(ctx, orderID, p.StoreScope())
	if err != nil {
		return err
	}
	if err := s.orders.Transition(ctx, orderID, to, order.ActorStaff, &p.ID, "", p.StoreScope()); err != nil {
		return err
	}

	if to == order.Ready && s.params.Bool(ctx, nil, "notify.event.order_ready_enabled") {
		_ = s.notifier.Queue(ctx, ports.QueuedNotification{
			OrderID: &o.ID, CustomerID: &o.CustomerID, Channel: "whatsapp",
			Provider: s.params.String(ctx, nil, "notify.provider"), Event: "order_ready",
			Target: o.ContactPhone, TemplateKey: "notify.template.order_ready", Language: "id",
		})
	}

	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "order.status." + string(to),
		EntityType: "order", EntityID: &orderID, StoreID: &o.StoreID,
		Before: map[string]any{"status": string(o.Status)},
		After:  map[string]any{"status": string(to)},
	})
}

// Cancel is staff cancellation, allowed at any time with a reason (BR-2.3.14).
func (s *Service) Cancel(ctx context.Context, p security.Principal, orderID uuid.UUID, reason string) error {
	if !p.Can(security.PermOrderCancelStaff) {
		return apierror.Forbidden(apierror.CodeForbidden, "You cannot cancel orders.")
	}
	if reason == "" {
		return apierror.Validation("A cancellation needs a reason.", map[string]any{"reason": "required"})
	}
	o, err := s.orders.GetInScope(ctx, orderID, p.StoreScope())
	if err != nil {
		return err
	}
	if err := s.orders.Transition(ctx, orderID, order.Cancelled,
		order.ActorStaff, &p.ID, reason, p.StoreScope()); err != nil {
		return err
	}
	return s.audit.Write(ctx, ports.AuditEntry{
		ActorType: "staff", ActorID: &p.ID, Action: "order.cancel.staff",
		EntityType: "order", EntityID: &orderID, StoreID: &o.StoreID,
		After: map[string]any{"reason": reason},
	})
}

// CancelBulk cancels several orders, reporting per-order outcomes rather than
// failing the whole batch (docs/06 §2.1).
type BulkResult struct {
	OrderID uuid.UUID
	Error   string
}

func (s *Service) CancelBulk(ctx context.Context, p security.Principal, orderIDs []uuid.UUID, reason string) ([]BulkResult, error) {
	if !p.Can(security.PermOrderCancelStaff) {
		return nil, apierror.Forbidden(apierror.CodeForbidden, "You cannot cancel orders.")
	}
	if reason == "" {
		return nil, apierror.Validation("A cancellation needs a reason.", map[string]any{"reason": "required"})
	}
	out := make([]BulkResult, 0, len(orderIDs))
	for _, id := range orderIDs {
		res := BulkResult{OrderID: id}
		if err := s.Cancel(ctx, p, id, reason); err != nil {
			res.Error = err.Error()
		}
		out = append(out, res)
	}
	return out, nil
}

// Unpaid lists ageing unpaid orders — the phase-1 slot-squatting control
// (BR-2.3.15, D25).
func (s *Service) Unpaid(ctx context.Context, p security.Principal, storeID *uuid.UUID) ([]ports.OrderView, error) {
	if storeID != nil && !p.CanAccessStore(*storeID) {
		return nil, apierror.Forbidden(apierror.CodeStoreOutOfScope, "That store is outside your access.")
	}
	return s.orders.Unpaid(ctx, storeID, p.StoreScope())
}

// AffectedByClosure lists live orders on a closed date so staff can contact
// those customers by hand (BR-2.1.9, D27).
func (s *Service) AffectedByClosure(ctx context.Context, p security.Principal, storeID uuid.UUID, date time.Time) ([]ports.OrderView, error) {
	if !p.CanAccessStore(storeID) {
		return nil, apierror.Forbidden(apierror.CodeStoreOutOfScope, "That store is outside your access.")
	}
	return s.orders.AffectedByClosure(ctx, storeID, date, p.StoreScope())
}

func firstName(full string) string {
	for i, r := range full {
		if r == ' ' {
			return full[:i]
		}
	}
	return full
}
