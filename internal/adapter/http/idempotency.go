package http

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// idempotent enforces the Idempotency-Key contract on every endpoint that
// creates money or reserves capacity (docs/04 §9).
//
// Same key + same body replays the stored response; same key + different body
// is 409. A failed request abandons its key so the caller may retry.
func (s *Server) idempotent(endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			fail(c, apierror.BadRequest(apierror.CodeValidation,
				"This request requires an Idempotency-Key header."))
			return
		}
		if len(key) > 200 {
			fail(c, apierror.BadRequest(apierror.CodeValidation, "That Idempotency-Key is too long."))
			return
		}

		p := principal(c)
		subjectType := string(security.SubjectCustomer)
		if p.SubjectType == security.SubjectStaff {
			subjectType = string(security.SubjectStaff)
		}
		body := requestBody(c)

		stored, err := s.Idempotency.Begin(c.Request.Context(), key, subjectType, p.ID, endpoint, body)
		if err != nil {
			fail(c, err)
			return
		}
		if stored != nil {
			// Replay the original outcome verbatim.
			c.Header("Idempotency-Replayed", "true")
			c.Data(stored.Code, "application/json; charset=utf-8", stored.Body)
			c.Abort()
			return
		}

		recorder := &responseRecorder{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = recorder

		c.Next()

		status := recorder.Status()
		if status >= 200 && status < 300 {
			_ = s.Idempotency.Complete(c.Request.Context(), key, subjectType, p.ID, endpoint,
				status, recorder.buf.Bytes())
			return
		}
		// A refusal is not an outcome worth replaying: free the key.
		_ = s.Idempotency.Abandon(c.Request.Context(), key, subjectType, p.ID, endpoint)
	}
}

type responseRecorder struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.buf.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteString(s string) (int, error) {
	r.buf.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}

func notFound() error { return apierror.NotFound("That endpoint does not exist.") }

var _ = http.StatusOK
