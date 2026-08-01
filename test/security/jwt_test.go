//go:build security

package security_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/platform/security"
	"github.com/stevenwilliam/ruuma/test/testenv"
)

// TestTokenTampering_docs12_A02 covers the classic token attacks end to end.
func TestTokenTampering_docs12_A02(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	valid := env.StaffToken(f.KitchenA, security.RoleKitchen)

	// The honest token works, so a blanket 401 is not what makes the rest pass.
	if res := env.Do(http.MethodGet, "/api/v1/me", valid, nil); res.Status != http.StatusOK {
		t.Fatalf("valid token: %d %s", res.Status, res.Raw)
	}

	parts := strings.Split(valid, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token shape")
	}

	t.Run("tampered signature", func(t *testing.T) {
		bad := parts[0] + "." + parts[1] + "." + parts[2][:len(parts[2])-2] + "AB"
		if res := env.Do(http.MethodGet, "/api/v1/me", bad, nil); res.Status != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", res.Status)
		}
	})

	t.Run("privilege escalation in the payload", func(t *testing.T) {
		// Re-encode the claims as owner and keep the original signature: this
		// is the attack the signature exists to stop.
		raw, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatalf("decode claims: %v", err)
		}
		var claims map[string]any
		if err := json.Unmarshal(raw, &claims); err != nil {
			t.Fatalf("unmarshal claims: %v", err)
		}
		claims["role"] = "owner"
		edited, _ := json.Marshal(claims)
		forged := parts[0] + "." + base64.RawURLEncoding.EncodeToString(edited) + "." + parts[2]

		res := env.Do(http.MethodGet, "/api/v1/admin/sys-parameters", forged, nil)
		if res.Status != http.StatusUnauthorized {
			t.Fatalf("forged role: %d %s, want 401", res.Status, res.Raw)
		}
	})

	t.Run("alg=none", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"iss": "ruuma", "sub": f.Owner.String(), "role": "owner", "typ": "staff",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if res := env.Do(http.MethodGet, "/api/v1/me", unsigned, nil); res.Status != http.StatusUnauthorized {
			t.Fatalf("alg=none: %d, want 401", res.Status)
		}
	})

	t.Run("foreign signing key", func(t *testing.T) {
		other := security.NewTokenSigner("a-completely-different-signing-key-32b!", "", "ruuma",
			time.Hour, time.Now)
		forged, _, err := other.Issue(security.SubjectStaff, f.Owner, security.RoleOwner)
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		if res := env.Do(http.MethodGet, "/api/v1/me", forged, nil); res.Status != http.StatusUnauthorized {
			t.Fatalf("foreign key: %d, want 401", res.Status)
		}
	})

	t.Run("expired", func(t *testing.T) {
		past := security.NewTokenSigner("test-signing-key-at-least-32-bytes-long!!", "", "ruuma",
			time.Minute, func() time.Time { return time.Now().Add(-2 * time.Hour) })
		expired, _, err := past.Issue(security.SubjectStaff, f.KitchenA, security.RoleKitchen)
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		if res := env.Do(http.MethodGet, "/api/v1/me", expired, nil); res.Status != http.StatusUnauthorized {
			t.Fatalf("expired: %d, want 401", res.Status)
		}
	})

	t.Run("unknown subject", func(t *testing.T) {
		ghost := env.StaffToken(uuid.New(), security.RoleOwner)
		if res := env.Do(http.MethodGet, "/api/v1/me", ghost, nil); res.Status != http.StatusUnauthorized {
			t.Fatalf("unknown subject: %d, want 401", res.Status)
		}
	})
}

// TestScopeComesFromTheDatabase_BR_2_7_9 is the reason the token carries no
// store list: revoking an assignment must take effect on the very next request,
// even though the token is unchanged.
func TestScopeComesFromTheDatabase_BR_2_7_9(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	token := env.StaffToken(f.KitchenA, security.RoleKitchen)

	res := env.Do(http.MethodGet, "/api/v1/ops/orders?store_id="+f.StoreA.String(), token, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("before revocation: %d %s", res.Status, res.Raw)
	}

	if err := env.DB.Exec(`DELETE FROM staff_store_assignments WHERE user_id = ?`, f.KitchenA).Error; err != nil {
		t.Fatalf("revoke: %v", err)
	}

	res = env.Do(http.MethodGet, "/api/v1/ops/orders?store_id="+f.StoreA.String(), token, nil)
	if res.Status != http.StatusForbidden {
		t.Fatalf("after revocation the same token still worked: %d %s", res.Status, res.Raw)
	}
}

// TestDeactivatedStaffLoseAccessImmediately_docs12_A01 — the same idea for the
// active flag.
func TestDeactivatedStaffLoseAccessImmediately_docs12_A01(t *testing.T) {
	env := testenv.New(t)
	f := env.Fixtures
	token := env.StaffToken(f.ManagerA, security.RoleStoreManager)

	if res := env.Do(http.MethodGet, "/api/v1/me", token, nil); res.Status != http.StatusOK {
		t.Fatalf("before: %d", res.Status)
	}
	if err := env.DB.Exec(`UPDATE users SET is_active = false WHERE id = ?`, f.ManagerA).Error; err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if res := env.Do(http.MethodGet, "/api/v1/me", token, nil); res.Status != http.StatusForbidden {
		t.Fatalf("after deactivation: %d, want 403", res.Status)
	}
}
