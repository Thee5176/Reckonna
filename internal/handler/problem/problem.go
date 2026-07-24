// Package problem writes RFC 7807 Problem Details responses
// (application/problem+json) with locale-neutral `code` and localized
// title/detail. It is deliberately neutral — it imports neither the command nor
// the query side — so both routers can share it without breaking the
// compile-time CQRS purity guarantee (IT9).
package problem

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/thee5176/reckonna/internal/config"
)

// Context keys shared across middleware + handlers.
const (
	// LocaleKey holds the resolved request locale (set by the i18n middleware).
	LocaleKey = "locale"
	// SubKey holds the authenticated owner id (JWT sub, set by the auth middleware).
	SubKey = "sub"
)

// FieldError is a per-line / per-field problem detail. All fields are
// locale-neutral so clients can render them.
type FieldError struct {
	LineIndex *int   `json:"line_index,omitempty"`
	Field     string `json:"field,omitempty"`
	Issue     string `json:"issue,omitempty"`
}

// Problem is the RFC 7807 envelope. `code` is the stable, locale-neutral key;
// tests assert on it, never on title/detail.
type Problem struct {
	Type           string       `json:"type"`
	Title          string       `json:"title"`
	Status         int          `json:"status"`
	Code           string       `json:"code"`
	Detail         string       `json:"detail"`
	Instance       string       `json:"instance"`
	Errors         []FieldError `json:"errors,omitempty"`
	CurrentVersion *int32       `json:"current_version,omitempty"`
}

// Writer renders Problems with translations from the loaded bundle.
type Writer struct {
	bundle *config.Bundle
}

// NewWriter builds a problem Writer over the i18n bundle.
func NewWriter(b *config.Bundle) *Writer { return &Writer{bundle: b} }

// Locale returns the request locale from context, defaulting to en.
func Locale(c *gin.Context) string {
	if v, ok := c.Get(LocaleKey); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return config.DefaultLocale
}

// Sub returns the authenticated owner id from context ("" if unauthenticated).
func Sub(c *gin.Context) string {
	if v, ok := c.Get(SubKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Write emits a localized Problem with the given status + code, aborting the
// gin chain. fields/current are optional.
func (w *Writer) Write(c *gin.Context, status int, code string, fields []FieldError, current *int32) {
	le := w.bundle.Error(Locale(c), code)
	p := Problem{
		Type:           "https://reckonna.dev/errors/" + dashed(code),
		Title:          le.Title,
		Status:         status,
		Code:           code,
		Detail:         le.Detail,
		Instance:       c.Request.URL.Path,
		Errors:         fields,
		CurrentVersion: current,
	}
	body, _ := json.Marshal(p)
	c.Header("Vary", "Accept-Language")
	c.Data(status, "application/problem+json", body)
	c.Abort()
}

// dashed converts a snake_case code to the kebab-case URL fragment.
func dashed(code string) string {
	out := make([]byte, len(code))
	for i := 0; i < len(code); i++ {
		if code[i] == '_' {
			out[i] = '-'
		} else {
			out[i] = code[i]
		}
	}
	return string(out)
}
