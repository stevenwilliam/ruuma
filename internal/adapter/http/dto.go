package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/catalog"
)

// menuDTO renders one resolved menu item. Sold-out items are still returned,
// carrying their state, so the menu never silently loses dishes (docs/10 §4.2).
func menuDTO(i ports.MenuItemView) gin.H {
	return gin.H{
		"id": i.ID, "category_id": i.CategoryID, "category_name_id": i.CategoryNameID,
		"category_name_en": i.CategoryNameEN, "cuisine": i.Cuisine, "sku": i.SKU,
		"name_id": i.NameID, "name_en": i.NameEN,
		"description_id": i.DescriptionID, "description_en": i.DescriptionEN,
		"price": rupiah(int64(i.Price)), "photo_key": i.PhotoKey,
		"prep_minutes": i.PrepMinutes, "min_lead_minutes": i.MinLeadMinutes,
		"tags": gin.H{
			"spice_level": i.SpiceLevel, "halal": i.IsHalal, "vegetarian": i.IsVegetarian,
			"contains_pork": i.ContainsPork, "contains_alcohol": i.ContainsAlcohol,
			"contains_nuts": i.ContainsNuts,
		},
		"is_available": i.Availability == catalog.Available,
		"availability": string(i.Availability),
		"stock_left":   i.StockLeft,
	}
}

func menuDTOs(rows []ports.MenuItemView) []gin.H {
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, menuDTO(r))
	}
	return out
}

// orderDTO renders an order for its customer.
func orderDTO(o *ports.OrderView, pay *ports.PaymentView, events []ports.OrderEventView, bank *ports.BankAccountView) gin.H {
	lines := make([]gin.H, 0, len(o.Lines))
	for _, l := range o.Lines {
		options := make([]gin.H, 0, len(l.Options))
		for _, opt := range l.Options {
			options = append(options, gin.H{
				"group": opt.GroupNameID, "choice_id": opt.ChoiceNameID,
				"choice_en": opt.ChoiceNameEN, "price_delta": rupiah(int64(opt.PriceDelta)),
			})
		}
		lines = append(lines, gin.H{
			"id": l.ID, "menu_item_id": l.MenuItemID,
			"name_id": l.ItemNameID, "name_en": l.ItemNameEN,
			"unit_price": rupiah(int64(l.UnitPrice)), "qty": l.Qty,
			"options_delta": rupiah(int64(l.OptionsDelta)),
			"line_total":    rupiah(int64(l.LineTotal)),
			"notes":         l.Notes, "options": options,
		})
	}

	body := gin.H{
		"id": o.ID, "order_code": o.OrderCode, "status": string(o.Status),
		"store": gin.H{"id": o.StoreID, "name": o.StoreName},
		"slot": gin.H{
			"id": o.SlotID, "business_date": o.BusinessDate.Format("2006-01-02"),
			"starts_at": o.SlotStartsAt, "ends_at": o.SlotEndsAt,
		},
		"fulfilment_type": string(o.FulfilmentType),
		"contact":         gin.H{"name": o.ContactName, "phone": o.ContactPhone},
		"notes":           o.Notes,
		"currency":        "IDR",
		"subtotal":        int64(o.Subtotal), "discount": int64(o.Discount),
		"service_charge": int64(o.ServiceCharge), "tax": int64(o.Tax),
		"delivery_fee": int64(o.DeliveryFee), "total": int64(o.Total),
		"unique_code": o.UniqueCode, "amount_due": int64(o.AmountDue),
		"promo_code": o.PromoCode, "created_at": o.CreatedAt, "lines": lines,
	}

	if pay != nil {
		body["payment"] = gin.H{
			"status": string(pay.Status), "method": string(pay.Method),
			"amount_due": int64(pay.AmountDue), "declared_amount": int64(pay.DeclaredAmount),
			"has_proof": pay.HasProof, "proof_uploaded_at": pay.ProofUploadedAt,
			// The rejection reason is shown prominently: no automated message is
			// sent, so the order page is how the customer learns (D28).
			"rejection_reason": pay.RejectionReason, "rejection_note": pay.RejectionNote,
			"verified_at": pay.VerifiedAt,
		}
	}
	if bank != nil {
		body["bank_account"] = gin.H{
			"bank_name": bank.BankName, "account_name": bank.AccountName,
			"account_number": bank.AccountNumber,
		}
	}
	if events != nil {
		history := make([]gin.H, 0, len(events))
		for _, e := range events {
			history = append(history, gin.H{
				"from": e.FromStatus, "to": e.ToStatus, "actor": e.ActorType,
				"reason": e.Reason, "at": e.CreatedAt,
			})
		}
		body["history"] = history
	}
	return body
}

func orderDTOs(rows []ports.OrderView) []gin.H {
	out := make([]gin.H, 0, len(rows))
	for i := range rows {
		out = append(out, orderDTO(&rows[i], nil, nil, nil))
	}
	return out
}

// staffOrderDTO is the board view: enough to cook and hand over, no payment
// detail beyond its state.
func staffOrderDTO(o ports.OrderView) gin.H {
	lines := make([]gin.H, 0, len(o.Lines))
	for _, l := range o.Lines {
		options := make([]string, 0, len(l.Options))
		for _, opt := range l.Options {
			options = append(options, opt.ChoiceNameID)
		}
		lines = append(lines, gin.H{
			"name": l.ItemNameID, "qty": l.Qty, "options": options, "notes": l.Notes,
		})
	}
	return gin.H{
		"id": o.ID, "order_code": o.OrderCode, "status": string(o.Status),
		"contact_name": o.ContactName, "contact_phone": o.ContactPhone,
		"slot_starts_at": o.SlotStartsAt, "slot_ends_at": o.SlotEndsAt,
		"total": int64(o.Total), "amount_due": int64(o.AmountDue),
		"kitchen_units": o.KitchenUnits, "lines": lines, "created_at": o.CreatedAt,
	}
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
