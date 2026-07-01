package command

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/thee5176/reckonna/internal/handler/problem"
	"github.com/thee5176/reckonna/internal/repository/command"
)

// Idempotency implements opt-in idempotent POSTs (plan 03 S8b). With no
// Idempotency-Key header the request proceeds normally. With a key:
//   - a replay of the same key + same owner + same body returns the cached
//     response, without re-running the handler (AT15 — no second row);
//   - the same key with a DIFFERENT body → 422 duplicate_idempotency_key (AT15b).
//
// Records are scoped by (key, owner_sub); the owner comes from the auth
// middleware, so this must be chained AFTER auth.
func (h *Handler) Idempotency() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		owner := problem.Sub(c)

		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			h.pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(raw)) // restore for the handler
		hash := sha256Hex(raw)

		rec, err := h.idem.GetIdempotencyRecord(ctx, command.GetIdempotencyRecordParams{Key: key, OwnerSub: owner})
		switch {
		case err == nil:
			if rec.RequestHash != hash {
				h.pw.Write(c, http.StatusUnprocessableEntity, "duplicate_idempotency_key", nil, nil)
				return
			}
			// Replay the cached response verbatim; do not re-run the handler.
			c.Data(int(rec.ResponseStatus), "application/json", rec.ResponseBody)
			c.Abort()
			return
		case errors.Is(err, pgx.ErrNoRows):
			// first time — capture and store below
		default:
			writeError(h.pw, c, err)
			return
		}

		cap := &bodyCapture{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = cap
		c.Next()

		// Only cache a successful creation; errors are not idempotency-cached.
		if cap.Status() == http.StatusCreated {
			_ = h.idem.InsertIdempotencyRecord(ctx, command.InsertIdempotencyRecordParams{
				Key:            key,
				OwnerSub:       owner,
				RequestHash:    hash,
				ResponseStatus: int32(cap.Status()),
				ResponseBody:   cap.buf.Bytes(),
			})
		}
	}
}

// bodyCapture tees the response body into a buffer so it can be cached.
type bodyCapture struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *bodyCapture) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyCapture) WriteString(s string) (int, error) {
	w.buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
