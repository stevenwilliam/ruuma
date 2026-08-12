// Package testenv builds a real ruuma stack against the ruuma_test database.
//
// The suites that use it (integration, security, e2e) exercise the actual
// router, the actual repositories and the actual constraints — the concurrency
// and tenancy rules only mean something when the database is real.
package testenv

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	_ "github.com/jackc/pgx/v5/stdlib"

	dbpkg "github.com/stevenwilliam/ruuma/db"

	adapterhttp "github.com/stevenwilliam/ruuma/internal/adapter/http"
	"github.com/stevenwilliam/ruuma/internal/adapter/postgres"
	"github.com/stevenwilliam/ruuma/internal/app/adminsvc"
	"github.com/stevenwilliam/ruuma/internal/app/authsvc"
	"github.com/stevenwilliam/ruuma/internal/app/catalogsvc"
	"github.com/stevenwilliam/ruuma/internal/app/opssvc"
	"github.com/stevenwilliam/ruuma/internal/app/ordersvc"
	"github.com/stevenwilliam/ruuma/internal/app/paymentsvc"
	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/identity"
	"github.com/stevenwilliam/ruuma/internal/platform/database"
	"github.com/stevenwilliam/ruuma/internal/platform/migrate"
	"github.com/stevenwilliam/ruuma/internal/platform/ratelimit"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

const signingKey = "test-signing-key-at-least-32-bytes-long!!"

// testEnvLockID is an arbitrary but fixed key for the advisory lock that keeps
// two test environments from sharing the database at the same time.
const testEnvLockID int64 = 0x7275756D61 // "ruuma"

// Env is a running stack.
type Env struct {
	T        *testing.T
	DB       *gorm.DB
	Server   *httptest.Server
	Signer   *security.TokenSigner
	Params   *postgres.ParamRepo
	Stores   *postgres.StoreRepo
	Orders   *postgres.OrderRepo
	Slots    *postgres.SlotRepo
	Payments *postgres.PaymentRepo
	Storage  ports.Storage
	Mail     *CapturedMailer
	Notify   *CapturedNotifier
	Fixtures *Fixtures
	Now      time.Time
}

// fixedClock keeps every time-dependent rule reproducible (docs/07 §4).
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// CapturedMailer records instead of sending.
type CapturedMailer struct {
	Sent []struct{ To, Subject, Body string }
}

func (m *CapturedMailer) Send(_ context.Context, to, subject, body string) error {
	m.Sent = append(m.Sent, struct{ To, Subject, Body string }{to, subject, body})
	return nil
}

// CapturedNotifier records queued notifications so a test can assert what the
// customer would have been told (BR-2.10.3).
type CapturedNotifier struct{ Queued []ports.QueuedNotification }

func (n *CapturedNotifier) Queue(_ context.Context, msg ports.QueuedNotification) error {
	n.Queued = append(n.Queued, msg)
	return nil
}

// memoryStorage stands in for MinIO so the suites do not need object storage
// running; the real adapter has its own tests.
type memoryStorage struct{ objects map[string][]byte }

func (m *memoryStorage) PutProof(_ context.Context, prefix string, data []byte) (string, error) {
	key := fmt.Sprintf("%s/%s", prefix, uuid.NewString())
	m.objects[key] = data
	return key, nil
}

func (m *memoryStorage) PutPhoto(ctx context.Context, prefix string, data []byte) (string, error) {
	return m.PutProof(ctx, prefix, data)
}

func (m *memoryStorage) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}

// DSN resolves the test database, defaulting to the local ruuma_test.
func DSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://ruuma@127.0.0.1:5432/ruuma_test?sslmode=disable"
}

// New migrates a clean schema, loads fixtures and starts the router.
// New builds the stack with permissive rate limits, so a test measures the
// behaviour it is about rather than the throttle.
func New(t *testing.T) *Env { return newEnv(t, permissiveLimits()) }

// NewWithLimits builds the stack with explicit limits, for the suite that
// tests throttling itself.
func NewWithLimits(t *testing.T, limits adapterhttp.Limits) *Env {
	return newEnv(t, limits)
}

// permissiveLimits effectively disables throttling. The real rules are proven
// with their real values in test/security/ratelimit_test.go.
func permissiveLimits() adapterhttp.Limits {
	wide := ratelimit.Rule{Burst: 100000, Window: time.Minute}
	return adapterhttp.Limits{
		Login: wide, StaffLogin: wide, OTPRequest: wide, OTPVerify: wide,
		Tracking: wide, OrderCreate: wide, MenuRead: wide,
	}
}

func newEnv(t *testing.T, limits adapterhttp.Limits) *Env {
	t.Helper()
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	conn, err := sql.Open("pgx", DSN())
	if err != nil {
		t.Skipf("test database unavailable: %v", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		t.Skipf("test database unavailable: %v", err)
	}

	// Every environment truncates and re-seeds the one test database, so two
	// running at once would pull the fixtures out from under each other. Go
	// runs packages in parallel by default, which makes that easy to trip into
	// — a suite that fails only when something else is running is worse than
	// no suite. A session-level advisory lock serialises them across processes.
	lock, err := sql.Open("pgx", DSN())
	if err != nil {
		t.Fatalf("open lock connection: %v", err)
	}
	lockConn, err := lock.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire lock connection: %v", err)
	}
	if _, err := lockConn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, testEnvLockID); err != nil {
		t.Fatalf("acquire test lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lockConn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, testEnvLockID)
		_ = lockConn.Close()
		_ = lock.Close()
	})

	if _, err := migrate.Up(ctx, conn, nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = conn.Close()

	db, err := database.Open(ctx, database.Options{URL: DSN(), MaxOpenConns: 20}, log)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })

	truncate(t, db)

	env := &Env{
		T: t, DB: db,
		Mail:   &CapturedMailer{},
		Notify: &CapturedNotifier{},
		// A Monday at 08:00 Jakarta: inside opening hours, before the lunch
		// slots, so lead-time and cutoff rules behave predictably.
		Now: time.Date(2026, 8, 3, 1, 0, 0, 0, time.UTC),
	}
	clk := fixedClock{t: env.Now}

	env.Params = postgres.NewParamRepo(db)
	if err := env.Params.Reload(ctx); err != nil {
		t.Fatalf("load params: %v", err)
	}
	env.Stores = postgres.NewStoreRepo(db, env.Params)
	env.Slots = postgres.NewSlotRepo(db)
	env.Orders = postgres.NewOrderRepo(db, env.Slots)
	env.Payments = postgres.NewPaymentRepo(db, env.Orders)
	catalogRepo := postgres.NewCatalogRepo(db)
	promoRepo := postgres.NewPromoRepo(db)
	customerRepo := postgres.NewCustomerRepo(db)
	userRepo := postgres.NewUserRepo(db)
	tokenRepo := postgres.NewTokenRepo(db)
	auditRepo := postgres.NewAuditRepo(db)
	idemRepo := postgres.NewIdempotencyRepo(db)

	storesPort := postgres.NewStoresPort(env.Stores, db)
	staffPort := postgres.NewStaffPort(userRepo, env.Stores)
	customersPort := postgres.NewCustomersPort(customerRepo)
	paymentsPort := postgres.NewPaymentsPort(env.Payments, clk.Now)
	cataloguePort := postgres.NewCataloguePort(catalogRepo)
	slotsPort := postgres.NewSlotsPort(env.Slots)
	ordersPort := postgres.NewOrdersPort(env.Orders, env.Stores, db)
	promoPort := postgres.NewPromotionsPort(promoRepo)
	auditPort := postgres.NewAuditPort(auditRepo)
	tokensPort := postgres.NewTokensPort(tokenRepo)

	env.Storage = &memoryStorage{objects: map[string][]byte{}}
	env.Signer = security.NewTokenSigner(signingKey, "", "ruuma", 15*time.Minute, clk.Now)

	catalogSvc := catalogsvc.New(storesPort, cataloguePort, slotsPort, env.Params, clk)
	orderSvc := ordersvc.New(storesPort, cataloguePort, slotsPort, ordersPort, paymentsPort,
		promoPort, customersPort, env.Params, auditPort, env.Notify, clk)
	paymentSvc := paymentsvc.New(paymentsPort, ordersPort, storesPort, env.Storage,
		env.Notify, env.Params, auditPort, clk)
	opsSvc := opssvc.New(ordersPort, storesPort, slotsPort, env.Notify, env.Params, auditPort, clk)
	adminSvc := adminsvc.New(storesPort, postgres.NewStoreWritePort(env.Stores, db),
		postgres.NewMenuWritePort(catalogRepo, db), postgres.NewParamWritePort(env.Params),
		staffPort, postgres.NewAuditReadPort(auditRepo), auditPort, env.Storage, clk)
	authSvc := authsvc.New(customersPort, staffPort, tokensPort, env.Notify, env.Mail,
		env.Params, auditPort, clk, env.Signer, "http://test.local",
		map[identity.Provider]authsvc.OAuthClient{})

	engine := adapterhttp.New(adapterhttp.Deps{
		Catalog: catalogSvc, Orders: orderSvc, Payments: paymentSvc, Auth: authSvc,
		Ops: opsSvc, Admin: adminSvc, Stores: storesPort, Staff: staffPort,
		Customers: customersPort, PaymentsRead: paymentsPort,
		Signer: env.Signer, Limiter: ratelimit.New(clk.Now), Params: env.Params,
		// Permissive limits so a concurrency test measures capacity, not the
		// throttle. The rate-limit rules themselves are proven in
		// test/security/ratelimit_test.go with their real values.
		Limits:      limits,
		Idempotency: idempotencyAdapter{idemRepo},
		Log:         log, IsProduction: false,
		Origins: []string{"http://test.local"}, Version: "test",
		Ready: func() error { return nil },
	})

	env.Server = httptest.NewServer(engine)
	t.Cleanup(env.Server.Close)

	env.seed(ctx)
	return env
}

type idempotencyAdapter struct{ repo *postgres.IdempotencyRepo }

func (i idempotencyAdapter) Begin(ctx context.Context, key, subjectType string, subjectID uuid.UUID,
	endpoint string, body []byte) (*adapterhttp.StoredResponse, error) {
	stored, err := i.repo.Begin(ctx, key, subjectType, subjectID, endpoint, body)
	if err != nil || stored == nil {
		return nil, err
	}
	return &adapterhttp.StoredResponse{Code: stored.Code, Body: stored.Body}, nil
}

func (i idempotencyAdapter) Complete(ctx context.Context, key, subjectType string, subjectID uuid.UUID,
	endpoint string, code int, body []byte) error {
	return i.repo.Complete(ctx, key, subjectType, subjectID, endpoint, code, body)
}

func (i idempotencyAdapter) Abandon(ctx context.Context, key, subjectType string, subjectID uuid.UUID,
	endpoint string) error {
	return i.repo.Abandon(ctx, key, subjectType, subjectID, endpoint)
}

// truncate empties every business table. TRUNCATE bypasses the row-level
// append-only triggers, which is exactly why the fixtures can be reloaded while
// the triggers still protect the tables at runtime (BR-2.10.2).
func truncate(t *testing.T, db *gorm.DB) {
	t.Helper()
	tables := []string{
		"payment_events", "payments", "order_line_options", "order_lines", "order_events",
		"promotion_redemptions", "orders", "slots", "item_daily_stock", "item_86s",
		"item_availability_rules", "store_menu_overrides", "favourites", "option_choices",
		"option_groups", "menu_items", "categories", "promotion_categories", "promotion_stores",
		"promotions", "delivery_zones", "store_date_overrides", "store_blackout_dates",
		"store_bank_accounts", "store_parameters", "store_hours", "store_fulfilment_modes",
		"staff_store_assignments", "refresh_tokens", "verification_tokens", "otp_codes",
		"customer_identities", "addresses", "customers", "audit_log", "notifications",
		"idempotency_keys", "stores", "users",
	}
	if err := db.Exec("TRUNCATE " + strings.Join(tables, ", ") + " CASCADE").Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// CASCADE reaches sys_parameters through its updated_by foreign key, so the
	// reference data — rates, capacities and every notification template — goes
	// with it. Put it back, or the suite silently tests a service running on
	// compiled fallbacks with no templates (BR-1.4.4, BR-2.10.5).
	data, err := db.DB()
	_ = data
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	reference, err := dbpkg.DataMigrations()
	if err != nil {
		t.Fatalf("reference data: %v", err)
	}
	for _, m := range reference {
		if err := db.Exec(m.Up).Error; err != nil {
			t.Fatalf("restore %s: %v", m.Name, err)
		}
	}
}

// ── HTTP helpers ─────────────────────────────────────────────────────────────

// Response is a decoded HTTP response.
type Response struct {
	Status int
	Body   map[string]any
	Raw    string
}

// Code returns the error code from the single error envelope (docs/04 §2).
func (r Response) Code() string {
	if errObj, ok := r.Body["error"].(map[string]any); ok {
		if code, ok := errObj["code"].(string); ok {
			return code
		}
	}
	return ""
}

// Do performs a request. token may be empty for an anonymous call.
func (e *Env) Do(method, path, token string, body any, headers ...[2]string) Response {
	e.T.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.T.Fatalf("marshal: %v", err)
		}
		reader = strings.NewReader(string(raw))
	}

	req, err := http.NewRequest(method, e.Server.URL+path, reader)
	if err != nil {
		e.T.Fatalf("request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, h := range headers {
		req.Header.Set(h[0], h[1])
	}

	res, err := e.Server.Client().Do(req)
	if err != nil {
		e.T.Fatalf("do: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, _ := io.ReadAll(res.Body)
	out := Response{Status: res.StatusCode, Raw: string(raw)}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
}

// Idempotent performs a request with a fresh Idempotency-Key.
func (e *Env) Idempotent(method, path, token string, body any) Response {
	return e.Do(method, path, token, body, [2]string{"Idempotency-Key", uuid.NewString()})
}
