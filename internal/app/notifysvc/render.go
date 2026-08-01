package notifysvc

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
)

// Composer turns an event plus an order into a queued message.
//
// It lives here so the order and payment services do not each grow their own
// copy of template lookup, placeholder filling and the per-event switch
// (BR-2.10.3/5).
type Composer struct {
	params   ports.Params
	notifier ports.Notifier
}

func NewComposer(params ports.Params, notifier ports.Notifier) *Composer {
	return &Composer{params: params, notifier: notifier}
}

// Bank is the destination shown in the order-received message.
type Bank struct{ Name, AccountName, AccountNumber string }

// Enabled reports whether an event is switched on (BR-2.10.3).
func (c *Composer) Enabled(ctx context.Context, event Event) bool {
	return c.params.Bool(ctx, nil, "notify.event."+string(event)+"_enabled")
}

// Queue renders and queues one message. A disabled event is a no-op, and an
// empty template is reported rather than sending a message full of
// unsubstituted placeholders.
func (c *Composer) Queue(ctx context.Context, event Event, order ports.OrderView,
	storeName, storeAddress string, bank Bank) error {

	if !c.Enabled(ctx, event) {
		return nil
	}

	language := "id"
	key := "notify.template." + string(event) + "." + language
	template := c.params.String(ctx, nil, key)
	if strings.TrimSpace(template) == "" {
		return ErrTemplateMissing{Key: key}
	}

	vars := OrderVars(order, storeName, storeAddress, bank.Name, bank.AccountName, bank.AccountNumber)
	body := Render(template, vars)

	orderID := order.ID
	customerID := order.CustomerID
	return c.notifier.Queue(ctx, ports.QueuedNotification{
		OrderID: &orderID, CustomerID: &customerID,
		Channel:     "whatsapp",
		Provider:    c.params.String(ctx, nil, "notify.provider"),
		Event:       string(event),
		Target:      order.ContactPhone,
		TemplateKey: key, Language: language,
		Body: body,
	})
}

// ErrTemplateMissing means a message could not be rendered. It is an error and
// not a silent skip: a customer who is never told to pay does not pay.
type ErrTemplateMissing struct{ Key string }

func (e ErrTemplateMissing) Error() string { return "notify: template " + e.Key + " is empty" }

var _ = uuid.Nil
