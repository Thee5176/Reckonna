// Package query holds the read-side Gin handlers. It imports only the read model
// (internal/query) + the neutral problem writer — never the command side — so
// cmd/query stays write-free at compile time (IT9).
package query

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/thee5176/reckonna/internal/handler/problem"
	qsvc "github.com/thee5176/reckonna/internal/query"
)

// Handler serves the read endpoints.
type Handler struct {
	svc *qsvc.Service
	pw  *problem.Writer
}

// NewHandler builds the query handler.
func NewHandler(svc *qsvc.Service, pw *problem.Writer) *Handler {
	return &Handler{svc: svc, pw: pw}
}

// Register wires the read endpoints onto the router.
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/query/journal-entries", h.List)
	r.GET("/query/journal-entries/:id", h.Get)
	r.GET("/query/journal-lines/:id", h.GetLines)
	r.GET("/query/accounts", h.ListAccounts)
	r.GET("/query/balances", h.Balances)
	r.GET("/query/statements/balance-sheet", h.BalanceSheet)
	r.GET("/query/statements/profit-loss", h.ProfitLoss)
}

// List returns an owner-scoped, cursor-paginated page of entries (AT3, IT5).
func (h *Handler) List(c *gin.Context) {
	limit := 0
	if s := c.Query("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
			return
		}
		limit = n
	}
	page, err := h.svc.ListLedgers(c.Request.Context(), problem.Sub(c), limit, c.Query("cursor"))
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// Get returns one owner-scoped entry with lines; not owned/existing → 404 (T16).
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
		return
	}
	view, err := h.svc.GetLedger(c.Request.Context(), id, problem.Sub(c))
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", strconv.Itoa(int(view.Version))))
	c.JSON(http.StatusOK, view)
}

// GetLines returns just the lines of an owner-scoped entry.
func (h *Handler) GetLines(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
		return
	}
	view, err := h.svc.GetLedger(c.Request.Context(), id, problem.Sub(c))
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": view.ID, "lines": view.Lines})
}

// ListAccounts returns the active chart of accounts (AT9).
func (h *Handler) ListAccounts(c *gin.Context) {
	accounts, err := h.svc.ListAccounts(c.Request.Context())
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

// Balances returns owner-scoped outstanding balances for repeated ?account= codes.
func (h *Handler) Balances(c *gin.Context) {
	raw := c.QueryArray("account")
	if len(raw) == 0 {
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
		return
	}
	codes := make([]int, len(raw))
	for i, s := range raw {
		n, err := strconv.Atoi(s)
		// A valid account is a 5-digit CoA code; reject anything outside the
		// range (also keeps the later int32 narrowing safe).
		if err != nil || n < 10000 || n > 99999 {
			h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
			return
		}
		codes[i] = n
	}
	balances, err := h.svc.Balances(c.Request.Context(), problem.Sub(c), codes)
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"balances": balances})
}

// BalanceSheet returns the owner-scoped balance sheet (AT6).
func (h *Handler) BalanceSheet(c *gin.Context) {
	view, err := h.svc.BalanceSheet(c.Request.Context(), problem.Sub(c))
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// ProfitLoss returns the owner-scoped profit-and-loss statement (AT7).
func (h *Handler) ProfitLoss(c *gin.Context) {
	view, err := h.svc.ProfitLoss(c.Request.Context(), problem.Sub(c))
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// writeErr maps read-model errors to RFC 7807 responses.
func (h *Handler) writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, qsvc.ErrNotFound):
		h.pw.Write(c, http.StatusNotFound, "not_found", nil, nil)
	case errors.Is(err, qsvc.ErrInvalidCursor):
		h.pw.Write(c, http.StatusBadRequest, "invalid_cursor", nil, nil)
	case errors.Is(err, qsvc.ErrTooManyItems):
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
	default:
		h.pw.Write(c, http.StatusInternalServerError, "validation_failed", nil, nil)
	}
}
