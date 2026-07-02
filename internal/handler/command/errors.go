package command

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/thee5176/reckonna/internal/domain"
	"github.com/thee5176/reckonna/internal/handler/problem"
	"github.com/thee5176/reckonna/internal/metrics"
	"github.com/thee5176/reckonna/internal/repository/command"
	"github.com/thee5176/reckonna/internal/service"
)

// writeError classifies a command-side error into an RFC 7807 response. The
// 400/422 split follows the API Conventions: syntactic → 400 validation_failed;
// business-rule → 422 with a specific code. DB-layer defenses (balance trigger
// 23514, FK violation 23503) map to the same codes as their domain equivalents.
func writeError(pw *problem.Writer, c *gin.Context, err error) {
	switch {
	case errors.Is(err, errBadRequest):
		pw.Write(c, http.StatusBadRequest, "validation_failed", nil, nil)

	case errors.Is(err, domain.ErrUnbalanced) || isPgCode(err, "23514"):
		metrics.RecordLedgerRejected(c.Request.Context(), "unbalanced_entry")
		zero := 0
		pw.Write(c, http.StatusUnprocessableEntity, "unbalanced_entry",
			[]problem.FieldError{{LineIndex: &zero, Field: "amount", Issue: "debit_credit_mismatch"}}, nil)

	case errors.Is(err, domain.ErrMixedCurrency):
		pw.Write(c, http.StatusUnprocessableEntity, "mixed_currency", nil, nil)

	case errors.Is(err, domain.ErrRequiredDimension):
		pw.Write(c, http.StatusUnprocessableEntity, "missing_required_dimension", nil, nil)

	case errors.Is(err, service.ErrUnknownAccountCode) || isPgCode(err, "23503"):
		pw.Write(c, http.StatusUnprocessableEntity, "unknown_account_code", nil, nil)

	case errors.Is(err, command.ErrUnknownDimension) ||
		errors.Is(err, service.ErrBookNotFound) ||
		errors.Is(err, domain.ErrNoLines):
		pw.Write(c, http.StatusUnprocessableEntity, "validation_failed", nil, nil)

	case errors.Is(err, service.ErrEntryNotFound):
		pw.Write(c, http.StatusNotFound, "not_found", nil, nil)

	case errors.Is(err, service.ErrForbidden):
		pw.Write(c, http.StatusForbidden, "forbidden", nil, nil)

	default:
		var vc *service.VersionConflictError
		if errors.As(err, &vc) {
			pw.Write(c, http.StatusConflict, "concurrency_conflict", nil, &vc.Current)
			return
		}
		pw.Write(c, http.StatusInternalServerError, "validation_failed", nil, nil)
	}
}

// isPgCode reports whether err wraps a PostgreSQL error with the given SQLSTATE.
func isPgCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
