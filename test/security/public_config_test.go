//go:build security

package security_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/test/testenv"
)

// GET /public-config is unauthenticated and reads from sys_parameters, the
// same table that holds notification templates, rate-limit tuning, OAuth
// switches and anything flagged is_secret. The endpoint is built on a
// compiled allowlist precisely so a row cannot widen it; these tests are what
// makes that claim checkable rather than a comment.

func getPublicConfig(t *testing.T, env *testenv.Env) (int, string) {
	t.Helper()
	res, err := env.Server.Client().Get(env.Server.URL + "/api/v1/public-config")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res.StatusCode, string(body)
}

// TestPublicConfigLeaksNothingElse plants values that must never be public and
// asserts none of them reach an anonymous caller.
func TestPublicConfigLeaksNothingElse(t *testing.T) {
	env := testenv.New(t)

	// Three shapes of thing that live in this table and must not escape: an
	// explicitly secret row, an operational tuning value, and customer-facing
	// message copy that still reveals how the business runs.
	planted := map[string]string{
		"integration.webhook_signing_key": "sk_live_MUSTNEVERAPPEAR_9f3a",
		"auth.otp_max_attempts":           "7331",
		"notify.template.secret_probe.id": "TEMPLATEMUSTNEVERAPPEAR",
	}
	for key, value := range planted {
		secret := key == "integration.webhook_signing_key"
		err := env.DB.Exec(`
			INSERT INTO sys_parameters (id, key, value, data_type, is_secret, created_at, updated_at)
			VALUES ($1, $2, $3, 'string', $4, now(), now())
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, is_secret = EXCLUDED.is_secret`,
			uuid.New(), key, value, secret).Error
		if err != nil {
			t.Fatalf("plant %s: %v", key, err)
		}
	}

	status, body := getPublicConfig(t, env)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", status, body)
	}

	for key, value := range planted {
		if strings.Contains(body, value) {
			t.Errorf("public-config leaked the value of %s", key)
		}
		if strings.Contains(body, key) {
			t.Errorf("public-config leaked the key name %s", key)
		}
	}
}

// TestPublicConfigCarriesOnlyAllowlistedFields walks the JSON and fails on any
// field that is not part of the documented shape. A future contributor who
// adds a value to the DTO has to come here and say so on purpose.
func TestPublicConfigCarriesOnlyAllowlistedFields(t *testing.T) {
	env := testenv.New(t)

	status, body := getPublicConfig(t, env)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode: %v — body = %s", err, body)
	}

	// ok() serialises the DTO directly — there is no data envelope on this
	// endpoint, so the allowlist applies to the top level as-is.
	doc := payload

	allowedTop := map[string]bool{"company_name": true, "whatsapp": true, "backdrop": true}
	for field := range doc {
		if !allowedTop[field] {
			t.Errorf("unexpected top-level field %q in public-config", field)
		}
	}

	wa, ok := doc["whatsapp"].(map[string]any)
	if !ok {
		t.Fatalf("whatsapp block missing or not an object: %s", body)
	}
	allowedWA := map[string]bool{"enabled": true, "number": true, "message_id": true, "message_en": true}
	for field := range wa {
		if !allowedWA[field] {
			t.Errorf("unexpected whatsapp field %q in public-config", field)
		}
	}
}

// TestPublicConfigDisablesWhatsAppWithoutANumber pins the safety interlock:
// the flag alone must not switch on a button that opens a chat with nobody.
func TestPublicConfigDisablesWhatsAppWithoutANumber(t *testing.T) {
	env := testenv.New(t)

	err := env.DB.Exec(`
		UPDATE sys_parameters SET value = '' WHERE key = 'company.whatsapp_number'`).Error
	if err != nil {
		t.Fatalf("blank the number: %v", err)
	}
	err = env.DB.Exec(`
		UPDATE sys_parameters SET value = 'true' WHERE key = 'company.whatsapp_enabled'`).Error
	if err != nil {
		t.Fatalf("enable the button: %v", err)
	}
	env.Params.Invalidate()

	status, body := getPublicConfig(t, env)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}

	var payload struct {
		WhatsApp struct {
			Enabled bool   `json:"enabled"`
			Number  string `json:"number"`
		} `json:"whatsapp"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode: %v — body = %s", err, body)
	}
	if payload.WhatsApp.Enabled {
		t.Error("whatsapp reported enabled with no number configured")
	}
	if payload.WhatsApp.Number != "" {
		t.Errorf("number = %q, want empty", payload.WhatsApp.Number)
	}
}

// TestPublicConfigNormalisesTheNumber proves an operator can paste a number in
// any of the shapes people actually write them and still get a working link.
func TestPublicConfigNormalisesTheNumber(t *testing.T) {
	cases := []struct {
		name  string
		typed string
		want  string
	}{
		{"already E.164 without plus", "6281234567890", "6281234567890"},
		{"with plus and spaces", "+62 812-3456-7890", "6281234567890"},
		{"local trunk prefix", "0812 3456 7890", "6281234567890"},
		{"parenthesised", "(0812) 3456-7890", "6281234567890"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := testenv.New(t)

			err := env.DB.Exec(`
				UPDATE sys_parameters SET value = $1 WHERE key = 'company.whatsapp_number'`,
				tc.typed).Error
			if err != nil {
				t.Fatalf("set the number: %v", err)
			}
			env.Params.Invalidate()

			_, body := getPublicConfig(t, env)

			var payload struct {
				WhatsApp struct {
					Number string `json:"number"`
				} `json:"whatsapp"`
			}
			if err := json.Unmarshal([]byte(body), &payload); err != nil {
				t.Fatalf("decode: %v — body = %s", err, body)
			}
			if payload.WhatsApp.Number != tc.want {
				t.Errorf("%q normalised to %q, want %q", tc.typed, payload.WhatsApp.Number, tc.want)
			}
		})
	}
}

// TestPublicConfigBackdropRejectsInjection pins the validation on the backdrop
// filename (BR-1.4.6). The value ends up inside a CSS url() in every customer's browser,
// so anyone holding the parameter permission would otherwise be one UPDATE away
// from injecting styles — or pointing the page at a third-party host, which is
// also a visitor-tracking channel. A bad value must fall back, not propagate.
func TestPublicConfigBackdropRejectsInjection(t *testing.T) {
	hostile := []struct {
		name  string
		value string
	}{
		{"css breakout", `x.jpg"); background: url(https://evil.example/p.png`},
		{"path traversal", "../../etc/passwd"},
		{"absolute url", "https://evil.example/tracker.png"},
		{"no extension", "backdrop"},
		{"script extension", "payload.svg"},
		{"semicolon", "a.jpg;color:red"},
		{"whitespace and quote", `a.jpg' `},
		{"empty", ""},
	}

	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			env := testenv.New(t)

			err := env.DB.Exec(`
				UPDATE sys_parameters SET value = $1 WHERE key = 'company.backdrop_file'`,
				tc.value).Error
			if err != nil {
				t.Fatalf("set the backdrop: %v", err)
			}
			env.Params.Invalidate()

			_, body := getPublicConfig(t, env)

			var payload struct {
				Backdrop struct {
					File string `json:"file"`
				} `json:"backdrop"`
			}
			if err := json.Unmarshal([]byte(body), &payload); err != nil {
				t.Fatalf("decode: %v — body = %s", err, body)
			}
			if payload.Backdrop.File != "backdrop.jpg" {
				t.Errorf("hostile value %q served as %q, want the default",
					tc.value, payload.Backdrop.File)
			}
		})
	}
}

// TestPublicConfigBackdropAcceptsAShippedFile is the other half: validation
// that rejects everything is not validation, it is a broken setting.
func TestPublicConfigBackdropAcceptsAShippedFile(t *testing.T) {
	env := testenv.New(t)

	const want = "ruuma-share-1200x630.png"
	if err := env.DB.Exec(`
		UPDATE sys_parameters SET value = $1 WHERE key = 'company.backdrop_file'`,
		want).Error; err != nil {
		t.Fatalf("set the backdrop: %v", err)
	}
	env.Params.Invalidate()

	_, body := getPublicConfig(t, env)

	var payload struct {
		Backdrop struct {
			Enabled bool   `json:"enabled"`
			File    string `json:"file"`
		} `json:"backdrop"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Backdrop.File != want {
		t.Errorf("file = %q, want %q", payload.Backdrop.File, want)
	}
	if !payload.Backdrop.Enabled {
		t.Error("backdrop reported disabled when the parameter is true")
	}
}
