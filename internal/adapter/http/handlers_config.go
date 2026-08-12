package http

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// GET /api/v1/public-config — the handful of sys_parameters values the
// unauthenticated SPA needs to render its chrome (BR-1.4.5).
//
// The allowlist below is compiled in, and that is the whole security design.
// sys_parameters also holds notification templates, rate-limit tuning, OAuth
// switches and anything an operator adds later, including rows flagged
// is_secret. Driving a public endpoint from a database flag would mean one
// mistaken UPDATE — or one new row seeded with the wrong default — silently
// publishes a secret to the internet. A Go allowlist cannot be widened by a
// row: adding a key here is a code change, a review and a deploy.
//
// Anything added to this list is public forever, to everyone, uncached-by-auth.
// Only add values that would be printed on a shopfront window.
//
// Enforced by TestPublicConfigLeaksNothingElse in test/security.
var publicConfigKeys = []string{
	"company.name",
	"company.whatsapp_enabled",
	"company.whatsapp_number",
	"company.whatsapp_message_id",
	"company.whatsapp_message_en",
}

// PublicConfigKeys exposes the allowlist so the security suite can assert that
// the response carries nothing beyond it.
func PublicConfigKeys() []string { return append([]string(nil), publicConfigKeys...) }

type whatsappConfigDTO struct {
	Enabled bool `json:"enabled"`
	// Number is E.164 digits with no + or separators, ready to drop into
	// https://wa.me/<number>. Empty means "not configured": the client hides
	// the button rather than rendering a link to nowhere.
	Number    string `json:"number"`
	MessageID string `json:"message_id"`
	MessageEN string `json:"message_en"`
}

type publicConfigDTO struct {
	CompanyName string            `json:"company_name"`
	WhatsApp    whatsappConfigDTO `json:"whatsapp"`
}

func (s *Server) publicConfig(c *gin.Context) {
	ctx := c.Request.Context()

	// Group scope only. This endpoint is reached from pages with no store
	// context — sign-in, credits — so resolving a per-store override here
	// would answer differently depending on which page asked.
	number := sanitiseMSISDN(s.Params.String(ctx, nil, "company.whatsapp_number"))

	ok(c, publicConfigDTO{
		CompanyName: s.Params.String(ctx, nil, "company.name"),
		WhatsApp: whatsappConfigDTO{
			// A blank number disables the button whatever the flag says: the
			// two settings can disagree, and "on but unreachable" is the worse
			// of the two failures.
			Enabled:   s.Params.Bool(ctx, nil, "company.whatsapp_enabled") && number != "",
			Number:    number,
			MessageID: s.Params.String(ctx, nil, "company.whatsapp_message_id"),
			MessageEN: s.Params.String(ctx, nil, "company.whatsapp_message_en"),
		},
	})
}

// sanitiseMSISDN reduces whatever an operator typed to the bare digits wa.me
// expects. Admins paste numbers as "+62 811-0000-0000" or "0811 0000 0000",
// and both have to work — a link built from the raw string 404s at WhatsApp
// and the operator has no way to tell why.
//
// A single leading 0 is the Indonesian trunk prefix and is replaced by the
// country code; the number is left alone if it already starts with one.
func sanitiseMSISDN(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()

	if strings.HasPrefix(digits, "0") {
		return "62" + strings.TrimLeft(digits, "0")
	}
	return digits
}
