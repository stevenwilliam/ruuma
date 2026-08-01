// Package metrics exposes Prometheus instrumentation. /metrics is bound to
// localhost and never proxied publicly (docs/12, A05).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPDuration is the request histogram, labelled by route template (never
	// the raw path — a raw path can contain an order code).
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ruuma_http_request_duration_seconds",
		Help:    "HTTP request duration by route and status.",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1, 2, 5},
	}, []string{"method", "route", "status"})

	// OrdersCreated counts order creations by outcome, so a spike in SLOT_FULL
	// is visible without reading logs.
	OrdersCreated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruuma_orders_created_total",
		Help: "Order creation attempts by store and outcome.",
	}, []string{"store", "outcome"})

	// SlotRejections counts why a slot was refused (BR-2.3.6 reason codes).
	SlotRejections = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruuma_slot_rejections_total",
		Help: "Slot booking rejections by reason.",
	}, []string{"store", "reason"})

	// PaymentDecisions counts finance verify/reject/refund actions.
	PaymentDecisions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruuma_payment_decisions_total",
		Help: "Finance payment decisions by store and decision.",
	}, []string{"store", "decision"})

	// PaymentQueueAge reports the age of the oldest pending verification, which
	// is what the finance SLA alarm watches (BR-2.9.1).
	PaymentQueueAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ruuma_payment_queue_oldest_seconds",
		Help: "Age of the oldest payment awaiting verification, per store.",
	}, []string{"store"})

	// NotificationSends counts notification results by provider and event.
	NotificationSends = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruuma_notification_sends_total",
		Help: "Notification sends by provider, event and result.",
	}, []string{"provider", "event", "result"})

	// AuthFailures backs the auth-failure-spike alert (docs/12, A09).
	AuthFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruuma_auth_failures_total",
		Help: "Authentication failures by kind.",
	}, []string{"kind"})

	// RateLimitHits counts throttled requests by rule.
	RateLimitHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruuma_rate_limit_hits_total",
		Help: "Requests refused by the rate limiter, by rule.",
	}, []string{"rule"})
)
