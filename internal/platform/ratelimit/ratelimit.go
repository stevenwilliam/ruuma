// Package ratelimit implements per-key token buckets for the limits in
// docs/04 §9 (login, OTP, tracking, promo validation, order creation).
//
// Limits are applied per identifier AND per IP (docs/12, A07) — the caller
// composes two keys rather than the limiter guessing.
package ratelimit

import (
	"sync"
	"time"
)

// Rule is a limit of Burst requests refilling at Rate per Window.
type Rule struct {
	Burst  int
	Window time.Duration
}

// Result reports the outcome of a single Allow call.
type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// Limiter is an in-memory token-bucket limiter, safe for concurrent use.
//
// In-memory is the right scope for a single-node deployment (docs/09 §1). If
// ruuma ever runs multiple API nodes, this type is the seam where a shared
// store goes — the interface does not change.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
	ttl     time.Duration
}

func New(now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		now:     now,
		ttl:     time.Hour,
	}
}

// Allow consumes one token for key under rule.
func (l *Limiter) Allow(key string, rule Rule) Result {
	if rule.Burst <= 0 || rule.Window <= 0 {
		return Result{Allowed: true}
	}
	now := l.now()
	refillPerSecond := float64(rule.Burst) / rule.Window.Seconds()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(rule.Burst), lastSeen: now}
		l.buckets[key] = b
	} else {
		elapsed := now.Sub(b.lastSeen).Seconds()
		if elapsed > 0 {
			b.tokens = minFloat(float64(rule.Burst), b.tokens+elapsed*refillPerSecond)
			b.lastSeen = now
		}
	}

	if b.tokens < 1 {
		deficit := 1 - b.tokens
		return Result{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: time.Duration(deficit/refillPerSecond*float64(time.Second)) + time.Second,
		}
	}

	b.tokens--
	return Result{Allowed: true, Remaining: int(b.tokens)}
}

// Reset clears a key — used after a successful login so a legitimate user is
// not punished for earlier typos.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.buckets, key)
	l.mu.Unlock()
}

// Sweep drops buckets untouched for longer than the TTL. Call periodically from
// the worker so an attacker cannot grow the map without bound.
func (l *Limiter) Sweep() int {
	cutoff := l.now().Add(-l.ttl)
	l.mu.Lock()
	defer l.mu.Unlock()
	removed := 0
	for k, b := range l.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(l.buckets, k)
			removed++
		}
	}
	return removed
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Default rules from docs/04 §9. They are overridable at runtime through
// sys_parameters; these are the compiled fallbacks (BR-1.4.4).
var (
	RuleLogin        = Rule{Burst: 5, Window: time.Minute}
	RuleStaffLogin   = Rule{Burst: 5, Window: time.Minute}
	RuleOTPRequest   = Rule{Burst: 3, Window: 10 * time.Minute}
	RuleOTPRequestIP = Rule{Burst: 10, Window: time.Hour}
	RuleOTPVerify    = Rule{Burst: 5, Window: 10 * time.Minute}
	RuleTracking     = Rule{Burst: 20, Window: time.Minute}
	RulePromo        = Rule{Burst: 10, Window: time.Minute}
	RuleOrderCreate  = Rule{Burst: 10, Window: time.Minute}
	RuleMenuRead     = Rule{Burst: 120, Window: time.Minute}
)
