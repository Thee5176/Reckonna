package command

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thee5176/reckonna/internal/handler/problem"
	"github.com/thee5176/reckonna/internal/repository/command"
	"github.com/thee5176/reckonna/internal/service"
)

// Handler serves the write-side journal-entry endpoints.
type Handler struct {
	svc  *service.LedgerCommandService
	pw   *problem.Writer
	idem *command.Queries // for the Idempotency-Key middleware
}

// NewHandler builds the command handler over the service, problem writer, and
// pool (the pool backs the idempotency-record reads/writes).
func NewHandler(svc *service.LedgerCommandService, pw *problem.Writer, pool *pgxpool.Pool) *Handler {
	return &Handler{svc: svc, pw: pw, idem: command.New(pool)}
}

// Register wires the write endpoints onto the router group.
func (h *Handler) Register(r gin.IRouter) {
	r.POST("/command/journal-entries", h.Post)
	r.PUT("/command/journal-entries/:id", h.Put)
	r.DELETE("/command/journal-entries/:id", h.Delete)
}

// Post creates a journal entry (AT1/AT2/AT10/AT13/AT14). 201 with id+version;
// ETag carries the version for later If-Match updates.
func (h *Handler) Post(c *gin.Context) {
	var dto entryDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
		return
	}
	in, err := dto.toInput(problem.Sub(c))
	if err != nil {
		writeError(h.pw, c, err)
		return
	}
	id, version, err := h.svc.PostLedger(c.Request.Context(), in)
	if err != nil {
		writeError(h.pw, c, err)
		return
	}
	c.Header("ETag", etag(version))
	c.Header("Location", "/command/journal-entries/"+id.String())
	c.JSON(http.StatusCreated, gin.H{"id": id, "version": version})
}

// Put replaces an entry under optimistic concurrency. If-Match is mandatory
// (missing → 428, AT16b); a stale version → 409 (AT16).
func (h *Handler) Put(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
		return
	}
	ifMatch := c.GetHeader("If-Match")
	if ifMatch == "" {
		h.pw.Write(c, http.StatusPreconditionRequired, "validation_failed", nil, nil)
		return
	}
	version, err := parseETag(ifMatch)
	if err != nil {
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
		return
	}

	var dto entryDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
		return
	}
	in, err := dto.toInput(problem.Sub(c))
	if err != nil {
		writeError(h.pw, c, err)
		return
	}
	newVersion, err := h.svc.UpdateLedger(c.Request.Context(), id, version, in)
	if err != nil {
		writeError(h.pw, c, err)
		return
	}
	c.Header("ETag", etag(newVersion))
	c.JSON(http.StatusOK, gin.H{"id": id, "version": newVersion})
}

// Delete removes an entry and its lines (AT5). 204 on success.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
		return
	}
	if err := h.svc.DeleteLedger(c.Request.Context(), id, problem.Sub(c)); err != nil {
		writeError(h.pw, c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// etag formats a version as a strong ETag value: "3".
func etag(version int32) string { return fmt.Sprintf("%q", strconv.Itoa(int(version))) }

// parseETag parses an If-Match / ETag value (quoted or bare) into a version.
func parseETag(v string) (int32, error) {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"`)
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}
