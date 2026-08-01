// Package notify sends customer messages through a provider port.
//
// Phase 1 ships WAHA (self-hosted, unofficial WhatsApp-Web gateway) with the
// ban risk accepted by the owner (D11); meta_cloud is the documented production
// swap and log is the dev sink. Switching provider is a sys_parameters change,
// not a deploy (docs/09 §8).
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Message is one outbound notification.
type Message struct {
	To   string // E.164 phone
	Body string
}

// Provider sends a message. Implementations must be safe for concurrent use.
type Provider interface {
	Name() string
	Send(ctx context.Context, m Message) error
}

// Event names the four automated messages ruuma sends (D28). Payment rejection
// is deliberately absent: finance and operations handle that by hand.
type Event string

const (
	EventOrderReceived   Event = "order_received"
	EventPaymentVerified Event = "payment_verified"
	EventOrderReady      Event = "order_ready"
	EventSlotReminder    Event = "slot_reminder"
)

// AllEvents lists the automated events, for the admin toggles.
func AllEvents() []Event {
	return []Event{EventOrderReceived, EventPaymentVerified, EventOrderReady, EventSlotReminder}
}

// Render fills {{placeholders}} in a template stored in sys_parameters
// (BR-2.10.5). An unknown placeholder is left untouched rather than blanked, so
// a typo in a template is visible instead of silently producing empty text.
func Render(template string, vars map[string]string) string {
	out := template
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

// ── WAHA ─────────────────────────────────────────────────────────────────────

// WAHA talks to the self-hosted WhatsApp HTTP API (D7, D11).
type WAHA struct {
	baseURL string
	session string
	apiKey  string
	client  *http.Client
}

func NewWAHA(baseURL, session, apiKey string) *WAHA {
	return &WAHA{
		baseURL: strings.TrimRight(baseURL, "/"),
		session: session,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (w *WAHA) Name() string { return "waha" }

func (w *WAHA) Send(ctx context.Context, m Message) error {
	payload := map[string]any{
		"session": w.session,
		"chatId":  toChatID(m.To),
		"text":    m.Body,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.baseURL+"/api/sendText",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.apiKey != "" {
		req.Header.Set("X-Api-Key", w.apiKey)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("waha: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		// The response can echo the message, so it is never logged verbatim
		// beyond this bounded snippet (docs/12, A09).
		return fmt.Errorf("waha: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// toChatID converts +6281… into WhatsApp's 6281…@c.us form.
func toChatID(phone string) string {
	digits := strings.TrimPrefix(strings.TrimSpace(phone), "+")
	return digits + "@c.us"
}

// ── Meta Cloud API (production alternative, D11) ─────────────────────────────

// MetaCloud is the official WhatsApp Business API client. It is wired but
// unused in phase 1; enabling it is a sys_parameters switch plus credentials.
type MetaCloud struct {
	phoneNumberID string
	accessToken   string
	client        *http.Client
}

func NewMetaCloud(phoneNumberID, accessToken string) *MetaCloud {
	return &MetaCloud{
		phoneNumberID: phoneNumberID,
		accessToken:   accessToken,
		client:        &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MetaCloud) Name() string { return "meta_cloud" }

func (m *MetaCloud) Send(ctx context.Context, msg Message) error {
	if m.phoneNumberID == "" || m.accessToken == "" {
		return fmt.Errorf("meta_cloud: credentials are not configured")
	}
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                strings.TrimPrefix(msg.To, "+"),
		"type":              "text",
		"text":              map[string]string{"body": msg.Body},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://graph.facebook.com/v20.0/%s/messages", m.phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.accessToken)

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("meta_cloud: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("meta_cloud: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// ── Log sink ─────────────────────────────────────────────────────────────────

// LogProvider records instead of sending. It is the dev default so a local run
// never touches the shared WAHA session (docs/11 §5).
type LogProvider struct{ log *slog.Logger }

func NewLogProvider(log *slog.Logger) *LogProvider { return &LogProvider{log: log} }

func (l *LogProvider) Name() string { return "log" }

func (l *LogProvider) Send(_ context.Context, m Message) error {
	if l.log != nil {
		// The phone is redacted by the logging handler; the body is not logged
		// in full because it contains the customer's name and order details.
		l.log.Info("notification (log provider)", "target", m.To, "length", len(m.Body))
	}
	return nil
}

// Resolve picks a provider by name, falling back to the log sink so a
// misconfigured value can never take ordering down.
func Resolve(name string, waha *WAHA, meta *MetaCloud, fallback *LogProvider) Provider {
	switch name {
	case "waha":
		if waha != nil {
			return waha
		}
	case "meta_cloud":
		if meta != nil {
			return meta
		}
	}
	return fallback
}
