//go:build security

package security_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stevenwilliam/ruuma/test/testenv"
)

// TestSecurityHeaders_docs12_A05 asserts the header set on a real response.
func TestSecurityHeaders_docs12_A05(t *testing.T) {
	env := testenv.New(t)

	res, err := env.Server.Client().Get(env.Server.URL + "/api/v1/stores")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for header, expected := range want {
		if got := res.Header.Get(header); got != expected {
			t.Errorf("%s = %q, want %q", header, got, expected)
		}
	}

	csp := res.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("no Content-Security-Policy")
	}
	// docs/12 A05: no unsafe-inline, and framing denied.
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP allows unsafe directives: %s", csp)
	}
	for _, directive := range []string{"default-src 'self'", "frame-ancestors 'none'", "object-src 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing %q: %s", directive, csp)
		}
	}
	if res.Header.Get("Permissions-Policy") == "" {
		t.Error("no Permissions-Policy")
	}
	// A request id is echoed for correlation (docs/12, A09).
	if res.Header.Get("X-Request-Id") == "" {
		t.Error("no X-Request-Id")
	}
}

// TestCORSAllowList_docs12_A01 — known origins only, never a wildcard.
func TestCORSAllowList_docs12_A01(t *testing.T) {
	env := testenv.New(t)

	check := func(origin string) http.Header {
		req, err := http.NewRequest(http.MethodGet, env.Server.URL+"/api/v1/stores", nil)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		req.Header.Set("Origin", origin)
		res, err := env.Server.Client().Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		defer func() { _ = res.Body.Close() }()
		return res.Header
	}

	allowed := check("http://test.local")
	if allowed.Get("Access-Control-Allow-Origin") != "http://test.local" {
		t.Errorf("known origin not allowed: %q", allowed.Get("Access-Control-Allow-Origin"))
	}

	blocked := check("https://evil.example")
	if got := blocked.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unknown origin allowed: %q", got)
	}
}

// TestErrorsDoNotLeakInternals_docs12_A05 — a client sees a stable code and a
// human message, never driver text or a stack trace.
func TestErrorsDoNotLeakInternals_docs12_A05(t *testing.T) {
	env := testenv.New(t)

	for _, path := range []string{
		"/api/v1/menu?store_id=not-a-uuid",
		"/api/v1/availability/slots?store_id=" + env.Fixtures.StoreA.String() + "&date=nonsense",
		"/api/v1/orders/00000000-0000-0000-0000-000000000000",
	} {
		res := env.Do(http.MethodGet, path, env.CustomerToken(env.Fixtures.Customer), nil)
		lower := strings.ToLower(res.Raw)
		for _, leak := range []string{"sqlstate", "pq:", "gorm", "goroutine", "panic:", "/home/dev"} {
			if strings.Contains(lower, leak) {
				t.Errorf("%s leaked %q: %s", path, leak, res.Raw)
			}
		}
		if res.Code() == "" {
			t.Errorf("%s returned no stable error code: %s", path, res.Raw)
		}
	}
}
