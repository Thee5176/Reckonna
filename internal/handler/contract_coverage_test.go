package handler_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ginParam matches a gin path segment like ":id" so it can be rewritten to
// the OpenAPI "{id}" form for lookup in the spec.
var ginParam = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// TestContract_RoutesMatchSpec asserts every route wired by
// newContractRouter — which mirrors cmd/command/main.go +
// cmd/query/main.go's registration exactly — has a matching path+method in
// api/openapi.yaml. This is the drift guard: a route added to a handler
// without a spec update fails here.
func TestContract_RoutesMatchSpec(t *testing.T) {
	r := newContractRouter(t)
	doc, _ := loadSpec(t)

	routes := r.Routes()
	require.NotEmpty(t, routes, "router registered no routes — test setup is broken")

	for _, ri := range routes {
		specPath := ginParam.ReplaceAllString(ri.Path, "{$1}")
		item := doc.Paths.Find(specPath)
		if !assert.NotNilf(t, item, "route %s %s has no matching path in api/openapi.yaml", ri.Method, ri.Path) {
			continue
		}
		op := item.GetOperation(ri.Method)
		assert.NotNilf(t, op, "route %s %s has no matching operation in api/openapi.yaml", ri.Method, ri.Path)
	}
}

// TestContract_SpecHasNoUnregisteredPaths is the inverse check: every path in
// api/openapi.yaml corresponds to a route actually registered by the
// services, catching a spec entry that documents an endpoint nobody wired up.
func TestContract_SpecHasNoUnregisteredPaths(t *testing.T) {
	r := newContractRouter(t)
	doc, _ := loadSpec(t)

	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		specPath := ginParam.ReplaceAllString(ri.Path, "{$1}")
		registered[ri.Method+" "+specPath] = true
	}

	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			key := method + " " + path
			assert.Truef(t, registered[key], "api/openapi.yaml documents %s but no handler registers it", key)
		}
	}
}
