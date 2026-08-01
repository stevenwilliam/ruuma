package http

import (
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/domain/order"
	dpay "github.com/stevenwilliam/ruuma/internal/domain/payment"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
)

// ── Finance ──────────────────────────────────────────────────────────────────

func (s *Server) paymentQueue(c *gin.Context) {
	p := principal(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	var storeID *uuid.UUID
	if v := c.Query("store_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(c, apierror.Validation("That store id is not valid.", nil))
			return
		}
		storeID = &id
	}

	rows, err := s.Payments.Queue(c.Request.Context(), p, storeID,
		c.DefaultQuery("status", "pending"), c.Query("q"), limit)
	if err != nil {
		fail(c, err)
		return
	}

	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"payment_id": r.Payment.ID, "order_id": r.Payment.OrderID,
			"order_code": r.OrderCode, "customer_name": r.ContactName,
			"store_id": r.Payment.StoreID, "store_name": r.StoreName,
			"status":          string(r.Payment.Status),
			"amount_due":      int64(r.Payment.AmountDue),
			"declared_amount": int64(r.Payment.DeclaredAmount),
			"mismatch":        int64(r.Payment.DeclaredAmount) - int64(r.Payment.AmountDue),
			"sender_name":     r.Payment.SenderName, "has_proof": r.Payment.HasProof,
			"uploaded_at": r.Payment.ProofUploadedAt, "age_minutes": r.AgeMinutes,
			"slot_starts_at": r.SlotStartsAt,
		})
	}
	list(c, out, "")
}

func (s *Server) paymentProof(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Payment not found."))
		return
	}
	url, err := s.Payments.ProofURL(c.Request.Context(), principal(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	// A short-lived presigned URL; the object is never public (BR-2.6.11).
	ok(c, gin.H{"url": url, "expires_in": 600})
}

type verifyReq struct {
	AcceptMismatch bool   `json:"accept_mismatch"`
	MismatchReason string `json:"mismatch_reason" binding:"max=280"`
}

func (s *Server) verifyPayment(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Payment not found."))
		return
	}
	var req verifyReq
	_ = c.ShouldBindJSON(&req)

	pay, err := s.Payments.Verify(c.Request.Context(), principal(c), id,
		req.AcceptMismatch, req.MismatchReason)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{
		"payment_id": pay.ID, "status": string(pay.Status),
		"verified_at": pay.VerifiedAt, "order_id": pay.OrderID,
	})
}

type rejectReq struct {
	Reason string `json:"reason" binding:"required"`
	Note   string `json:"note" binding:"max=280"`
}

func (s *Server) rejectPayment(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Payment not found."))
		return
	}
	var req rejectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Unprocessable(apierror.CodeRejectionReasonRequired,
			"A rejection needs a reason."))
		return
	}

	if err := s.Payments.Reject(c.Request.Context(), principal(c), id,
		dpay.RejectionReason(req.Reason), req.Note); err != nil {
		fail(c, err)
		return
	}
	// No automated message goes out; the reason shows on the customer's order
	// page and finance follows up by hand (D28).
	ok(c, gin.H{"status": "rejected", "notify_customer_manually": true})
}

func (s *Server) refundPayment(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Payment not found."))
		return
	}

	var proof []byte
	amountRaw := ""
	reference, reason := "", ""

	if err := c.Request.ParseMultipartForm(6 << 20); err == nil {
		amountRaw = c.PostForm("amount")
		reference, reason = c.PostForm("reference"), c.PostForm("reason")
		if file, _, err := c.Request.FormFile("proof"); err == nil {
			defer func() { _ = file.Close() }()
			proof, _ = io.ReadAll(io.LimitReader(file, 6<<20))
		}
	} else {
		var body struct {
			Amount    int64  `json:"amount"`
			Reference string `json:"reference"`
			Reason    string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			fail(c, apierror.Validation("Please state the refund amount and reason.", nil))
			return
		}
		amountRaw = strconv.FormatInt(body.Amount, 10)
		reference, reason = body.Reference, body.Reason
	}

	amount, err := strconv.ParseInt(amountRaw, 10, 64)
	if err != nil || amount <= 0 {
		fail(c, apierror.Validation("Please state a valid refund amount.", nil))
		return
	}
	if reason == "" {
		fail(c, apierror.Validation("A refund needs a reason.", map[string]any{"reason": "required"}))
		return
	}

	if err := s.Payments.Refund(c.Request.Context(), principal(c), id,
		money.Rupiah(amount), reference, reason, proof); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "refunded"})
}

func (s *Server) reconciliation(c *gin.Context) {
	date, err := time.Parse("2006-01-02", c.DefaultQuery("date", time.Now().Format("2006-01-02")))
	if err != nil {
		fail(c, apierror.Validation("Please choose a date.", nil))
		return
	}
	var storeID *uuid.UUID
	if v := c.Query("store_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(c, apierror.Validation("That store id is not valid.", nil))
			return
		}
		storeID = &id
	}

	rows, err := s.Payments.Reconciliation(c.Request.Context(), principal(c), date, storeID)
	if err != nil {
		fail(c, err)
		return
	}

	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"store_id": r.StoreID, "store_name": r.StoreName, "orders": r.Orders,
			"order_total": int64(r.OrderTotal), "unique_codes": int64(r.UniqueCodes),
			"declared": int64(r.Declared), "refunds": int64(r.Refunds),
			"mismatches": r.Mismatches, "rejections": r.Rejections,
			"net": int64(r.Declared) - int64(r.Refunds),
		})
	}
	ok(c, gin.H{"date": date.Format("2006-01-02"), "rows": out})
}

// ── Operations ───────────────────────────────────────────────────────────────

func (s *Server) opsBoard(c *gin.Context) {
	p := principal(c)

	var storeID *uuid.UUID
	if v := c.Query("store_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(c, apierror.Validation("That store id is not valid.", nil))
			return
		}
		storeID = &id
	}
	var date *time.Time
	if v := c.Query("date"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			fail(c, apierror.Validation("Please choose a date.", nil))
			return
		}
		date = &parsed
	}

	groups, err := s.Ops.Board(c.Request.Context(), p, storeID, date,
		c.QueryArray("status"), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}

	out := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		orders := make([]gin.H, 0, len(g.Orders))
		for _, o := range g.Orders {
			orders = append(orders, staffOrderDTO(o))
		}
		out = append(out, gin.H{
			"slot_id": g.SlotID, "starts_at": g.StartsAt, "ends_at": g.EndsAt,
			"order_count": len(g.Orders), "orders": orders,
		})
	}
	list(c, out, "")
}

func (s *Server) opsProduction(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Slot not found."))
		return
	}
	rows, err := s.Ops.Production(c.Request.Context(), principal(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"menu_item_id": r.MenuItemID, "item": r.ItemName, "option": r.OptionName,
			"qty": r.Qty, "prep_minutes": r.PrepMinutes,
		})
	}
	list(c, out, "")
}

func (s *Server) opsTicket(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Order not found."))
		return
	}
	t, err := s.Ops.Ticket(c.Request.Context(), principal(c), id)
	if err != nil {
		fail(c, err)
		return
	}

	lines := make([]gin.H, 0, len(t.Lines))
	for _, l := range t.Lines {
		options := make([]string, 0, len(l.Options))
		for _, o := range l.Options {
			options = append(options, o.ChoiceNameID)
		}
		lines = append(lines, gin.H{
			"name": l.ItemNameID, "qty": l.Qty, "options": options, "notes": l.Notes,
		})
	}
	// No payment details and no full contact — a ticket is a cook list
	// (BR-2.8.4).
	ok(c, gin.H{
		"order_code": t.OrderCode, "slot_starts_at": t.SlotStartsAt,
		"slot_ends_at": t.SlotEndsAt, "customer": t.CustomerName,
		"lines": lines, "notes": t.Notes,
	})
}

type advanceReq struct {
	Status string `json:"status" binding:"required,oneof=IN_KITCHEN READY PICKED_UP COMPLETED"`
}

func (s *Server) opsAdvance(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Order not found."))
		return
	}
	var req advanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("That state cannot be set.", nil))
		return
	}
	if err := s.Ops.Advance(c.Request.Context(), principal(c), id, order.Status(req.Status)); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": req.Status})
}

func (s *Server) opsCancel(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Order not found."))
		return
	}
	var req cancelReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Reason == "" {
		fail(c, apierror.Validation("A cancellation needs a reason.",
			map[string]any{"reason": "required"}))
		return
	}
	if err := s.Ops.Cancel(c.Request.Context(), principal(c), id, req.Reason); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "cancelled"})
}

type bulkCancelReq struct {
	OrderIDs []uuid.UUID `json:"order_ids" binding:"required,min=1,max=200"`
	Reason   string      `json:"reason" binding:"required,max=280"`
}

func (s *Server) opsCancelBulk(c *gin.Context) {
	var req bulkCancelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Select orders and give a reason.", nil))
		return
	}
	results, err := s.Ops.CancelBulk(c.Request.Context(), principal(c), req.OrderIDs, req.Reason)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(results))
	for _, r := range results {
		out = append(out, gin.H{"order_id": r.OrderID, "error": r.Error, "cancelled": r.Error == ""})
	}
	ok(c, gin.H{"results": out})
}

func (s *Server) opsUnpaid(c *gin.Context) {
	var storeID *uuid.UUID
	if v := c.Query("store_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(c, apierror.Validation("That store id is not valid.", nil))
			return
		}
		storeID = &id
	}
	rows, err := s.Ops.Unpaid(c.Request.Context(), principal(c), storeID)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, o := range rows {
		dto := staffOrderDTO(o)
		dto["age_minutes"] = int(time.Since(o.CreatedAt).Minutes())
		out = append(out, dto)
	}
	list(c, out, "")
}

func (s *Server) opsAffected(c *gin.Context) {
	storeID, err := uuid.Parse(c.Query("store_id"))
	if err != nil {
		fail(c, apierror.Validation("A store must be chosen.", nil))
		return
	}
	date, err := time.Parse("2006-01-02", c.Query("date"))
	if err != nil {
		fail(c, apierror.Validation("Please choose a date.", nil))
		return
	}
	rows, err := s.Ops.AffectedByClosure(c.Request.Context(), principal(c), storeID, date)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, o := range rows {
		out = append(out, staffOrderDTO(o))
	}
	// These orders are never auto-cancelled — staff decide (BR-2.1.9, D27).
	ok(c, gin.H{"items": out, "note": "These orders are untouched. Cancel and refund each one deliberately."})
}
