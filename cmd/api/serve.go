package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	router "github.com/stevenwilliam/ruuma/internal/adapter/http"

	"github.com/stevenwilliam/ruuma/internal/adapter/postgres"
	"github.com/stevenwilliam/ruuma/internal/platform/config"
)

func runServe(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	a, err := build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer a.close()

	engine := router.New(router.Deps{
		Catalog: a.catalogSvc, Orders: a.orderSvc, Payments: a.paymentSvc,
		Auth: a.authSvc, Ops: a.opsSvc, Admin: a.adminSvc,
		Stores: a.storesPort, Staff: a.staffPort, Customers: a.customerPort,
		PaymentsRead: a.paymentsPort,
		Signer:       a.signer, Limiter: a.limiter, Limits: a.rateLimits(ctx),
		Idempotency: idempotencyAdapter{a.idem},
		Log:         log, IsProduction: cfg.App.IsProduction(), Origins: cfg.App.AllowedOrigins,
		Version: version, Commit: commit,
		Ready: func() error {
			sqlDB, err := a.db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Ping()
		},
	})

	// Read/write/idle timeouts bound every connection (docs/12).
	srv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", cfg.App.Port),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	// /metrics is bound to loopback only and never proxied (docs/12, A05).
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", router.MetricsHandler())
	metricsSrv := &http.Server{
		Addr:              "127.0.0.1:9090",
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		log.Info("api listening", "addr", srv.Addr, "env", cfg.App.Env, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn("metrics listener stopped", "error", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	// Graceful shutdown drains in-flight requests (docs/05 §7).
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()
	_ = metricsSrv.Shutdown(shutdownCtx)
	return srv.Shutdown(shutdownCtx)
}

// idempotencyAdapter maps the repository's response type to the router's.
type idempotencyAdapter struct{ repo *postgres.IdempotencyRepo }

func (i idempotencyAdapter) Begin(ctx context.Context, key, subjectType string,
	subjectID uuid.UUID, endpoint string, body []byte) (*router.StoredResponse, error) {

	stored, err := i.repo.Begin(ctx, key, subjectType, subjectID, endpoint, body)
	if err != nil || stored == nil {
		return nil, err
	}
	return &router.StoredResponse{Code: stored.Code, Body: stored.Body}, nil
}

func (i idempotencyAdapter) Complete(ctx context.Context, key, subjectType string,
	subjectID uuid.UUID, endpoint string, code int, body []byte) error {
	return i.repo.Complete(ctx, key, subjectType, subjectID, endpoint, code, body)
}

func (i idempotencyAdapter) Abandon(ctx context.Context, key, subjectType string,
	subjectID uuid.UUID, endpoint string) error {
	return i.repo.Abandon(ctx, key, subjectType, subjectID, endpoint)
}
