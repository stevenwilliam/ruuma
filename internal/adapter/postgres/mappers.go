package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/catalog"
	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/domain/order"
	dpay "github.com/stevenwilliam/ruuma/internal/domain/payment"
	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
)

// This file maps database rows to the app layer's DTOs. It exists so that the
// app never imports gorm or this package (CLAUDE.md §2): the adapter knows the
// app's shapes, not the other way round.

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toStoreView(s Store, modes []StoreFulfilmentMode) ports.StoreView {
	v := ports.StoreView{
		ID: s.ID, Code: s.Code, Name: s.Name, Slug: s.Slug,
		AddressLine: s.AddressLine, City: s.City, Phone: s.Phone,
		Timezone: s.Timezone, IsActive: s.IsActive,
	}
	for _, m := range modes {
		if m.IsEnabled {
			v.Modes = append(v.Modes, schedule.FulfilmentType(m.FulfilmentType))
		}
	}
	return v
}

func toMenuItemView(r ResolvedItem) ports.MenuItemView {
	return ports.MenuItemView{
		ID: r.Item.ID, CategoryID: r.Item.CategoryID,
		CategoryNameID: r.Category.NameID, CategoryNameEN: r.Category.NameEN,
		Cuisine: r.Category.Cuisine, SKU: r.Item.SKU,
		NameID: r.Item.NameID, NameEN: r.Item.NameEN,
		DescriptionID: str(r.Item.DescriptionID), DescriptionEN: str(r.Item.DescriptionEN),
		Price: r.Price, KitchenUnits: r.Item.KitchenUnits, PrepMinutes: r.Item.PrepMinutes,
		MinLeadMinutes: r.Item.MinLeadMinutes, PhotoKey: str(r.Item.PhotoKey),
		SpiceLevel: r.Item.SpiceLevel, IsHalal: r.Item.IsHalal,
		IsVegetarian: r.Item.IsVegetarian, ContainsPork: r.Item.ContainsPork,
		ContainsAlcohol: r.Item.ContainsAlcohol, ContainsNuts: r.Item.ContainsNuts,
		Availability: r.Availability, StockLeft: r.StockLeft,
	}
}

func toOrderView(o Order, storeName string, lines []OrderLine, opts map[uuid.UUID][]OrderLineOption) ports.OrderView {
	v := ports.OrderView{
		ID: o.ID, OrderCode: o.OrderCode, StoreID: o.StoreID, StoreName: storeName,
		CustomerID: o.CustomerID, SlotID: o.SlotID,
		FulfilmentType: schedule.FulfilmentType(o.FulfilmentType),
		BusinessDate:   o.BusinessDate, SlotStartsAt: o.SlotStartsAt, SlotEndsAt: o.SlotEndsAt,
		Status: order.Status(o.Status), ContactName: o.ContactName, ContactPhone: o.ContactPhone,
		Notes:    str(o.Notes),
		Subtotal: money.Rupiah(o.Subtotal), Discount: money.Rupiah(o.Discount),
		ServiceCharge: money.Rupiah(o.ServiceCharge), Tax: money.Rupiah(o.Tax),
		DeliveryFee: money.Rupiah(o.DeliveryFee), Total: money.Rupiah(o.Total),
		UniqueCode: o.UniqueCode, AmountDue: money.Rupiah(o.AmountDue),
		PromoCode: str(o.PromoCode), KitchenUnits: o.KitchenUnits, CreatedAt: o.CreatedAt,
	}
	for _, l := range lines {
		lv := ports.OrderLineView{
			ID: l.ID, MenuItemID: l.MenuItemID, ItemNameID: l.ItemNameID, ItemNameEN: l.ItemNameEN,
			UnitPrice: money.Rupiah(l.UnitPrice), Qty: l.Qty,
			OptionsDelta: money.Rupiah(l.OptionsDelta), LineTotal: money.Rupiah(l.LineTotal),
			KitchenUnits: l.KitchenUnits, Notes: str(l.Notes),
		}
		for _, o := range opts[l.ID] {
			lv.Options = append(lv.Options, ports.OrderLineOptionView{
				OptionGroupID: o.OptionGroupID, OptionChoiceID: o.OptionChoiceID,
				GroupNameID: o.GroupNameID, ChoiceNameID: o.ChoiceNameID,
				ChoiceNameEN: o.ChoiceNameEN, PriceDelta: money.Rupiah(o.PriceDelta),
			})
		}
		v.Lines = append(v.Lines, lv)
	}
	return v
}

func toPaymentView(p Payment) ports.PaymentView {
	return ports.PaymentView{
		ID: p.ID, OrderID: p.OrderID, StoreID: p.StoreID,
		Method: dpay.Method(p.Method), Status: dpay.Status(p.Status),
		AmountDue: money.Rupiah(p.AmountDue), DeclaredAmount: money.Rupiah(deref(p.DeclaredAmount)),
		SenderName: str(p.SenderName), HasProof: p.ProofObjectKey != nil,
		ProofObjectKey: str(p.ProofObjectKey), ProofUploadedAt: p.ProofUploadedAt,
		RejectionReason: str(p.RejectionReason), RejectionNote: str(p.RejectionNote),
		VerifiedAt: p.VerifiedAt, RefundedAmount: money.Rupiah(deref(p.RefundedAmount)),
	}
}

func toCustomerView(c Customer) ports.CustomerView {
	return ports.CustomerView{
		ID: c.ID, FullName: c.FullName, Email: str(c.Email), EmailVerifiedAt: c.EmailVerifiedAt,
		Phone: str(c.Phone), PhoneVerifiedAt: c.PhoneVerifiedAt,
		PreferredLanguage: c.PreferredLanguage, MarketingOptIn: c.MarketingOptIn,
		IsActive: c.IsActive, HasPassword: c.PasswordHash != nil, LockedUntil: c.LockedUntil,
	}
}

func toStaffView(u User, stores []uuid.UUID) ports.StaffView {
	return ports.StaffView{
		ID: u.ID, Email: u.Email, FullName: u.FullName, Role: u.Role,
		IsGroupScope: u.IsGroupScope, IsActive: u.IsActive, Stores: stores,
		LockedUntil: u.LockedUntil,
	}
}

func toOptionGroupViews(groups []OptionGroup, choices map[uuid.UUID]OptionChoice) []ports.OptionGroupView {
	byGroup := map[uuid.UUID][]ports.OptionChoiceView{}
	for _, c := range choices {
		byGroup[c.OptionGroupID] = append(byGroup[c.OptionGroupID], ports.OptionChoiceView{
			ID: c.ID, NameID: c.NameID, NameEN: c.NameEN,
			PriceDelta: money.Rupiah(c.PriceDelta), KitchenUnits: c.KitchenUnits,
			IsAvailable: c.IsAvailable,
		})
	}
	out := make([]ports.OptionGroupView, 0, len(groups))
	for _, g := range groups {
		out = append(out, ports.OptionGroupView{
			ID: g.ID, NameID: g.NameID, NameEN: g.NameEN,
			Selection: catalog.Selection(g.Selection), IsRequired: g.IsRequired,
			MinSelect: g.MinSelect, MaxSelect: g.MaxSelect, Choices: byGroup[g.ID],
		})
	}
	return out
}

func toNewOrder(in ports.NewOrderInput) NewOrder {
	out := NewOrder{
		StoreID: in.StoreID, CustomerID: in.CustomerID, SlotID: in.SlotID,
		FulfilmentType: string(in.FulfilmentType), BusinessDate: in.BusinessDate,
		SlotStartsAt: in.SlotStartsAt, SlotEndsAt: in.SlotEndsAt,
		ContactName: in.ContactName, ContactPhone: in.ContactPhone, Notes: strPtr(in.Notes),
		Subtotal: int64(in.Totals.Subtotal), Discount: int64(in.Totals.Discount),
		ServiceCharge: int64(in.Totals.ServiceCharge), Tax: int64(in.Totals.Tax),
		DeliveryFee: int64(in.Totals.DeliveryFee), Total: int64(in.Totals.Total),
		TaxBps: int(in.TaxBps), ServiceChargeBps: int(in.ServiceChargeBps),
		KitchenUnits: in.KitchenUnits, PromotionID: in.PromotionID,
		PromoCode: strPtr(in.PromoCode), BankAccountID: in.BankAccountID,
		MaxUnpaid: in.MaxUnpaid,
	}
	for _, l := range in.Lines {
		line := NewOrderLine{
			MenuItemID: l.MenuItemID, ItemNameID: l.ItemNameID, ItemNameEN: l.ItemNameEN,
			UnitPrice: int64(l.UnitPrice), Qty: l.Qty, OptionsDelta: int64(l.OptionsDelta),
			LineTotal: int64(l.LineTotal), KitchenUnits: l.KitchenUnits, Notes: strPtr(l.Notes),
		}
		for _, o := range l.Options {
			line.Options = append(line.Options, NewOrderLineOption{
				OptionGroupID: o.OptionGroupID, OptionChoiceID: o.OptionChoiceID,
				GroupNameID: o.GroupNameID, ChoiceNameID: o.ChoiceNameID,
				ChoiceNameEN: o.ChoiceNameEN, PriceDelta: int64(o.PriceDelta),
			})
		}
		out.Lines = append(out.Lines, line)
	}
	return out
}

func ageMinutes(t *time.Time, now time.Time) int {
	if t == nil {
		return 0
	}
	return int(now.Sub(*t).Minutes())
}
