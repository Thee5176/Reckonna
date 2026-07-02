package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ErrorCodeRegistry is the locale-neutral error-code registry (plan 03 §API
// Conventions). Every code must have a translation in every shipped locale
// (IT14). Clients map a code to a localized message.
var ErrorCodeRegistry = []string{
	"unbalanced_entry", "mixed_currency", "missing_required_dimension",
	"unknown_account_code", "invalid_cursor", "concurrency_conflict",
	"duplicate_idempotency_key", "unsupported_media_type", "unauthorized",
	"forbidden", "not_found", "validation_failed",
}

// ShippedLocales are the languages v1 ships (extensible without schema change).
var ShippedLocales = []string{"en", "ja"}

// DefaultLocale is the fallback when the request locale is unavailable.
const DefaultLocale = "en"

// LocalizedError is a code's human-facing title + detail in one locale.
type LocalizedError struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// Bundle holds all loaded translations: account display names (by code) and
// error messages (by code), per locale.
type Bundle struct {
	coa  map[string]map[string]string         // lang -> code -> display name
	errs map[string]map[string]LocalizedError // lang -> code -> {title, detail}
}

type coaLocaleFile struct {
	Coa map[string]string `json:"coa"`
}

type errLocaleFile struct {
	Errors map[string]LocalizedError `json:"errors"`
}

// LoadBundle reads locales/<lang>.json (account names) and
// locales/errors.<lang>.json (error messages) for every shipped locale.
func LoadBundle(dir string) (*Bundle, error) {
	b := &Bundle{
		coa:  map[string]map[string]string{},
		errs: map[string]map[string]LocalizedError{},
	}
	for _, lang := range ShippedLocales {
		var cf coaLocaleFile
		if err := readJSON(filepath.Join(dir, lang+".json"), &cf); err != nil {
			return nil, err
		}
		b.coa[lang] = cf.Coa

		var ef errLocaleFile
		if err := readJSON(filepath.Join(dir, "errors."+lang+".json"), &ef); err != nil {
			return nil, err
		}
		b.errs[lang] = ef.Errors
	}
	return b, nil
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read locale %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("parse locale %s: %w", path, err)
	}
	return nil
}

// Error returns the localized title/detail for a code, falling back to the
// default locale then to the code itself.
func (b *Bundle) Error(lang, code string) LocalizedError {
	if m, ok := b.errs[lang]; ok {
		if le, ok := m[code]; ok {
			return le
		}
	}
	if le, ok := b.errs[DefaultLocale][code]; ok {
		return le
	}
	return LocalizedError{Title: code, Detail: code}
}

// CoA returns the code→display-name map for a locale (empty if unknown).
func (b *Bundle) CoA(lang string) map[string]string { return b.coa[lang] }

// LocalesDir resolves the locales directory: RECKONNA_LOCALES_DIR if set, else
// <module-root>/locales (module root = nearest ancestor with go.mod).
func LocalesDir() string {
	if d := os.Getenv("RECKONNA_LOCALES_DIR"); d != "" {
		return d
	}
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "locales")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "locales"
		}
		dir = parent
	}
}
