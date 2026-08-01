// Package notifysvc renders and dispatches customer notifications (BR-2.10.3/4).
//
// Only four events are automated (D28); payment rejection is deliberately not
// one of them — finance and operations call the customer instead.
package notifysvc

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
)

// Sender is the outbound channel (WAHA, Meta Cloud or the log sink).
type Sender interface {
	Name() string
	Send(ctx context.Context, to, body string) error
}

// Queued is one message the dispatcher should attempt.
type Queued struct {
	ID          uuid.UUID
	Target      string
	Body        string
	TemplateKey string
	Language    string
	Attempts    int
}

// Store is the notification queue.
type Store interface {
	Due(ctx context.Context, limit int) ([]Queued, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, cause string, attempt int) error
}

type Service struct {
	store  Store
	sender Sender
	params ports.Params
	log    *slog.Logger
}

func New(store Store, sender Sender, params ports.Params, log *slog.Logger) *Service {
	return &Service{store: store, sender: sender, params: params, log: log}
}

// Dispatch sends everything due, recording each outcome. A failure is retried
// with backoff by the store, never lost silently (BR-2.10.4).
func (s *Service) Dispatch(ctx context.Context, limit int) (sent, failed int, err error) {
	due, err := s.store.Due(ctx, limit)
	if err != nil {
		return 0, 0, err
	}
	for _, m := range due {
		if err := s.sender.Send(ctx, m.Target, m.Body); err != nil {
			failed++
			_ = s.store.MarkFailed(ctx, m.ID, err.Error(), m.Attempts+1)
			if s.log != nil {
				s.log.Warn("notification failed", "id", m.ID, "provider", s.sender.Name(),
					"attempt", m.Attempts+1)
			}
			continue
		}
		sent++
		_ = s.store.MarkSent(ctx, m.ID)
	}
	return sent, failed, nil
}

// OrderVars builds the placeholder set for an order message (BR-2.10.5).
func OrderVars(o ports.OrderView, storeName, storeAddress, bankName, accountName, accountNumber string) map[string]string {
	return map[string]string{
		"name":           o.ContactName,
		"code":           o.OrderCode,
		"store":          storeName,
		"address":        storeAddress,
		"slot":           o.SlotStartsAt.Format("2006-01-02 15:04"),
		"total":          formatRupiah(int64(o.Total)),
		"amount_due":     formatRupiah(int64(o.AmountDue)),
		"unique_code":    strconv.Itoa(o.UniqueCode),
		"bank":           bankName,
		"account_name":   accountName,
		"account_number": accountNumber,
	}
}

// formatRupiah renders an integer amount the way Indonesians read it:
// 150000 → "150.000". Money stays an integer end to end (BR-1.1.4).
func formatRupiah(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	digits := strconv.FormatInt(v, 10)

	var out []byte
	for i, d := range []byte(digits) {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, d)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// ReminderWindow reports whether a slot is close enough to send its pre-slot
// reminder (BR-2.10.3).
func ReminderWindow(slotStartsAt, now time.Time, minutesBefore int) bool {
	target := slotStartsAt.Add(-time.Duration(minutesBefore) * time.Minute)
	return !now.Before(target) && now.Before(slotStartsAt)
}
