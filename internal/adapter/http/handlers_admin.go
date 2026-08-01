package http

import (
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/adminsvc"
	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// registerAdmin mounts the admin surface in its own router group (docs/12, A01).
func (s *Server) registerAdmin(g *gin.RouterGroup) {
	a := g.Group("/admin", requireAuthenticated())

	a.GET("/stores", requirePermission(security.PermOrderViewStore), s.adminListStores)
	a.POST("/stores", requirePermission(security.PermStoreManage), s.adminCreateStore)
	a.PATCH("/stores/:id", requirePermission(security.PermStoreManage), s.adminUpdateStore)
	a.POST("/stores/:id/activate", requirePermission(security.PermStoreManage), s.adminActivateStore)
	a.POST("/stores/:id/deactivate", requirePermission(security.PermStoreManage), s.adminDeactivateStore)
	a.PUT("/stores/:id/fulfilment-modes", requirePermission(security.PermStoreManage), s.adminSetModes)

	a.GET("/stores/:id/hours", requirePermission(security.PermStoreScheduleManage), s.adminHours)
	a.PUT("/stores/:id/hours", requirePermission(security.PermStoreScheduleManage), s.adminReplaceHours)
	a.GET("/stores/:id/date-overrides", requirePermission(security.PermStoreScheduleManage), s.adminDateOverrides)
	a.POST("/stores/:id/date-overrides", requirePermission(security.PermStoreScheduleManage), s.adminSaveDateOverride)
	a.DELETE("/stores/:id/date-overrides/:overrideID", requirePermission(security.PermStoreScheduleManage), s.adminDeleteDateOverride)
	a.GET("/stores/:id/blackouts", requirePermission(security.PermStoreScheduleManage), s.adminBlackouts)
	a.POST("/stores/:id/blackouts", requirePermission(security.PermStoreScheduleManage), s.adminAddBlackout)
	a.DELETE("/stores/:id/blackouts", requirePermission(security.PermStoreScheduleManage), s.adminRemoveBlackout)

	a.GET("/stores/:id/bank-accounts", requirePermission(security.PermStoreBankRead), s.adminBankAccounts)
	a.POST("/stores/:id/bank-accounts", requirePermission(security.PermStoreBankManage), s.adminSaveBankAccount)

	a.POST("/categories", requirePermission(security.PermMenuManage), s.adminSaveCategory)
	a.POST("/menu-items", requirePermission(security.PermMenuManage), s.adminSaveItem)
	a.POST("/menu-items/:id/photo", requirePermission(security.PermMenuManage), s.adminItemPhoto)
	a.PUT("/stores/:id/menu-overrides", requirePermission(security.PermMenuAvailability), s.adminStoreOverride)
	a.POST("/stores/:id/86", requirePermission(security.PermMenu86), s.adminAdd86)
	a.DELETE("/stores/:id/86/:itemID", requirePermission(security.PermMenu86), s.adminLift86)
	a.PUT("/stores/:id/daily-stock", requirePermission(security.PermMenuAvailability), s.adminDailyStock)

	a.GET("/sys-parameters", requirePermission(security.PermParametersManage), s.adminParameters)
	a.PUT("/sys-parameters", requirePermission(security.PermParametersManage), s.adminSetParameter)
	a.DELETE("/sys-parameters/:key", requirePermission(security.PermParametersManage), s.adminDeleteParameter)
	a.GET("/stores/:id/parameters", requirePermission(security.PermStoreCapacityManage), s.adminStoreParameters)
	a.PUT("/stores/:id/parameters", requirePermission(security.PermStoreCapacityManage), s.adminSetStoreParameter)

	a.GET("/users", requirePermission(security.PermStaffManage), s.adminListStaff)
	a.POST("/users", requirePermission(security.PermStaffManage), s.adminCreateStaff)
	a.PATCH("/users/:id", requirePermission(security.PermStaffManage), s.adminUpdateStaff)
	a.DELETE("/users/:id", requirePermission(security.PermStaffManage), s.adminDeactivateStaff)

	a.GET("/audit-log", requirePermission(security.PermAuditView), s.adminAudit)
}

// ── Stores ───────────────────────────────────────────────────────────────────

func (s *Server) adminListStores(c *gin.Context) {
	rows, err := s.Admin.Stores(c.Request.Context(), principal(c), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

type storeReq struct {
	Code        string   `json:"code" binding:"required,max=20"`
	Name        string   `json:"name" binding:"required,max=120"`
	Slug        string   `json:"slug" binding:"required,max=120"`
	AddressLine string   `json:"address_line" binding:"required,max=280"`
	City        string   `json:"city" binding:"required,max=80"`
	Phone       string   `json:"phone" binding:"required,max=20"`
	Timezone    string   `json:"timezone" binding:"max=64"`
	Modes       []string `json:"fulfilment_modes"`
}

func (s *Server) adminCreateStore(c *gin.Context) {
	var req storeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the store details.", nil))
		return
	}
	tz := req.Timezone
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	created, err := s.Admin.CreateStore(c.Request.Context(), principal(c), ports.StoreView{
		Code: req.Code, Name: req.Name, Slug: req.Slug, AddressLine: req.AddressLine,
		City: req.City, Phone: req.Phone, Timezone: tz,
	})
	if err != nil {
		fail(c, err)
		return
	}
	if len(req.Modes) > 0 {
		if err := s.Admin.SetModes(c.Request.Context(), principal(c), created.ID, req.Modes); err != nil {
			fail(c, err)
			return
		}
	}
	created2(c, created)
}

func (s *Server) adminUpdateStore(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req storeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the store details.", nil))
		return
	}
	if err := s.Admin.UpdateStore(c.Request.Context(), principal(c), ports.StoreView{
		ID: id, Code: req.Code, Name: req.Name, Slug: req.Slug,
		AddressLine: req.AddressLine, City: req.City, Phone: req.Phone,
		Timezone: req.Timezone, IsActive: true,
	}); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "updated"})
}

func (s *Server) adminActivateStore(c *gin.Context)   { s.setStoreActive(c, true) }
func (s *Server) adminDeactivateStore(c *gin.Context) { s.setStoreActive(c, false) }

func (s *Server) setStoreActive(c *gin.Context, active bool) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	if err := s.Admin.SetStoreActive(c.Request.Context(), principal(c), id, active); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"is_active": active})
}

type modesReq struct {
	Modes []string `json:"fulfilment_modes" binding:"required,min=1,dive,oneof=pickup delivery"`
}

func (s *Server) adminSetModes(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req modesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Choose at least one fulfilment mode.", nil))
		return
	}
	if err := s.Admin.SetModes(c.Request.Context(), principal(c), id, req.Modes); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"fulfilment_modes": req.Modes})
}

// ── Schedule ─────────────────────────────────────────────────────────────────

func (s *Server) adminHours(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	rows, err := s.Admin.Hours(c.Request.Context(), principal(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

type hoursRowReq struct {
	Weekday    int    `json:"weekday" binding:"min=0,max=6"`
	Mode       string `json:"fulfilment_type" binding:"required,oneof=pickup delivery"`
	BlockIndex int    `json:"block_index" binding:"min=0,max=5"`
	IsClosed   bool   `json:"is_closed"`
	OpensAt    string `json:"opens_at" binding:"max=8"`
	ClosesAt   string `json:"closes_at" binding:"max=8"`
}

func (s *Server) adminReplaceHours(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req struct {
		Hours []hoursRowReq `json:"hours" binding:"required,dive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the opening hours.", nil))
		return
	}

	rows := make([]adminsvc.HoursRow, 0, len(req.Hours))
	for _, h := range req.Hours {
		rows = append(rows, adminsvc.HoursRow{
			Weekday: h.Weekday, Mode: h.Mode, BlockIndex: h.BlockIndex,
			IsClosed: h.IsClosed, OpensAt: h.OpensAt, ClosesAt: h.ClosesAt,
		})
	}
	if err := s.Admin.ReplaceHours(c.Request.Context(), principal(c), id, rows); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "updated", "note": "Applies to slots generated from now on; booked slots keep their capacity."})
}

func (s *Server) adminDateOverrides(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	from, to := dateRange(c)
	rows, err := s.Admin.DateOverrides(c.Request.Context(), principal(c), id, from, to)
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

type overrideReq struct {
	Date       string `json:"business_date" binding:"required,len=10"`
	Mode       string `json:"fulfilment_type" binding:"required,oneof=pickup delivery"`
	BlockIndex int    `json:"block_index" binding:"min=0,max=5"`
	IsClosed   bool   `json:"is_closed"`
	OpensAt    string `json:"opens_at" binding:"max=8"`
	ClosesAt   string `json:"closes_at" binding:"max=8"`
	Reason     string `json:"reason" binding:"max=280"`
}

func (s *Server) adminSaveDateOverride(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req overrideReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the override details.", nil))
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		fail(c, apierror.Validation("Please choose a valid date.", nil))
		return
	}

	if err := s.Admin.SaveDateOverride(c.Request.Context(), principal(c), adminsvc.OverrideRow{
		StoreID: id, Date: date, Mode: req.Mode, BlockIndex: req.BlockIndex,
		IsClosed: req.IsClosed, OpensAt: req.OpensAt, ClosesAt: req.ClosesAt, Reason: req.Reason,
	}); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "saved"})
}

func (s *Server) adminDeleteDateOverride(c *gin.Context) {
	storeID, ok1 := uuidParam(c, "id")
	overrideID, ok2 := uuidParam(c, "overrideID")
	if !ok1 || !ok2 {
		fail(c, apierror.NotFound("Override not found."))
		return
	}
	if err := s.Admin.DeleteDateOverride(c.Request.Context(), principal(c), storeID, overrideID); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (s *Server) adminBlackouts(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	from, to := dateRange(c)
	rows, err := s.Admin.Blackouts(c.Request.Context(), principal(c), id, from, to)
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

type blackoutReq struct {
	Date   string `json:"business_date" binding:"required,len=10"`
	Reason string `json:"reason" binding:"required,max=280"`
}

// adminAddBlackout closes a store for a date — today included. Booked orders
// are reported, never cancelled (BR-2.1.7, BR-2.1.9, D27).
func (s *Server) adminAddBlackout(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req blackoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("A closure needs a date and a reason.", nil))
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		fail(c, apierror.Validation("Please choose a valid date.", nil))
		return
	}

	result, err := s.Admin.AddBlackout(c.Request.Context(), principal(c), adminsvc.BlackoutRow{
		StoreID: id, Date: date, Reason: req.Reason,
	})
	if err != nil {
		fail(c, err)
		return
	}
	created2(c, gin.H{
		"business_date": req.Date, "reason": req.Reason,
		"affected_orders": result.AffectedOrders, "note": result.Note,
	})
}

func (s *Server) adminRemoveBlackout(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	date, err := time.Parse("2006-01-02", c.Query("date"))
	if err != nil {
		fail(c, apierror.Validation("Please choose a valid date.", nil))
		return
	}
	if err := s.Admin.RemoveBlackout(c.Request.Context(), principal(c), id, date); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

// ── Bank accounts ────────────────────────────────────────────────────────────

func (s *Server) adminBankAccounts(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	rows, err := s.Admin.BankAccounts(c.Request.Context(), principal(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

type bankAccountReq struct {
	BankName      string `json:"bank_name" binding:"required,max=80"`
	AccountName   string `json:"account_name" binding:"required,max=120"`
	AccountNumber string `json:"account_number" binding:"required,max=40"`
	IsPrimary     bool   `json:"is_primary"`
}

func (s *Server) adminSaveBankAccount(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req bankAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the account details.", nil))
		return
	}
	if err := s.Admin.SaveBankAccount(c.Request.Context(), principal(c), id, adminsvc.BankAccountInput{
		BankName: req.BankName, AccountName: req.AccountName,
		AccountNumber: req.AccountNumber, IsPrimary: req.IsPrimary, IsActive: true,
	}); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "saved"})
}

// ── Menu ─────────────────────────────────────────────────────────────────────

type categoryReq struct {
	ID        *uuid.UUID `json:"id"`
	NameID    string     `json:"name_id" binding:"required,max=120"`
	NameEN    string     `json:"name_en" binding:"required,max=120"`
	Slug      string     `json:"slug" binding:"required,max=120"`
	Cuisine   string     `json:"cuisine" binding:"required,oneof=indonesian chinese western other"`
	SortOrder int        `json:"sort_order"`
	IsActive  bool       `json:"is_active"`
}

func (s *Server) adminSaveCategory(c *gin.Context) {
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the category details.", nil))
		return
	}
	in := adminsvc.CategoryInput{
		NameID: req.NameID, NameEN: req.NameEN, Slug: req.Slug,
		Cuisine: req.Cuisine, SortOrder: req.SortOrder, IsActive: req.IsActive,
	}
	if req.ID != nil {
		in.ID = *req.ID
	}
	id, err := s.Admin.SaveCategory(c.Request.Context(), principal(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	created2(c, gin.H{"id": id})
}

type itemReq struct {
	ID              *uuid.UUID `json:"id"`
	CategoryID      uuid.UUID  `json:"category_id" binding:"required"`
	SKU             string     `json:"sku" binding:"required,max=40"`
	NameID          string     `json:"name_id" binding:"required,max=140"`
	NameEN          string     `json:"name_en" binding:"required,max=140"`
	DescriptionID   string     `json:"description_id" binding:"max=500"`
	DescriptionEN   string     `json:"description_en" binding:"max=500"`
	BasePrice       int64      `json:"base_price" binding:"min=0"`
	KitchenUnits    int        `json:"kitchen_units" binding:"min=1,max=100"`
	PrepMinutes     int        `json:"prep_minutes" binding:"min=0,max=1440"`
	MinLeadMinutes  int        `json:"min_lead_minutes" binding:"min=0,max=10080"`
	SpiceLevel      int        `json:"spice_level" binding:"min=0,max=3"`
	IsHalal         bool       `json:"is_halal"`
	IsVegetarian    bool       `json:"is_vegetarian"`
	ContainsPork    bool       `json:"contains_pork"`
	ContainsAlcohol bool       `json:"contains_alcohol"`
	ContainsNuts    bool       `json:"contains_nuts"`
	IsActive        bool       `json:"is_active"`
	SortOrder       int        `json:"sort_order"`
}

func (s *Server) adminSaveItem(c *gin.Context) {
	var req itemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the item details.", nil))
		return
	}
	in := adminsvc.ItemInput{
		CategoryID: req.CategoryID, SKU: req.SKU, NameID: req.NameID, NameEN: req.NameEN,
		DescriptionID: req.DescriptionID, DescriptionEN: req.DescriptionEN,
		BasePrice: req.BasePrice, KitchenUnits: req.KitchenUnits, PrepMinutes: req.PrepMinutes,
		MinLeadMinutes: req.MinLeadMinutes, SpiceLevel: req.SpiceLevel,
		IsHalal: req.IsHalal, IsVegetarian: req.IsVegetarian, ContainsPork: req.ContainsPork,
		ContainsAlcohol: req.ContainsAlcohol, ContainsNuts: req.ContainsNuts,
		IsActive: req.IsActive, SortOrder: req.SortOrder,
	}
	if req.ID != nil {
		in.ID = *req.ID
	}
	id, err := s.Admin.SaveItem(c.Request.Context(), principal(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	created2(c, gin.H{"id": id})
}

func (s *Server) adminItemPhoto(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Menu item not found."))
		return
	}
	if err := c.Request.ParseMultipartForm(9 << 20); err != nil {
		fail(c, apierror.Validation("That upload is too large.", nil))
		return
	}
	file, _, err := c.Request.FormFile("photo")
	if err != nil {
		fail(c, apierror.Validation("Please attach a photo.", nil))
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 9<<20))
	if err != nil {
		fail(c, apierror.Validation("That file could not be read.", nil))
		return
	}
	key, err := s.Admin.SetItemPhoto(c.Request.Context(), principal(c), id, data)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"photo_key": key})
}

type overrideMenuReq struct {
	MenuItemID    uuid.UUID `json:"menu_item_id" binding:"required"`
	IsAvailable   *bool     `json:"is_available"`
	PriceOverride *int64    `json:"price_override"`
}

func (s *Server) adminStoreOverride(c *gin.Context) {
	storeID, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req overrideMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the override details.", nil))
		return
	}
	if err := s.Admin.SetStoreOverride(c.Request.Context(), principal(c), storeID,
		req.MenuItemID, req.IsAvailable, req.PriceOverride); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "saved"})
}

type eightySixReq struct {
	MenuItemID uuid.UUID `json:"menu_item_id" binding:"required"`
	Until      string    `json:"until"`
	Reason     string    `json:"reason" binding:"max=280"`
}

func (s *Server) adminAdd86(c *gin.Context) {
	storeID, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req eightySixReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please choose an item.", nil))
		return
	}
	var until *time.Time
	if req.Until != "" {
		parsed, err := time.Parse(time.RFC3339, req.Until)
		if err != nil {
			fail(c, apierror.Validation("That end time is not valid.", nil))
			return
		}
		until = &parsed
	}
	if err := s.Admin.Add86(c.Request.Context(), principal(c), storeID,
		req.MenuItemID, until, req.Reason); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "86"})
}

func (s *Server) adminLift86(c *gin.Context) {
	storeID, ok1 := uuidParam(c, "id")
	itemID, ok2 := uuidParam(c, "itemID")
	if !ok1 || !ok2 {
		fail(c, apierror.NotFound("Item not found."))
		return
	}
	if err := s.Admin.Lift86(c.Request.Context(), principal(c), storeID, itemID); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "available"})
}

type dailyStockReq struct {
	MenuItemID uuid.UUID `json:"menu_item_id" binding:"required"`
	Date       string    `json:"business_date" binding:"required,len=10"`
	Total      int       `json:"stock_total" binding:"min=0"`
}

func (s *Server) adminDailyStock(c *gin.Context) {
	storeID, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req dailyStockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the stock details.", nil))
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		fail(c, apierror.Validation("Please choose a valid date.", nil))
		return
	}
	if err := s.Admin.SetDailyStock(c.Request.Context(), principal(c), storeID,
		req.MenuItemID, date, req.Total); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "saved"})
}

// ── Parameters ───────────────────────────────────────────────────────────────

func (s *Server) adminParameters(c *gin.Context) {
	rows, err := s.Admin.Parameters(c.Request.Context(), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

type paramReq struct {
	Key   string `json:"key" binding:"required,max=120"`
	Value string `json:"value" binding:"max=4000"`
}

func (s *Server) adminSetParameter(c *gin.Context) {
	var req paramReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the parameter.", nil))
		return
	}
	if err := s.Admin.SetParameter(c.Request.Context(), principal(c), req.Key, req.Value); err != nil {
		fail(c, err)
		return
	}
	// Takes effect without a restart (BR-2.9.2).
	ok(c, gin.H{"key": req.Key, "value": req.Value, "applies": "immediately"})
}

func (s *Server) adminDeleteParameter(c *gin.Context) {
	if err := s.Admin.DeleteParameter(c.Request.Context(), principal(c), c.Param("key")); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (s *Server) adminStoreParameters(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	rows, err := s.Admin.StoreParameters(c.Request.Context(), principal(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

func (s *Server) adminSetStoreParameter(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("Store not found."))
		return
	}
	var req paramReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the parameter.", nil))
		return
	}
	if err := s.Admin.SetStoreParameter(c.Request.Context(), principal(c), id, req.Key, req.Value); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"key": req.Key, "value": req.Value, "scope": "store"})
}

// ── Staff ────────────────────────────────────────────────────────────────────

func (s *Server) adminListStaff(c *gin.Context) {
	rows, err := s.Admin.Staff(c.Request.Context(), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, u := range rows {
		out = append(out, gin.H{
			"id": u.ID, "email": u.Email, "full_name": u.FullName, "role": u.Role,
			"group_scope": u.IsGroupScope, "is_active": u.IsActive, "stores": u.Stores,
		})
	}
	list(c, out, "")
}

type staffReq struct {
	Email        string      `json:"email" binding:"required,email,max=254"`
	FullName     string      `json:"full_name" binding:"required,max=120"`
	Role         string      `json:"role" binding:"required,oneof=kitchen counter finance store_manager admin owner"`
	IsGroupScope bool        `json:"group_scope"`
	IsActive     bool        `json:"is_active"`
	Stores       []uuid.UUID `json:"stores"`
	Password     string      `json:"password" binding:"omitempty,min=12,max=200"`
}

func (s *Server) adminCreateStaff(c *gin.Context) {
	var req staffReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the staff details.", nil))
		return
	}
	created, err := s.Admin.CreateStaff(c.Request.Context(), principal(c), ports.StaffView{
		Email: req.Email, FullName: req.FullName, Role: req.Role,
		IsGroupScope: req.IsGroupScope, IsActive: true, Stores: req.Stores,
	}, req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	created2(c, gin.H{"id": created.ID, "email": created.Email, "role": created.Role})
}

func (s *Server) adminUpdateStaff(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("User not found."))
		return
	}
	var req staffReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, apierror.Validation("Please check the staff details.", nil))
		return
	}
	if err := s.Admin.UpdateStaff(c.Request.Context(), principal(c), ports.StaffView{
		ID: id, Email: req.Email, FullName: req.FullName, Role: req.Role,
		IsGroupScope: req.IsGroupScope, IsActive: req.IsActive, Stores: req.Stores,
	}); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "updated"})
}

func (s *Server) adminDeactivateStaff(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		fail(c, apierror.NotFound("User not found."))
		return
	}
	if err := s.Admin.DeactivateStaff(c.Request.Context(), principal(c), id); err != nil {
		fail(c, err)
		return
	}
	// Deactivated, never deleted: the audit trail outlives the person.
	ok(c, gin.H{"status": "deactivated"})
}

// ── Audit ────────────────────────────────────────────────────────────────────

func (s *Server) adminAudit(c *gin.Context) {
	var storeID *uuid.UUID
	if v := c.Query("store_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(c, apierror.Validation("That store id is not valid.", nil))
			return
		}
		storeID = &id
	}
	var from, to *time.Time
	if v := c.Query("from"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			from = &parsed
		}
	}
	if v := c.Query("to"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			to = &parsed
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	rows, err := s.Admin.Audit(c.Request.Context(), principal(c), c.Query("q"),
		c.Query("entity"), storeID, from, to, limit)
	if err != nil {
		fail(c, err)
		return
	}
	list(c, rows, "")
}

func dateRange(c *gin.Context) (time.Time, time.Time) {
	from := time.Now().AddDate(0, 0, -1)
	to := time.Now().AddDate(0, 2, 0)
	if v := c.Query("from"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			from = parsed
		}
	}
	if v := c.Query("to"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			to = parsed
		}
	}
	return from, to
}

// created2 is the 201 helper (named to avoid shadowing local variables called
// "created" in these handlers).
func created2(c *gin.Context, body any) { created(c, body) }
