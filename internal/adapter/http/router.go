package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/stevenwilliam/ruuma/internal/app/adminsvc"
	"github.com/stevenwilliam/ruuma/internal/app/authsvc"
	"github.com/stevenwilliam/ruuma/internal/app/catalogsvc"
	"github.com/stevenwilliam/ruuma/internal/app/opssvc"
	"github.com/stevenwilliam/ruuma/internal/app/ordersvc"
	"github.com/stevenwilliam/ruuma/internal/app/paymentsvc"
	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/platform/ratelimit"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// Deps is everything the HTTP layer needs. Services come from internal/app;
// this package only maps requests and responses.
type Deps struct {
	Catalog      *catalogsvc.Service
	Orders       *ordersvc.Service
	Payments     *paymentsvc.Service
	Auth         *authsvc.Service
	Ops          *opssvc.Service
	Admin        *adminsvc.Service
	Stores       ports.Stores
	Staff        ports.Staff
	Customers    ports.Customers
	PaymentsRead ports.Payments
	Signer       *security.TokenSigner
	Limiter      *ratelimit.Limiter
	// Limits are resolved from sys_parameters at boot (docs/04 §9). A zero
	// value falls back to the compiled default in platform/ratelimit.
	Limits       Limits
	Idempotency  Idempotency
	Log          *slog.Logger
	IsProduction bool
	Origins      []string
	Version      string
	Commit       string
	Ready        func() error
}

// Idempotency replays a stored response for a repeated key (docs/04 §1).
type Idempotency interface {
	Begin(ctx context.Context, key, subjectType string, subjectID uuid.UUID, endpoint string, body []byte) (*StoredResponse, error)
	Complete(ctx context.Context, key, subjectType string, subjectID uuid.UUID, endpoint string, code int, body []byte) error
	Abandon(ctx context.Context, key, subjectType string, subjectID uuid.UUID, endpoint string) error
}

// Limits carries the tunable rate-limit rules. They live here rather than as
// constants because docs/04 §9 makes them operational settings, and an
// operator retunes them without a deploy (BR-1.4.1).
type Limits struct {
	Login       ratelimit.Rule
	StaffLogin  ratelimit.Rule
	OTPRequest  ratelimit.Rule
	OTPVerify   ratelimit.Rule
	Tracking    ratelimit.Rule
	OrderCreate ratelimit.Rule
	MenuRead    ratelimit.Rule
}

// withDefaults fills any unset rule from the compiled fallbacks.
func (l Limits) withDefaults() Limits {
	pick := func(rule, def ratelimit.Rule) ratelimit.Rule {
		if rule.Burst <= 0 || rule.Window <= 0 {
			return def
		}
		return rule
	}
	return Limits{
		Login:       pick(l.Login, ratelimit.RuleLogin),
		StaffLogin:  pick(l.StaffLogin, ratelimit.RuleStaffLogin),
		OTPRequest:  pick(l.OTPRequest, ratelimit.RuleOTPRequest),
		OTPVerify:   pick(l.OTPVerify, ratelimit.RuleOTPVerify),
		Tracking:    pick(l.Tracking, ratelimit.RuleTracking),
		OrderCreate: pick(l.OrderCreate, ratelimit.RuleOrderCreate),
		MenuRead:    pick(l.MenuRead, ratelimit.RuleMenuRead),
	}
}

// StoredResponse is a previously recorded response.
type StoredResponse struct {
	Code int
	Body []byte
}

// Server holds the wired dependencies.
type Server struct {
	Deps
}

// New builds the gin engine with the full middleware stack.
func New(d Deps) *gin.Engine {
	if d.IsProduction {
		gin.SetMode(gin.ReleaseMode)
	}
	d.Limits = d.Limits.withDefaults()
	s := &Server{Deps: d}

	r := gin.New()
	r.RedirectTrailingSlash = false
	_ = r.SetTrustedProxies([]string{"127.0.0.1"})

	r.Use(gin.Recovery(), requestID(), observability(),
		securityHeaders(d.IsProduction), corsMiddleware(d.Origins), bodyLimit(1<<20))

	r.GET("/health", s.health)
	r.GET("/health/ready", s.ready)

	// /metrics is served only on the loopback listener (docs/12, A05); the
	// public router deliberately does not expose it.

	api := r.Group("/api/v1")
	api.Use(authenticate(d.Signer, d.Stores, d.Staff))

	s.registerPublic(api)
	s.registerAuth(api)
	s.registerCustomer(api)
	s.registerFinance(api)
	s.registerOps(api)
	s.registerAdmin(api)

	r.NoRoute(func(c *gin.Context) {
		fail(c, notFound())
	})
	return r
}

// MetricsHandler is mounted on the loopback-only listener.
func MetricsHandler() http.Handler { return promhttp.Handler() }

func (s *Server) health(c *gin.Context) {
	ok(c, gin.H{"status": "ok", "version": s.Version, "commit": s.Commit})
}

func (s *Server) ready(c *gin.Context) {
	if s.Ready != nil {
		if err := s.Ready(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
	}
	ok(c, gin.H{"status": "ready"})
}

func (s *Server) registerPublic(g *gin.RouterGroup) {
	menuLimit := rateLimit(s.Limiter, "menu", s.Limits.MenuRead, byIP)

	g.GET("/stores", menuLimit, s.listStores)
	g.GET("/stores/:id", menuLimit, s.getStore)
	g.GET("/menu", menuLimit, s.listMenu)
	g.GET("/menu/:id", menuLimit, s.getMenuItem)
	g.GET("/categories", menuLimit, s.listCategories)
	g.GET("/availability/dates", menuLimit, s.availableDates)
	g.GET("/availability/slots", menuLimit, s.availableSlots)
	g.POST("/cart/quote", rateLimit(s.Limiter, "quote", s.Limits.OrderCreate, bySubject), s.quoteCart)
}

func (s *Server) registerAuth(g *gin.RouterGroup) {
	auth := g.Group("/auth")
	auth.POST("/register", rateLimit(s.Limiter, "register", s.Limits.Login, byIP), s.register)
	auth.GET("/verify-email", s.verifyEmail)
	auth.POST("/login", captureBody(), rateLimit(s.Limiter, "login", s.Limits.Login, byEmailBody), s.login)
	auth.POST("/refresh", rateLimit(s.Limiter, "refresh", s.Limits.Login, byIP), s.refresh)
	auth.POST("/logout", requireAuthenticated(), s.logout)
	auth.POST("/oauth/:provider/start", s.oauthStart)
	auth.GET("/oauth/:provider/callback", s.oauthCallback)

	g.POST("/otp/request", captureBody(), rateLimit(s.Limiter, "otp_request", s.Limits.OTPRequest, byPhoneBody), s.requestOTP)
	g.POST("/otp/verify", captureBody(), rateLimit(s.Limiter, "otp_verify", s.Limits.OTPVerify, byPhoneBody), s.verifyOTP)

	g.POST("/staff/login", captureBody(), rateLimit(s.Limiter, "staff_login", s.Limits.StaffLogin, byEmailBody), s.staffLogin)
}

func (s *Server) registerCustomer(g *gin.RouterGroup) {
	me := g.Group("/me", requireAuthenticated())
	me.GET("", s.getProfile)
	me.PATCH("", requirePermission(security.PermProfileManage), s.updateProfile)

	orders := g.Group("/orders", requireAuthenticated())
	orders.POST("", requirePermission(security.PermOrderCreate),
		rateLimit(s.Limiter, "order_create", s.Limits.OrderCreate, bySubject),
		captureBody(), s.idempotent("orders.create"), s.createOrder)
	orders.GET("", requirePermission(security.PermOrderViewOwn), s.listOrders)
	orders.GET("/track", requirePermission(security.PermOrderViewOwn),
		rateLimit(s.Limiter, "track", s.Limits.Tracking, bySubject), s.trackOrder)
	orders.GET("/:id", requirePermission(security.PermOrderViewOwn), s.getOrder)
	orders.POST("/:id/cancel", requirePermission(security.PermOrderCancelOwn), s.cancelOwnOrder)
	orders.POST("/:id/reorder", requirePermission(security.PermOrderViewOwn), s.reorder)
	orders.POST("/:id/payment-proof", requirePermission(security.PermPaymentProofUpload),
		s.uploadProof)
}

func (s *Server) registerFinance(g *gin.RouterGroup) {
	fin := g.Group("/finance", requireAuthenticated())
	fin.GET("/payments", requirePermission(security.PermPaymentQueueView), s.paymentQueue)
	fin.GET("/payments/:id/proof", requirePermission(security.PermPaymentQueueView), s.paymentProof)
	fin.POST("/payments/:id/verify", requirePermission(security.PermPaymentVerify),
		captureBody(), s.idempotent("payments.verify"), s.verifyPayment)
	fin.POST("/payments/:id/reject", requirePermission(security.PermPaymentVerify),
		captureBody(), s.idempotent("payments.reject"), s.rejectPayment)
	fin.POST("/payments/:id/refund", requirePermission(security.PermPaymentRefund),
		captureBody(), s.idempotent("payments.refund"), s.refundPayment)
	fin.GET("/reconciliation", requirePermission(security.PermReconciliation), s.reconciliation)
}

func (s *Server) registerOps(g *gin.RouterGroup) {
	ops := g.Group("/ops", requireAuthenticated())
	ops.GET("/orders", requirePermission(security.PermOrderViewStore), s.opsBoard)
	ops.GET("/orders/unpaid", requirePermission(security.PermOrderViewStore), s.opsUnpaid)
	ops.GET("/orders/affected", requirePermission(security.PermOrderViewStore), s.opsAffected)
	ops.GET("/orders/:id/ticket", requirePermission(security.PermTicketPrint), s.opsTicket)
	ops.POST("/orders/:id/status", requirePermission(security.PermOrderViewStore),
		captureBody(), s.idempotent("ops.status"), s.opsAdvance)
	ops.POST("/orders/:id/cancel", requirePermission(security.PermOrderCancelStaff), s.opsCancel)
	ops.POST("/orders/cancel-bulk", requirePermission(security.PermOrderCancelStaff),
		captureBody(), s.idempotent("ops.cancel_bulk"), s.opsCancelBulk)
	ops.GET("/slots/:id/production", requirePermission(security.PermKitchenBoard), s.opsProduction)
}
