package http

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/catalogsvc"
	"github.com/stevenwilliam/ruuma/internal/app/ordersvc"
	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
)

// ── Stores ───────────────────────────────────────────────────────────────────

func (s *Server) listStores(c *gin.Context) {
	rows, err := s.Catalog.Stores(c.Request.Context(), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}

	type storeDTO struct {
		ID           uuid.UUID  `json:"id"`
		Code         string     `json:"code"`
		Name         string     `json:"name"`
		Slug         string     `json:"slug"`
		AddressLine  string     `json:"address_line"`
		City         string     `json:"city"`
		Phone        string     `json:"phone"`
		Timezone     string     `json:"timezone"`
		Modes        []string   `json:"fulfilment_modes"`
		OpenToday    bool       `json:"open_today"`
		TodayReason  string     `json:"today_reason,omitempty"`
		TodayHours   []string   `json:"today_hours,omitempty"`
		NextOpenDate *time.Time `json:"next_open_date,omitempty"`
	}

	out := make([]storeDTO, 0, len(rows))
	for _, r := range rows {
		modes := make([]string, 0, len(r.Store.Modes))
		for _, m := range r.Store.Modes {
			modes = append(modes, string(m))
		}
		out = append(out, storeDTO{
			ID: r.Store.ID, Code: r.Store.Code, Name: r.Store.Name, Slug: r.Store.Slug,
			AddressLine: r.Store.AddressLine, City: r.Store.City, Phone: r.Store.Phone,
			Timezone: r.Store.Timezone, Modes: modes,
			OpenToday: r.OpenToday, TodayReason: string(r.TodayReason),
			TodayHours: r.TodayBlocks, NextOpenDate: r.NextOpenDate,
		})
	}
	list(c, out, "")
}

func (s *Server) getStore(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		fail(c, apierror.Validation("That store id is not valid.", nil))
		return
	}
	detail, err := s.Catalog.Store(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}

	type hoursDTO struct {
		Weekday  int      `json:"weekday"`
		Mode     string   `json:"fulfilment_type"`
		IsClosed bool     `json:"is_closed"`
		Blocks   []string `json:"blocks,omitempty"`
	}
	hours := make([]hoursDTO, 0, len(detail.Schedule.Weekly))
	for _, w := range detail.Schedule.Weekly {
		row := hoursDTO{Weekday: int(w.Weekday), Mode: string(w.Mode), IsClosed: w.IsClosed}
		for _, b := range w.Blocks {
			row.Blocks = append(row.Blocks, formatTOD(b.Opens)+"–"+formatTOD(b.Closes))
		}
		hours = append(hours, row)
	}
	blackouts := make([]string, 0, len(detail.Schedule.Blackouts))
	for _, b := range detail.Schedule.Blackouts {
		blackouts = append(blackouts, b.String())
	}

	ok(c, gin.H{
		"id": detail.Store.ID, "code": detail.Store.Code, "name": detail.Store.Name,
		"address_line": detail.Store.AddressLine, "city": detail.Store.City,
		"phone": detail.Store.Phone, "timezone": detail.Store.Timezone,
		"hours": hours, "blackout_dates": blackouts,
	})
}

// ── Menu ─────────────────────────────────────────────────────────────────────

func (s *Server) listMenu(c *gin.Context) {
	storeID, err := uuid.Parse(c.Query("store_id"))
	if err != nil {
		fail(c, apierror.Validation("A store must be chosen first.",
			map[string]any{"store_id": "required"}))
		return
	}

	q := portsMenuQuery(c, storeID)
	rows, err := s.Catalog.Menu(c.Request.Context(), q)
	if err != nil {
		fail(c, err)
		return
	}
	list(c, menuDTOs(rows), "")
}

func (s *Server) getMenuItem(c *gin.Context) {
	storeID, err := uuid.Parse(c.Query("store_id"))
	if err != nil {
		fail(c, apierror.Validation("A store must be chosen first.", nil))
		return
	}
	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		fail(c, apierror.Validation("That item id is not valid.", nil))
		return
	}

	detail, err := s.Catalog.Item(c.Request.Context(), storeID, itemID)
	if err != nil {
		fail(c, err)
		return
	}

	type choiceDTO struct {
		ID          uuid.UUID `json:"id"`
		NameID      string    `json:"name_id"`
		NameEN      string    `json:"name_en"`
		PriceDelta  amount    `json:"price_delta"`
		IsAvailable bool      `json:"is_available"`
	}
	type groupDTO struct {
		ID         uuid.UUID   `json:"id"`
		NameID     string      `json:"name_id"`
		NameEN     string      `json:"name_en"`
		Selection  string      `json:"selection"`
		IsRequired bool        `json:"is_required"`
		MinSelect  int         `json:"min_select"`
		MaxSelect  int         `json:"max_select"`
		Choices    []choiceDTO `json:"choices"`
	}

	groups := make([]groupDTO, 0, len(detail.Options))
	for _, g := range detail.Options {
		row := groupDTO{
			ID: g.ID, NameID: g.NameID, NameEN: g.NameEN, Selection: string(g.Selection),
			IsRequired: g.IsRequired, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect,
		}
		for _, ch := range g.Choices {
			row.Choices = append(row.Choices, choiceDTO{
				ID: ch.ID, NameID: ch.NameID, NameEN: ch.NameEN,
				PriceDelta: rupiah(int64(ch.PriceDelta)), IsAvailable: ch.IsAvailable,
			})
		}
		groups = append(groups, row)
	}

	ok(c, gin.H{"item": menuDTO(detail.Item), "option_groups": groups})
}

func (s *Server) listCategories(c *gin.Context) {
	rows, err := s.Catalog.Categories(c.Request.Context(), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

// ── Availability ─────────────────────────────────────────────────────────────

func (s *Server) availableDates(c *gin.Context) {
	storeID, err := uuid.Parse(c.Query("store_id"))
	if err != nil {
		fail(c, apierror.Validation("A store must be chosen first.", nil))
		return
	}
	mode := fulfilmentMode(c.DefaultQuery("type", "pickup"))
	from := time.Now()
	if v := c.Query("from"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			from = parsed
		}
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "31"))

	rows, err := s.Catalog.Dates(c.Request.Context(), storeID, mode, from, days)
	if err != nil {
		fail(c, err)
		return
	}

	type dateDTO struct {
		Date       string `json:"date"`
		IsBookable bool   `json:"is_bookable"`
		Reason     string `json:"reason,omitempty"`
	}
	out := make([]dateDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, dateDTO{
			Date: r.Date.Format("2006-01-02"), IsBookable: r.IsBookable, Reason: string(r.Reason),
		})
	}
	list(c, out, "")
}

func (s *Server) availableSlots(c *gin.Context) {
	storeID, err := uuid.Parse(c.Query("store_id"))
	if err != nil {
		fail(c, apierror.Validation("A store must be chosen first.", nil))
		return
	}
	date, err := time.Parse("2006-01-02", c.Query("date"))
	if err != nil {
		fail(c, apierror.Validation("Please choose a date.", map[string]any{"date": "YYYY-MM-DD"}))
		return
	}

	var itemIDs []uuid.UUID
	for _, raw := range c.QueryArray("items") {
		if id, err := uuid.Parse(raw); err == nil {
			itemIDs = append(itemIDs, id)
		}
	}
	qty, _ := strconv.Atoi(c.DefaultQuery("units", "1"))

	rows, err := s.Catalog.Slots(c.Request.Context(), catalogsvc.SlotQuery{
		StoreID: storeID, Mode: fulfilmentMode(c.DefaultQuery("type", "pickup")),
		Date: date, ItemIDs: itemIDs, Qty: qty,
	})
	if err != nil {
		fail(c, err)
		return
	}

	type slotDTO struct {
		ID              uuid.UUID `json:"slot_id"`
		StartsAt        time.Time `json:"starts_at"`
		EndsAt          time.Time `json:"ends_at"`
		Label           string    `json:"label"`
		IsBookable      bool      `json:"is_bookable"`
		Reason          string    `json:"reason,omitempty"`
		RemainingOrders int       `json:"remaining_orders"`
		RemainingUnits  int       `json:"remaining_units"`
		AlmostFull      bool      `json:"almost_full"`
	}
	out := make([]slotDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, slotDTO{
			ID: r.ID, StartsAt: r.StartsAt, EndsAt: r.EndsAt,
			Label:      r.StartsAt.In(time.Local).Format("15:04") + "–" + r.EndsAt.In(time.Local).Format("15:04"),
			IsBookable: r.IsBookable, Reason: string(r.Reason),
			RemainingOrders: r.RemainingOrders, RemainingUnits: r.RemainingUnits,
			AlmostFull: r.AlmostFull,
		})
	}
	list(c, out, "")
}

// ── Quote ────────────────────────────────────────────────────────────────────

type cartLineReq struct {
	MenuItemID      uuid.UUID   `json:"menu_item_id" binding:"required"`
	Qty             int         `json:"qty" binding:"required,min=1,max=99"`
	Notes           string      `json:"notes" binding:"max=280"`
	OptionChoiceIDs []uuid.UUID `json:"option_choice_ids"`
}

type quoteReq struct {
	StoreID        uuid.UUID     `json:"store_id" binding:"required"`
	FulfilmentType string        `json:"fulfilment_type" binding:"omitempty,oneof=pickup delivery"`
	SlotID         *uuid.UUID    `json:"slot_id"`
	PromoCode      string        `json:"promo_code" binding:"max=40"`
	Lines          []cartLineReq `json:"lines" binding:"required,min=1,max=50,dive"`
}

func (s *Server) quoteCart(c *gin.Context) {
	var req quoteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the details of your cart.", nil))
		return
	}

	p := principal(c)
	quote, err := s.Orders.Quote(c.Request.Context(), ordersvc.CartRequest{
		StoreID: req.StoreID, CustomerID: p.ID,
		FulfilmentType: fulfilmentMode(req.FulfilmentType), SlotID: req.SlotID,
		PromoCode: req.PromoCode, Lines: toCartLines(req.Lines),
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, quoteDTO(quote))
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toCartLines(in []cartLineReq) []ordersvc.CartLine {
	out := make([]ordersvc.CartLine, 0, len(in))
	for _, l := range in {
		out = append(out, ordersvc.CartLine{
			MenuItemID: l.MenuItemID, Qty: l.Qty, Notes: l.Notes,
			OptionChoiceIDs: l.OptionChoiceIDs,
		})
	}
	return out
}

func quoteDTO(q *ordersvc.Quote) gin.H {
	lines := make([]gin.H, 0, len(q.Lines))
	for _, l := range q.Lines {
		lines = append(lines, gin.H{
			"menu_item_id": l.MenuItemID, "name_id": l.ItemNameID, "name_en": l.ItemNameEN,
			"unit_price": rupiah(int64(l.UnitPrice)), "options_delta": rupiah(int64(l.OptionsDelta)),
			"qty": l.Qty, "line_total": rupiah(int64(l.LineTotal)), "kitchen_units": l.KitchenUnits,
		})
	}
	return gin.H{
		"currency": "IDR",
		"subtotal": int64(q.Totals.Subtotal), "discount": int64(q.Totals.Discount),
		"service_charge": int64(q.Totals.ServiceCharge), "tax": int64(q.Totals.Tax),
		"delivery_fee": int64(q.Totals.DeliveryFee), "total": int64(q.Totals.Total),
		"tax_bps": int(q.TaxBps), "service_charge_bps": int(q.ServiceChargeBps),
		"kitchen_units": q.KitchenUnits, "lines": lines, "expires_at": q.ExpiresAt,
	}
}

func portsMenuQuery(c *gin.Context, storeID uuid.UUID) ports.MenuQuery {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	q := ports.MenuQuery{
		StoreID: storeID, Q: c.Query("q"), Cuisine: c.Query("cuisine"),
		Diet: c.Query("diet"), Sort: c.Query("sort"), Limit: limit, Offset: offset,
	}
	if v := c.Query("category_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			q.CategoryID = &id
		}
	}
	return q
}

func fulfilmentMode(v string) schedule.FulfilmentType {
	if v == string(schedule.Delivery) {
		return schedule.Delivery
	}
	return schedule.Pickup
}

func formatTOD(t schedule.TimeOfDay) string {
	return pad2(t.Hour) + ":" + pad2(t.Minute)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
