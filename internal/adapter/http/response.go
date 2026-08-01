// Package http is the gin adapter: routing, request/response mapping and
// middleware. All business logic lives in internal/app and internal/domain.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
	"github.com/stevenwilliam/ruuma/internal/platform/logging"
)

// errorBody is the single error envelope from docs/04 §2.
type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    apierror.Code  `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// fail renders any error through the one error model. The internal cause is
// logged, never serialised (docs/12, A05).
func fail(c *gin.Context, err error) {
	ae := apierror.From(err)

	if ae.Status >= 500 {
		logging.From(c.Request.Context()).Error("request failed",
			"code", string(ae.Code), "route", c.FullPath(), "error", ae.Error())
	} else {
		logging.From(c.Request.Context()).Info("request refused",
			"code", string(ae.Code), "route", c.FullPath(), "status", ae.Status)
	}

	c.AbortWithStatusJSON(ae.Status, errorBody{Error: errorPayload{
		Code:    ae.Code,
		Message: ae.Message,
		Details: ae.Details,
	}})
}

// ok renders a success payload.
func ok(c *gin.Context, body any) { c.JSON(http.StatusOK, body) }

// created renders a 201.
func created(c *gin.Context, body any) { c.JSON(http.StatusCreated, body) }

// noContent renders a 204.
func noContent(c *gin.Context) { c.Status(http.StatusNoContent) }

// listBody is the cursor-paginated list envelope (docs/04 §1).
type listBody struct {
	Items      any    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

func list(c *gin.Context, items any, nextCursor string) {
	c.JSON(http.StatusOK, listBody{Items: items, NextCursor: nextCursor})
}

// money is rendered as an integer plus its currency — never a decimal, never a
// formatted string (BR-1.1.4).
type amount struct {
	Value    int64  `json:"value"`
	Currency string `json:"currency"`
}

func rupiah(v int64) amount { return amount{Value: v, Currency: "IDR"} }
