// Package mail sends transactional email over SMTP (mailpit in development,
// a real relay in production — docs/00 Q9).
package mail

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Config mirrors the SMTP settings in .env.example.
type Config struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	TLS       bool
}

// Sender delivers email. Kept as an interface so tests capture messages instead
// of dialling anything.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SMTPSender is the production implementation.
type SMTPSender struct{ cfg Config }

func New(cfg Config) *SMTPSender { return &SMTPSender{cfg: cfg} }

func (s *SMTPSender) Send(ctx context.Context, to, subject, body string) error {
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprint(s.cfg.Port))

	msg := strings.Builder{}
	fmt.Fprintf(&msg, "From: %s <%s>\r\n", s.cfg.FromName, s.cfg.FromEmail)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	fmt.Fprintf(&msg, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(body)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, s.cfg.FromEmail, []string{to}, []byte(msg.String()))
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("mail: send: %w", err)
		}
		return nil
	}
}

// Captured is a test double that records instead of sending.
type Captured struct {
	Messages []struct{ To, Subject, Body string }
}

func (c *Captured) Send(_ context.Context, to, subject, body string) error {
	c.Messages = append(c.Messages, struct{ To, Subject, Body string }{to, subject, body})
	return nil
}
