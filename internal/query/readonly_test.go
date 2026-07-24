package query_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCmdQueryHasNoWritePath enforces CQRS read-purity at the package-import
// level (IT9): the query binary must not transitively import any write-side
// package (the command repository, the command service, or the command HTTP
// handler). It walks cmd/query's full dependency set via `go list -deps`.
func TestCmdQueryHasNoWritePath(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/thee5176/reckonna/cmd/query").CombinedOutput()
	require.NoErrorf(t, err, "go list failed: %s", out)
	deps := string(out)

	forbidden := []string{
		"github.com/thee5176/reckonna/internal/repository/command",
		"github.com/thee5176/reckonna/internal/service",
		"github.com/thee5176/reckonna/internal/handler/command",
	}
	for _, pkg := range forbidden {
		assert.NotContainsf(t, lines(deps), pkg,
			"cmd/query must NOT import write-side package %q (CQRS purity)", pkg)
	}

	// Sanity: it DOES depend on the read repository.
	assert.Contains(t, deps, "github.com/thee5176/reckonna/internal/repository/query")
}

// lines splits go list output into a slice for exact package matching (so a
// substring like ".../query" never accidentally matches ".../command").
func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
