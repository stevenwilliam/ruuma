// Package logging provides structured JSON logging with a request id carried
// through context and PII redaction at write time (docs/12, A09).
//
// Phone numbers, emails, addresses and object keys must never reach a log line
// or a URL; the redacting handler enforces that centrally rather than trusting
// every call site.
package logging

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyLogger
)

// Sensitive attribute keys are replaced wholesale.
var sensitiveKeys = map[string]bool{
	"phone": true, "contact_phone": true, "msisdn": true, "target": true,
	"email": true, "contact_email": true, "password": true, "password_hash": true,
	"token": true, "access_token": true, "refresh_token": true, "authorization": true,
	"api_key": true, "secret": true, "signing_key": true, "otp": true, "code_hash": true,
	"address": true, "address_line": true, "proof_object_key": true, "photo_key": true,
	"account_number": true, "sender_name": true,
}

var (
	emailRe = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phoneRe = regexp.MustCompile(`\+?62[0-9]{8,13}|0[0-9]{9,13}`)
)

// New builds the application logger. level is one of debug|info|warn|error.
func New(level string, jsonOutput bool) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lv, ReplaceAttr: redactAttr}
	var h slog.Handler
	if jsonOutput {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

// redactAttr masks sensitive keys and scrubs PII patterns from free text.
func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if sensitiveKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, "[redacted]")
	}
	if a.Value.Kind() == slog.KindString {
		if s := Scrub(a.Value.String()); s != a.Value.String() {
			return slog.String(a.Key, s)
		}
	}
	return a
}

// Scrub removes email addresses and Indonesian phone numbers from free text so
// an error message quoting user input cannot leak PII (docs/12, A09).
func Scrub(s string) string {
	s = emailRe.ReplaceAllString(s, "[email]")
	s = phoneRe.ReplaceAllString(s, "[phone]")
	return s
}

// WithRequestID stores a request id in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestID reads the request id, or "" if unset.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// WithLogger stores a logger in the context.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger, l)
}

// From returns the context logger, decorated with the request id, falling back
// to the default logger.
func From(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(ctxKeyLogger).(*slog.Logger)
	if !ok || l == nil {
		l = slog.Default()
	}
	if rid := RequestID(ctx); rid != "" {
		l = l.With("request_id", rid)
	}
	return l
}
