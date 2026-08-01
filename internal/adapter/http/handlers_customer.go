package http

import (
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/authsvc"
	"github.com/stevenwilliam/ruuma/internal/app/ordersvc"
	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/identity"
	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// ── Auth ─────────────────────────────────────────────────────────────────────

type registerReq struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=12,max=200"`
	FullName string `json:"full_name" binding:"required,max=120"`
}

func (s *Server) register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the details you entered.", nil))
		return
	}
	if err := s.Auth.Register(c.Request.Context(), req.Email, req.Password, req.FullName); err != nil {
		fail(c, err)
		return
	}
	// The same response whether or not the address was already registered
	// (docs/12, A07).
	ok(c, gin.H{"status": "check_your_email"})
}

func (s *Server) verifyEmail(c *gin.Context) {
	if err := s.Auth.VerifyEmail(c.Request.Context(), c.Query("token")); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "verified"})
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,max=200"`
}

func (s *Server) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Unauthorized("Those sign-in details are not correct."))
		return
	}
	session, err := s.Auth.Login(c.Request.Context(), req.Email, req.Password,
		c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, sessionDTO(session))
}

func (s *Server) staffLogin(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Unauthorized("Those sign-in details are not correct."))
		return
	}
	session, err := s.Auth.StaffLogin(c.Request.Context(), req.Email, req.Password,
		c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, sessionDTO(session))
}

type otpRequestReq struct {
	Phone   string `json:"phone" binding:"required,max=20"`
	Purpose string `json:"purpose" binding:"omitempty,oneof=signup login verify_phone"`
}

func (s *Server) requestOTP(c *gin.Context) {
	var req otpRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please enter a valid mobile number.", nil))
		return
	}
	purpose := req.Purpose
	if purpose == "" {
		purpose = string(identity.PurposeLogin)
	}
	if err := s.Auth.RequestOTP(c.Request.Context(), req.Phone, purpose, c.ClientIP()); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "sent"})
}

type otpVerifyReq struct {
	Phone   string `json:"phone" binding:"required,max=20"`
	Code    string `json:"code" binding:"required,len=6"`
	Purpose string `json:"purpose" binding:"omitempty,oneof=signup login verify_phone"`
}

func (s *Server) verifyOTP(c *gin.Context) {
	var req otpVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Unprocessable(apierror.CodeValidation, "That code is not valid."))
		return
	}
	purpose := req.Purpose
	if purpose == "" {
		purpose = string(identity.PurposeLogin)
	}

	var existing *uuid.UUID
	if p := principal(c); p.ID != uuid.Nil && p.SubjectType == security.SubjectCustomer {
		id := p.ID
		existing = &id
	}

	session, err := s.Auth.VerifyOTP(c.Request.Context(), req.Phone, req.Code, purpose,
		existing, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, sessionDTO(session))
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (s *Server) refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Unauthorized("Your session has expired. Please sign in again."))
		return
	}
	session, err := s.Auth.Refresh(c.Request.Context(), req.RefreshToken,
		c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, sessionDTO(session))
}

func (s *Server) logout(c *gin.Context) {
	p := principal(c)
	if err := s.Auth.Logout(c.Request.Context(), p.SubjectType, p.ID); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (s *Server) oauthStart(c *gin.Context) {
	provider := identity.Provider(c.Param("provider"))
	state := uuid.NewString()
	url, err := s.Auth.OAuthStart(c.Request.Context(), provider, state)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"authorize_url": url, "state": state})
}

func (s *Server) oauthCallback(c *gin.Context) {
	provider := identity.Provider(c.Param("provider"))
	session, err := s.Auth.OAuthCallback(c.Request.Context(), provider, c.Query("code"),
		c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, sessionDTO(session))
}

// ── Profile ──────────────────────────────────────────────────────────────────

func (s *Server) getProfile(c *gin.Context) {
	p := principal(c)
	if p.SubjectType == security.SubjectStaff {
		u, err := s.Staff.Get(c.Request.Context(), p.ID)
		if err != nil {
			fail(c, err)
			return
		}
		ok(c, gin.H{
			"id": u.ID, "email": u.Email, "full_name": u.FullName, "role": u.Role,
			"group_scope": u.IsGroupScope, "stores": u.Stores,
			"permissions": security.PermissionsFor(security.Role(u.Role)),
		})
		return
	}
	cust, err := s.Customers.Get(c.Request.Context(), p.ID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, customerDTO(cust))
}

type updateProfileReq struct {
	FullName       string `json:"full_name" binding:"max=120"`
	Language       string `json:"preferred_language" binding:"omitempty,oneof=id en"`
	MarketingOptIn bool   `json:"marketing_opt_in"`
}

func (s *Server) updateProfile(c *gin.Context) {
	var req updateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the details you entered.", nil))
		return
	}
	p := principal(c)
	if err := s.Customers.UpdateProfile(c.Request.Context(), p.ID,
		req.FullName, req.Language, req.MarketingOptIn); err != nil {
		fail(c, err)
		return
	}
	cust, err := s.Customers.Get(c.Request.Context(), p.ID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, customerDTO(cust))
}

// ── Orders ───────────────────────────────────────────────────────────────────

type createOrderReq struct {
	StoreID        uuid.UUID     `json:"store_id" binding:"required"`
	SlotID         uuid.UUID     `json:"slot_id" binding:"required"`
	FulfilmentType string        `json:"fulfilment_type" binding:"omitempty,oneof=pickup delivery"`
	ContactName    string        `json:"contact_name" binding:"required,max=120"`
	ContactPhone   string        `json:"contact_phone" binding:"required,max=20"`
	Notes          string        `json:"notes" binding:"max=500"`
	PromoCode      string        `json:"promo_code" binding:"max=40"`
	ExpectedTotal  *int64        `json:"expected_total"`
	Lines          []cartLineReq `json:"lines" binding:"required,min=1,max=50,dive"`
}

func (s *Server) createOrder(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check your order details.", nil))
		return
	}
	p := principal(c)

	in := ordersvc.CreateRequest{
		CartRequest: ordersvc.CartRequest{
			StoreID: req.StoreID, CustomerID: p.ID,
			FulfilmentType: fulfilmentMode(req.FulfilmentType),
			PromoCode:      req.PromoCode, Lines: toCartLines(req.Lines),
		},
		SlotID: req.SlotID, ContactName: req.ContactName,
		ContactPhone: req.ContactPhone, Notes: req.Notes,
	}
	if req.ExpectedTotal != nil {
		v := money.Rupiah(*req.ExpectedTotal)
		in.ExpectedTotal = &v
	}

	o, err := s.Orders.Create(c.Request.Context(), in)
	if err != nil {
		fail(c, err)
		return
	}

	bank, _ := s.Stores.PrimaryBankAccount(c.Request.Context(), o.StoreID)
	pay, _ := s.PaymentsRead.ForOrder(c.Request.Context(), o.ID)
	created(c, orderDTO(o, pay, nil, bank))
}

func (s *Server) listOrders(c *gin.Context) {
	p := principal(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	rows, err := s.Orders.History(c.Request.Context(), p.ID, c.Query("q"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	list(c, orderDTOs(rows), "")
}

func (s *Server) getOrder(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Order not found."))
		return
	}
	p := principal(c)

	o, events, pay, err := s.Orders.Get(c.Request.Context(), id, p.ID)
	if err != nil {
		fail(c, err)
		return
	}
	bank, _ := s.Stores.PrimaryBankAccount(c.Request.Context(), o.StoreID)
	ok(c, orderDTO(o, pay, events, bank))
}

func (s *Server) trackOrder(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		fail(c, apierror.Validation("Please enter your order code.", nil))
		return
	}
	p := principal(c)

	// The code alone is never enough: the caller must own the order (BR-2.7.11).
	o, events, pay, err := s.Orders.Track(c.Request.Context(), code, p.ID)
	if err != nil {
		fail(c, err)
		return
	}
	bank, _ := s.Stores.PrimaryBankAccount(c.Request.Context(), o.StoreID)
	ok(c, orderDTO(o, pay, events, bank))
}

type cancelReq struct {
	Reason string `json:"reason" binding:"max=280"`
}

func (s *Server) cancelOwnOrder(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Order not found."))
		return
	}
	var req cancelReq
	_ = c.ShouldBindJSON(&req)

	p := principal(c)
	if err := s.Orders.Cancel(c.Request.Context(), id, p.ID, req.Reason); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "cancelled"})
}

func (s *Server) reorder(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Order not found."))
		return
	}
	p := principal(c)

	result, err := s.Orders.Reorder(c.Request.Context(), id, p.ID)
	if err != nil {
		fail(c, err)
		return
	}

	lines := make([]gin.H, 0, len(result.Lines))
	for _, l := range result.Lines {
		lines = append(lines, gin.H{
			"menu_item_id": l.MenuItemID, "qty": l.Qty, "notes": l.Notes,
			"option_choice_ids": l.OptionChoiceIDs,
		})
	}
	warnings := make([]gin.H, 0, len(result.Warnings))
	for _, w := range result.Warnings {
		warnings = append(warnings, gin.H{
			"code": w.Code, "menu_item_id": w.MenuItemID, "message": w.Message,
		})
	}
	ok(c, gin.H{"lines": lines, "warnings": warnings})
}

// uploadProof accepts the transfer proof as multipart (docs/04 §5).
func (s *Server) uploadProof(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Order not found."))
		return
	}

	if err := c.Request.ParseMultipartForm(6 << 20); err != nil {
		fail(c, apierror.Validation("That upload is too large.", nil))
		return
	}
	file, _, err := c.Request.FormFile("proof")
	if err != nil {
		fail(c, apierror.Validation("Please attach your transfer proof.", nil))
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 6<<20))
	if err != nil {
		fail(c, apierror.Validation("That file could not be read.", nil))
		return
	}

	declared, err := strconv.ParseInt(c.PostForm("declared_amount"), 10, 64)
	if err != nil || declared <= 0 {
		fail(c, apierror.Validation("Please state the amount you transferred.", nil))
		return
	}

	p := principal(c)
	pay, err := s.Payments.UploadProof(c.Request.Context(), id, p.ID, data,
		money.Rupiah(declared), c.PostForm("sender_name"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{
		"status": string(pay.Status), "declared_amount": int64(pay.DeclaredAmount),
		"amount_due": int64(pay.AmountDue), "uploaded_at": pay.ProofUploadedAt,
	})
}

// ── DTOs ─────────────────────────────────────────────────────────────────────

func sessionDTO(s *authsvc.Session) gin.H {
	body := gin.H{
		"access_token": s.AccessToken, "refresh_token": s.RefreshToken,
		"expires_in": s.ExpiresIn, "token_type": "Bearer",
	}
	if s.Customer != nil {
		body["customer"] = customerDTO(s.Customer)
	}
	if s.Staff != nil {
		body["staff"] = gin.H{
			"id": s.Staff.ID, "email": s.Staff.Email, "full_name": s.Staff.FullName,
			"role": s.Staff.Role, "stores": s.Staff.Stores,
			"permissions": security.PermissionsFor(security.Role(s.Staff.Role)),
		}
	}
	return body
}

func customerDTO(c *ports.CustomerView) gin.H {
	return gin.H{
		"id": c.ID, "full_name": c.FullName, "email": c.Email,
		"email_verified": c.EmailVerifiedAt != nil,
		"phone":          c.Phone, "phone_verified": c.PhoneVerifiedAt != nil,
		"preferred_language": c.PreferredLanguage, "marketing_opt_in": c.MarketingOptIn,
		// The frontend uses this to gate checkout with a clear message rather
		// than letting the order fail at the end (BR-2.7.4).
		"can_order": c.PhoneVerifiedAt != nil,
	}
}

var _ = time.Now
