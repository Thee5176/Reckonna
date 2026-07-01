// Command gen-coa reads config/coa.yaml, validates it against the CoA
// governance standard (doc/coa-governance-standard.md), and emits the account
// seed migration plus any missing locale stubs. Run via `make gen-coa` (which
// calls `go run ./scripts/gen-coa`) — in CI so a hand-edited seed or an
// untranslated account fails the build.
//
// It is intentionally dependency-light (stdlib + yaml) and keeps its validation
// and SQL-emission logic as exported-in-package functions so gen_test.go can
// exercise them without touching the filesystem.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Account mirrors one entry in config/coa.yaml (standard §6 attributes).
type Account struct {
	Code               int      `yaml:"code"`
	Name               string   `yaml:"name"`
	Description        string   `yaml:"description"`
	Type               string   `yaml:"type"`
	NormalBalance      string   `yaml:"normal_balance"`
	Postable           bool     `yaml:"postable"`
	CurrentNoncurrent  string   `yaml:"current_noncurrent"`
	IFRSLineItem       string   `yaml:"ifrs_line_item"`
	AllowedBooks       []string `yaml:"allowed_books"`
	RequiredDimensions []string `yaml:"required_dimensions"`
	Status             string   `yaml:"status"`
	Owner              string   `yaml:"owner"`
}

// CoA is the parsed config/coa.yaml.
type CoA struct {
	Version  int       `yaml:"version"`
	Accounts []Account `yaml:"accounts"`
}

// knownDimensions are the v1 dimension types (standard §7). required_dimensions
// may only reference these.
var knownDimensions = map[string]bool{"entity": true, "currency": true, "counterparty": true}

// knownBooks are the v1 books (standard §8). v1 has only base.
var knownBooks = map[string]bool{"base": true}

// typeRanges maps an account type to the §4 code ranges it may occupy. The
// 70000–99999 bands are "mixed" and shared by income + expense.
var typeRanges = map[string][][2]int{
	"asset":     {{10000, 19999}},
	"liability": {{20000, 29999}},
	"equity":    {{30000, 39999}},
	"income":    {{40000, 49999}, {70000, 79999}, {80000, 89999}, {90000, 99999}},
	"expense":   {{50000, 69999}, {70000, 79999}, {80000, 89999}, {90000, 99999}},
}

// normalForType is the type↔normal_balance consistency rule (§4).
var normalForType = map[string]string{
	"asset": "debit", "expense": "debit",
	"liability": "credit", "equity": "credit", "income": "credit",
}

// LoadCoA reads and parses config/coa.yaml.
func LoadCoA(path string) (*CoA, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read coa: %w", err)
	}
	var c CoA
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse coa: %w", err)
	}
	return &c, nil
}

// Validate checks every account against the governance standard and returns all
// violations (empty slice = valid). Collecting all errors — rather than failing
// on the first — makes a hand-edited file's problems visible in one CI run.
func (c *CoA) Validate() []error {
	var errs []error
	seenCode := map[int]bool{}
	seenName := map[string]bool{}

	for _, a := range c.Accounts {
		where := fmt.Sprintf("account %d (%q)", a.Code, a.Name)

		if _, ok := normalForType[a.Type]; !ok {
			errs = append(errs, fmt.Errorf("%s: unknown type %q", where, a.Type))
		} else if want := normalForType[a.Type]; a.NormalBalance != want {
			errs = append(errs, fmt.Errorf("%s: type %s requires normal_balance %s, got %q (§4)", where, a.Type, want, a.NormalBalance))
		}

		if ranges, ok := typeRanges[a.Type]; ok && !inRanges(a.Code, ranges) {
			errs = append(errs, fmt.Errorf("%s: code out of range for type %s (§4)", where, a.Type))
		}

		if seenCode[a.Code] {
			errs = append(errs, fmt.Errorf("%s: duplicate code", where))
		}
		seenCode[a.Code] = true
		if seenName[a.Name] {
			errs = append(errs, fmt.Errorf("%s: duplicate name (§5 R5.4)", where))
		}
		seenName[a.Name] = true

		// Required §6 attributes.
		if strings.TrimSpace(a.Description) == "" {
			errs = append(errs, fmt.Errorf("%s: missing description (§6)", where))
		}
		if strings.TrimSpace(a.IFRSLineItem) == "" {
			errs = append(errs, fmt.Errorf("%s: missing ifrs_line_item (§6)", where))
		}
		if strings.TrimSpace(a.Owner) == "" {
			errs = append(errs, fmt.Errorf("%s: missing owner (§6)", where))
		}
		if a.Status == "" {
			errs = append(errs, fmt.Errorf("%s: missing status (§6)", where))
		}
		if len(a.AllowedBooks) == 0 {
			errs = append(errs, fmt.Errorf("%s: missing allowed_books (§6)", where))
		}
		for _, bk := range a.AllowedBooks {
			if !knownBooks[bk] {
				errs = append(errs, fmt.Errorf("%s: allowed_books references unknown book %q (§8)", where, bk))
			}
		}

		// current_noncurrent required for asset/liability, absent otherwise (§6, R4.3).
		if a.Type == "asset" || a.Type == "liability" {
			if a.CurrentNoncurrent != "current" && a.CurrentNoncurrent != "non_current" {
				errs = append(errs, fmt.Errorf("%s: current_noncurrent required for %s (§6)", where, a.Type))
			}
		} else if a.CurrentNoncurrent != "" {
			errs = append(errs, fmt.Errorf("%s: current_noncurrent must be empty for %s (§6)", where, a.Type))
		}

		for _, d := range a.RequiredDimensions {
			if !knownDimensions[d] {
				errs = append(errs, fmt.Errorf("%s: required_dimensions references unknown dimension %q (§7)", where, d))
			}
		}
	}
	return errs
}

func inRanges(code int, ranges [][2]int) bool {
	for _, r := range ranges {
		if code >= r[0] && code <= r[1] {
			return true
		}
	}
	return false
}

// SeedSQL renders the up + down migration bodies for the account seed. The up
// is idempotent-ish via a fixed code list; the down deletes exactly those codes.
func (c *CoA) SeedSQL() (up, down string) {
	var b strings.Builder
	b.WriteString("-- 003_seed_account.up.sql — GENERATED by scripts/gen-coa from config/coa.yaml.\n")
	b.WriteString("-- Do not hand-edit; change config/coa.yaml and run `make gen-coa`. (plan 03 S5b)\n")
	b.WriteString("INSERT INTO account (code, name, description, type, normal_balance, postable, current_noncurrent, ifrs_line_item, allowed_books, required_dimensions, status, owner) VALUES\n")

	rows := make([]string, 0, len(c.Accounts))
	codes := make([]int, 0, len(c.Accounts))
	for _, a := range c.Accounts {
		rows = append(rows, fmt.Sprintf("  (%d, %s, %s, %s, %s, %t, %s, %s, %s, %s, %s, %s)",
			a.Code, q(a.Name), q(a.Description), q(a.Type), q(a.NormalBalance), a.Postable,
			nullable(a.CurrentNoncurrent), q(a.IFRSLineItem), pgArray(a.AllowedBooks),
			pgArray(a.RequiredDimensions), q(a.Status), q(a.Owner)))
		codes = append(codes, a.Code)
	}
	b.WriteString(strings.Join(rows, ",\n"))
	b.WriteString(";\n")

	sort.Ints(codes)
	strCodes := make([]string, len(codes))
	for i, cd := range codes {
		strCodes[i] = fmt.Sprintf("%d", cd)
	}
	down = fmt.Sprintf("-- 003_seed_account.down.sql — GENERATED by scripts/gen-coa.\nDELETE FROM account WHERE code IN (%s);\n", strings.Join(strCodes, ", "))
	return b.String(), down
}

// q single-quotes a SQL string literal, escaping embedded quotes.
func q(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// nullable renders ” as SQL NULL, else a quoted literal.
func nullable(s string) string {
	if s == "" {
		return "NULL"
	}
	return q(s)
}

// pgArray renders a Go string slice as a PG text[] literal, e.g. '{base}'.
func pgArray(xs []string) string {
	if len(xs) == 0 {
		return "'{}'"
	}
	quoted := make([]string, len(xs))
	for i, x := range xs {
		quoted[i] = `"` + strings.ReplaceAll(x, `"`, `\"`) + `"`
	}
	return "'{" + strings.Join(quoted, ",") + "}'"
}

func main() {
	root := repoRoot()
	coaPath := filepath.Join(root, "config", "coa.yaml")

	coa, err := LoadCoA(coaPath)
	if err != nil {
		fatal(err)
	}
	if errs := coa.Validate(); len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "gen-coa: config/coa.yaml violates the governance standard:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %v\n", e)
		}
		os.Exit(1)
	}

	up, down := coa.SeedSQL()
	if err := os.WriteFile(filepath.Join(root, "db", "migration", "003_seed_account.up.sql"), []byte(up), 0o644); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "db", "migration", "003_seed_account.down.sql"), []byte(down), 0o644); err != nil {
		fatal(err)
	}

	codes := make([]int, len(coa.Accounts))
	names := make(map[int]string, len(coa.Accounts))
	for i, a := range coa.Accounts {
		codes[i] = a.Code
		names[a.Code] = a.Name
	}
	for _, loc := range []string{"en", "ja"} {
		path := filepath.Join(root, "locales", loc+".json")
		added, err := syncLocale(path, codes, names)
		if err != nil {
			fatal(err)
		}
		if added > 0 {
			fmt.Printf("gen-coa: %s stubbed %d missing coa key(s)\n", loc, added)
		}
	}
	fmt.Printf("gen-coa: validated %d accounts; wrote 003_seed_account.{up,down}.sql\n", len(coa.Accounts))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "gen-coa: %v\n", err)
	os.Exit(1)
}

// repoRoot walks up from the CWD to the module root (dir containing go.mod).
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fatal(fmt.Errorf("go.mod not found from CWD"))
		}
		dir = parent
	}
}
