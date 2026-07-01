package config_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thee5176/reckonna/internal/config"
)

// TestI18nCoverage asserts every account code in config/coa.yaml AND every error
// code in the registry has a translation in every shipped locale (IT14).
func TestI18nCoverage(t *testing.T) {
	dir := config.LocalesDir()
	bundle, err := config.LoadBundle(dir)
	require.NoError(t, err)

	codes := accountCodes(t, filepath.Join(filepath.Dir(dir), "config", "coa.yaml"))
	require.NotEmpty(t, codes)

	for _, lang := range config.ShippedLocales {
		coa := bundle.CoA(lang)
		for _, code := range codes {
			key := strconv.Itoa(code)
			name, ok := coa[key]
			require.Truef(t, ok, "locale %q missing coa translation for %s", lang, key)
			require.NotEmptyf(t, name, "locale %q has empty coa name for %s", lang, key)
		}
		for _, code := range config.ErrorCodeRegistry {
			le := bundle.Error(lang, code)
			require.NotEqualf(t, code, le.Title, "locale %q missing error title for %q", lang, code)
			require.NotEmptyf(t, le.Detail, "locale %q missing error detail for %q", lang, code)
		}
	}
}

func accountCodes(t *testing.T, path string) []int {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc struct {
		Accounts []struct {
			Code int `yaml:"code"`
		} `yaml:"accounts"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	codes := make([]int, len(doc.Accounts))
	for i, a := range doc.Accounts {
		codes[i] = a.Code
	}
	return codes
}
