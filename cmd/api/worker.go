package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/stevenwilliam/ruuma/internal/domain/schedule"
	"github.com/stevenwilliam/ruuma/internal/platform/clock"
	"github.com/stevenwilliam/ruuma/internal/platform/config"
	"github.com/stevenwilliam/ruuma/internal/platform/metrics"
)

// runWorker runs the background jobs: slot materialisation, notification
// dispatch, token cleanup and the finance-ageing gauge.
//
// It deliberately does NOT cancel orders. Phase 1 has no auto-cancel; only a
// human releases a customer's slot (BR-2.3.11, D25).
func runWorker(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	a, err := build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer a.close()

	log.Info("worker started")

	notifyTicker := time.NewTicker(15 * time.Second)
	slotTicker := time.NewTicker(10 * time.Minute)
	sweepTicker := time.NewTicker(time.Hour)
	defer notifyTicker.Stop()
	defer slotTicker.Stop()
	defer sweepTicker.Stop()

	// Do the slow work once at start so a fresh deployment has slots.
	a.materialiseSlots(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped")
			return nil

		case <-notifyTicker.C:
			sent, failed, err := a.notifySvc.Dispatch(ctx, 25)
			if err != nil {
				log.Warn("notification dispatch failed", "error", err)
			}
			if sent > 0 || failed > 0 {
				log.Info("notifications dispatched", "sent", sent, "failed", failed)
			}
			a.updateQueueGauge(ctx)

		case <-slotTicker.C:
			a.materialiseSlots(ctx)

		case <-sweepTicker.C:
			if err := a.tokens.SweepExpired(ctx); err != nil {
				log.Warn("token sweep failed", "error", err)
			}
			if n, err := a.idem.Sweep(ctx, 48*time.Hour); err == nil && n > 0 {
				log.Info("idempotency keys swept", "removed", n)
			}
			a.limiter.Sweep()
		}
	}
}

// materialiseSlots keeps a rolling window of slots for every active store and
// every mode it supports (BR-2.3.3/4).
func (a *app) materialiseSlots(ctx context.Context) {
	stores, err := a.stores.ListActive(ctx, "")
	if err != nil {
		a.log.Warn("slot materialisation: cannot list stores", "error", err)
		return
	}
	deliveryEnabled := a.params.Bool(ctx, nil, "fulfilment.delivery_enabled")

	for _, st := range stores {
		loc := clock.Location(st.Timezone)
		today := schedule.DateOf(a.clk.Now(), loc)
		params := a.stores.Params(ctx, st.ID)
		horizon := params.MaxAdvanceDays
		if horizon <= 0 {
			horizon = 14
		}

		modes, err := a.stores.Modes(ctx, st.ID)
		if err != nil {
			continue
		}
		sched, _, err := a.stores.LoadSchedule(ctx, st.ID, today, today.AddDays(horizon, loc))
		if err != nil {
			a.log.Warn("slot materialisation: schedule", "store", st.Code, "error", err)
			continue
		}

		created := 0
		for _, m := range modes {
			mode := schedule.FulfilmentType(m.FulfilmentType)
			if !m.IsEnabled {
				continue
			}
			if mode == schedule.Delivery && !deliveryEnabled {
				continue // phase 2 (D16)
			}
			for day := 0; day <= horizon; day++ {
				d := today.AddDays(day, loc)
				generated, reason := schedule.Generate(sched, d, mode)
				if reason != schedule.ReasonOK || len(generated) == 0 {
					continue // closed weekday, blackout or override — no slots at all
				}
				n, err := a.slots.Materialise(ctx, st.ID, generated, params)
				if err != nil {
					a.log.Warn("slot materialisation failed",
						"store", st.Code, "date", d.String(), "error", err)
					continue
				}
				created += n
			}
		}
		if created > 0 {
			a.log.Info("slots materialised", "store", st.Code, "created", created)
		}
	}
}

// updateQueueGauge publishes the finance-ageing metric the SLA alert watches
// (BR-2.9.1, docs/05 §5).
func (a *app) updateQueueGauge(ctx context.Context) {
	oldest, err := a.payments.OldestPending(ctx)
	if err != nil {
		return
	}
	for storeID, at := range oldest {
		metrics.PaymentQueueAge.WithLabelValues(storeID.String()).
			Set(time.Since(at).Seconds())
	}
}
