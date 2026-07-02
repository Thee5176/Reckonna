package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thee5176/reckonna/internal/config"
	"github.com/thee5176/reckonna/internal/handler/problem"
)

// Locale resolves the request locale from Accept-Language and stores it on the
// context for the problem writer (localized title/detail). Only shipped locales
// are honoured; otherwise the default (en) is used. Sets Vary: Accept-Language.
func Locale() gin.HandlerFunc {
	shipped := map[string]bool{}
	for _, l := range config.ShippedLocales {
		shipped[l] = true
	}
	return func(c *gin.Context) {
		c.Header("Vary", "Accept-Language")
		lang := config.DefaultLocale
		if tag := primaryLang(c.GetHeader("Accept-Language")); tag != "" && shipped[tag] {
			lang = tag
		}
		c.Set(problem.LocaleKey, lang)
		c.Next()
	}
}

// primaryLang extracts the primary subtag of the first Accept-Language entry,
// e.g. "ja,en;q=0.8" -> "ja", "en-US" -> "en". Quality ordering beyond the first
// entry is not honoured in v1.
func primaryLang(header string) string {
	if header == "" {
		return ""
	}
	first := strings.TrimSpace(strings.Split(header, ",")[0])
	first = strings.TrimSpace(strings.Split(first, ";")[0])
	return strings.ToLower(strings.Split(first, "-")[0])
}
