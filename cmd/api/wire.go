package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/adapter/mail"
	"github.com/stevenwilliam/ruuma/internal/adapter/notify"
	"github.com/stevenwilliam/ruuma/internal/adapter/postgres"
	"github.com/stevenwilliam/ruuma/internal/adapter/storage"
	"github.com/stevenwilliam/ruuma/internal/app/adminsvc"
	"github.com/stevenwilliam/ruuma/internal/app/authsvc"
	"github.com/stevenwilliam/ruuma/internal/app/catalogsvc"
	"github.com/stevenwilliam/ruuma/internal/app/notifysvc"
	"github.com/stevenwilliam/ruuma/internal/app/opssvc"
	"github.com/stevenwilliam/ruuma/internal/app/ordersvc"
	"github.com/stevenwilliam/ruuma/internal/app/paymentsvc"
	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/domain/identity"
	"github.com/stevenwilliam/ruuma/internal/platform/clock"
	"github.com/stevenwilliam/ruuma/internal/platform/config"
	"github.com/stevenwilliam/ruuma/internal/platform/database"
	"github.com/stevenwilliam/ruuma/internal/platform/ratelimit"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// app is everything wired together. Composition happens here, once, so no
// package below has to know how its collaborators are built.
type app struct {
	cfg *config.Config
	log *slog.Logger
	db  *gorm.DB
	clk ports.Clock

	params   *postgres.ParamRepo
	stores   *postgres.StoreRepo
	slots    *postgres.SlotRepo
	orders   *postgres.OrderRepo
	payments *postgres.PaymentRepo
	catalog  *postgres.CatalogRepo
	promos   *postgres.PromoRepo
	customer *postgres.CustomerRepo
	users    *postgres.UserRepo
	tokens   *postgres.TokenRepo
	audit    *postgres.AuditRepo
	notifyR  *postgres.NotifyRepo
	idem     *postgres.IdempotencyRepo

	storesPort   *postgres.StoresPort
	staffPort    *postgres.StaffPort
	customerPort *postgres.CustomersPort
	paymentsPort *postgres.PaymentsPort

	catalogSvc  *catalogsvc.Service
	orderSvc    *ordersvc.Service
	paymentSvc  *paymentsvc.Service
	authSvc     *authsvc.Service
	opsSvc      *opssvc.Service
	adminSvc    *adminsvc.Service
	notifySvc   *notifysvc.Service
	notifyQueue *postgres.NotifyQueuePort

	signer  *security.TokenSigner
	limiter *ratelimit.Limiter
	storage *storage.Client
}

// systemClock injects time everywhere; the domain never calls time.Now
// (docs/05 §3.3).
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

func build(ctx context.Context, cfg *config.Config, log *slog.Logger) (*app, error) {
	db, err := database.Open(ctx, database.Options{
		URL:          cfg.Database.URL,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
		Debug:        !cfg.App.IsProduction(),
	}, log)
	if err != nil {
		return nil, err
	}

	a := &app{cfg: cfg, log: log, db: db, clk: systemClock{}}

	a.params = postgres.NewParamRepo(db)
	if err := a.params.Reload(ctx); err != nil {
		return nil, fmt.Errorf("load parameters: %w", err)
	}
	a.stores = postgres.NewStoreRepo(db, a.params)
	a.slots = postgres.NewSlotRepo(db)
	a.orders = postgres.NewOrderRepo(db, a.slots)
	a.payments = postgres.NewPaymentRepo(db, a.orders)
	a.catalog = postgres.NewCatalogRepo(db)
	a.promos = postgres.NewPromoRepo(db)
	a.customer = postgres.NewCustomerRepo(db)
	a.users = postgres.NewUserRepo(db)
	a.tokens = postgres.NewTokenRepo(db)
	a.audit = postgres.NewAuditRepo(db)
	a.notifyR = postgres.NewNotifyRepo(db)
	a.idem = postgres.NewIdempotencyRepo(db)

	a.storesPort = postgres.NewStoresPort(a.stores, db)
	a.staffPort = postgres.NewStaffPort(a.users, a.stores)
	a.customerPort = postgres.NewCustomersPort(a.customer)
	a.paymentsPort = postgres.NewPaymentsPort(a.payments, nil)
	cataloguePort := postgres.NewCataloguePort(a.catalog)
	slotsPort := postgres.NewSlotsPort(a.slots)
	ordersPort := postgres.NewOrdersPort(a.orders, a.stores, db)
	promoPort := postgres.NewPromotionsPort(a.promos)
	auditPort := postgres.NewAuditPort(a.audit)
	notifyPort := postgres.NewNotifyPort(a.notifyR)
	tokensPort := postgres.NewTokensPort(a.tokens)
	a.notifyQueue = postgres.NewNotifyQueuePort(a.notifyR)

	a.storage, err = storage.New(ctx, storage.Config{
		Endpoint: cfg.Storage.Endpoint, PublicEndpoint: cfg.Storage.PublicEndpoint,
		AccessKey: cfg.Storage.AccessKey, SecretKey: cfg.Storage.SecretKey,
		Bucket: cfg.Storage.Bucket, UseSSL: cfg.Storage.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	storagePort := storageAdapter{a.storage}

	mailer := mail.New(mail.Config{
		Host: cfg.Mail.Host, Port: cfg.Mail.Port, Username: cfg.Mail.Username,
		Password: cfg.Mail.Password, FromEmail: cfg.Mail.FromEmail,
		FromName: cfg.Mail.FromName, TLS: cfg.Mail.TLS,
	})

	a.signer = security.NewTokenSigner(cfg.Auth.SigningKey, cfg.Auth.PreviousKey,
		cfg.Auth.Issuer,
		time.Duration(a.params.Int(ctx, nil, "auth.access_token_minutes"))*time.Minute, nil)
	a.limiter = ratelimit.New(nil)

	a.catalogSvc = catalogsvc.New(a.storesPort, cataloguePort, slotsPort, a.params, a.clk)
	a.orderSvc = ordersvc.New(a.storesPort, cataloguePort, slotsPort, ordersPort,
		a.paymentsPort, promoPort, a.customerPort, a.params, auditPort, a.clk)
	a.paymentSvc = paymentsvc.New(a.paymentsPort, ordersPort, a.storesPort, storagePort,
		notifyPort, a.params, auditPort, a.clk)
	a.opsSvc = opssvc.New(ordersPort, a.storesPort, slotsPort, notifyPort, a.params, auditPort, a.clk)
	a.adminSvc = adminsvc.New(a.storesPort, postgres.NewStoreWritePort(a.stores, db),
		postgres.NewMenuWritePort(a.catalog, db), postgres.NewParamWritePort(a.params),
		a.staffPort, postgres.NewAuditReadPort(a.audit), auditPort, storagePort, a.clk)

	oauth := map[identity.Provider]authsvc.OAuthClient{}
	a.authSvc = authsvc.New(a.customerPort, a.staffPort, tokensPort, notifyPort, mailer,
		a.params, auditPort, a.clk, a.signer, cfg.App.BaseURL, oauth)

	sender := notifySender{notify.Resolve(
		a.params.String(ctx, nil, "notify.provider"),
		notify.NewWAHA(cfg.WhatsApp.WAHAURL, cfg.WhatsApp.WAHASession, cfg.WhatsApp.WAHAAPIKey),
		notify.NewMetaCloud(cfg.WhatsApp.MetaPhoneID, cfg.WhatsApp.MetaToken),
		notify.NewLogProvider(log),
	)}
	a.notifySvc = notifysvc.New(a.notifyQueue, sender, a.params, log)

	return a, nil
}

func (a *app) close() {
	if a.db != nil {
		_ = database.Close(a.db)
	}
}

// storageAdapter narrows the MinIO client to the app's Storage port.
type storageAdapter struct{ c *storage.Client }

func (s storageAdapter) PutProof(ctx context.Context, prefix string, data []byte) (string, error) {
	return s.c.Put(ctx, storage.KindProof, prefix, data)
}

func (s storageAdapter) PutPhoto(ctx context.Context, prefix string, data []byte) (string, error) {
	return s.c.Put(ctx, storage.KindPhoto, prefix, data)
}

func (s storageAdapter) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return s.c.PresignGet(ctx, key, ttl)
}

// notifySender narrows a notify.Provider to the dispatcher's Sender.
type notifySender struct{ p notify.Provider }

func (n notifySender) Name() string { return n.p.Name() }

func (n notifySender) Send(ctx context.Context, to, body string) error {
	return n.p.Send(ctx, notify.Message{To: to, Body: body})
}

var _ = clock.Jakarta
