package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/app/ports"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/logging"
	"github.com/stevenwilliam/ruuma/internal/platform/metrics"
	"github.com/stevenwilliam/ruuma/internal/platform/ratelimit"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

const (
	ctxPrincipal = "ruuma.principal"
	ctxBody      = "ruuma.body"
)

// requestID assigns and propagates a request id (docs/12, A09).
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-Id")
		if rid == "" {
			rid = uuid.New().String()
		}
		ctx := logging.WithRequestID(c.Request.Context(), rid)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Request-Id", rid)
		c.Next()
	}
}

// observability logs each request and records its duration by route template —
// never the raw path, which can contain an order code (docs/12, A09).
func observability() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		metrics.HTTPDuration.WithLabelValues(c.Request.Method, route,
			http.StatusText(c.Writer.Status())).Observe(time.Since(start).Seconds())

		logging.From(c.Request.Context()).Info("request",
			"method", c.Request.Method, "route", route,
			"status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	}
}

// securityHeaders applies the header set from docs/12, A05.
func securityHeaders(isProduction bool) gin.HandlerFunc {
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self'",
		"img-src 'self' data: blob:",
		"font-src 'self'",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")

	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", csp)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		if isProduction {
			c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		c.Next()
	}
}

// corsMiddleware allows only the configured origins, with credentials handled
// deliberately (docs/12, A01). There is no wildcard.
func corsMiddleware(allowed []string) gin.HandlerFunc {
	allow := map[string]bool{}
	for _, o := range allowed {
		allow[strings.TrimRight(o, "/")] = true
	}
	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if origin != "" && allow[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers",
				"Authorization, Content-Type, Idempotency-Key, X-Request-Id, Accept-Language")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// bodyLimit caps request bodies before anything parses them (docs/12).
func bodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// authenticate resolves the bearer token into a Principal and — critically —
// re-resolves store scope from the database on every request, never from a
// token claim (BR-2.7.9).
func authenticate(signer *security.TokenSigner, stores ports.Stores, staff ports.Staff) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Next() // anonymous; requirePermission decides whether that is enough
			return
		}
		raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
		if raw == "" || raw == header {
			fail(c, apierror.Unauthorized("Sign in to continue."))
			return
		}

		claims, err := signer.Parse(raw)
		if err != nil {
			metrics.AuthFailures.WithLabelValues("token").Inc()
			fail(c, apierror.Unauthorized("Your session has expired. Please sign in again."))
			return
		}

		subjectID, err := uuid.Parse(claims.Subject)
		if err != nil {
			fail(c, apierror.Unauthorized("Your session is not valid."))
			return
		}

		p := security.Principal{
			SubjectType: claims.SubjectType,
			ID:          subjectID,
			Role:        claims.Role,
		}

		if claims.SubjectType == security.SubjectStaff {
			u, err := staff.Get(c.Request.Context(), subjectID)
			if err != nil {
				fail(c, apierror.Unauthorized("Your session is not valid."))
				return
			}
			if !u.IsActive {
				fail(c, apierror.Forbidden(apierror.CodeForbidden, "This account is not active."))
				return
			}
			// Role and scope come from the database, so a stale token cannot
			// carry an old role or an old store list (BR-2.7.9).
			p.Role = security.Role(u.Role)
			p.GroupScope = u.IsGroupScope
			p.Stores = u.Stores
		}

		c.Set(ctxPrincipal, p)
		c.Next()
	}
}

// principal returns the authenticated caller, or the zero Principal — which
// holds no permission at all (BR-2.7.6).
func principal(c *gin.Context) security.Principal {
	if v, ok := c.Get(ctxPrincipal); ok {
		if p, ok := v.(security.Principal); ok {
			return p
		}
	}
	return security.Principal{}
}

// requirePermission is the deny-by-default gate. Every protected route declares
// exactly one permission; a route with none is unreachable (docs/12, A01).
func requirePermission(perm security.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		if p.Role == "" {
			fail(c, apierror.Unauthorized("Sign in to continue."))
			return
		}
		if !p.Can(perm) {
			fail(c, apierror.Forbidden(apierror.CodeForbidden,
				"You do not have access to that."))
			return
		}
		c.Next()
	}
}

// requireAuthenticated allows any signed-in subject.
func requireAuthenticated() gin.HandlerFunc {
	return func(c *gin.Context) {
		if principal(c).Role == "" {
			fail(c, apierror.Unauthorized("Sign in to continue."))
			return
		}
		c.Next()
	}
}

// rateLimit throttles per identifier and per IP (docs/04 §9, docs/12 A07).
func rateLimit(limiter *ratelimit.Limiter, name string, rule ratelimit.Rule, key func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ident := key(c)
		for _, k := range []string{name + ":id:" + ident, name + ":ip:" + c.ClientIP()} {
			if res := limiter.Allow(k, rule); !res.Allowed {
				metrics.RateLimitHits.WithLabelValues(name).Inc()
				c.Header("Retry-After", strconv.Itoa(int(res.RetryAfter.Seconds())))
				fail(c, apierror.TooManyRequests("Too many attempts. Please wait a moment."))
				return
			}
		}
		c.Next()
	}
}

// byIP keys a limit on the caller's address alone.
func byIP(c *gin.Context) string { return c.ClientIP() }

// bySubject keys a limit on the authenticated subject, falling back to the IP.
func bySubject(c *gin.Context) string {
	if p := principal(c); p.ID != uuid.Nil {
		return p.ID.String()
	}
	return c.ClientIP()
}

// captureBody buffers the request body so idempotency can hash it and the
// handler can still bind it.
func captureBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				fail(c, apierror.Validation("The request body could not be read.", nil))
				return
			}
			c.Set(ctxBody, body)
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		c.Next()
	}
}

func requestBody(c *gin.Context) []byte {
	if v, ok := c.Get(ctxBody); ok {
		if b, ok := v.([]byte); ok {
			return b
		}
	}
	return nil
}

// byEmailBody keys a login limit on the email attempted, so one attacker cannot
// lock out every account from one address, and one address cannot brute-force
// one account (docs/12, A07).
func byEmailBody(c *gin.Context) string {
	var body struct {
		Email string `json:"email"`
	}
	if raw := requestBody(c); len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	if body.Email == "" {
		return c.ClientIP()
	}
	return strings.ToLower(strings.TrimSpace(body.Email))
}

// byPhoneBody keys OTP limits on the phone number requested.
func byPhoneBody(c *gin.Context) string {
	var body struct {
		Phone string `json:"phone"`
	}
	if raw := requestBody(c); len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	if body.Phone == "" {
		return c.ClientIP()
	}
	return strings.TrimSpace(body.Phone)
}
