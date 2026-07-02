package metrics_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/thee5176/reckonna/internal/metrics"
)

// collect gathers all metric data from a manual reader into a name→int64 sum map.
func collectSums(t *testing.T, reader sdkmetric.Reader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	sums := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if s, ok := m.Data.(metricdata.Sum[int64]); ok {
				var total int64
				for _, dp := range s.DataPoints {
					total += dp.Value
				}
				sums[m.Name] = total
			}
		}
	}
	return sums
}

// TestInstrumentNamesAndIncrement pins the exact OTel instrument names (the
// dashboard/Prometheus contract) and verifies the ledger-reject counter counts.
// OTel `reckonna.ledger.rejected` (monotonic) becomes Prometheus
// `reckonna_ledger_rejected_total`; `reckonna.http.server.requests` becomes
// `reckonna_http_server_requests_total`.
func TestInstrumentNamesAndIncrement(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	require.NoError(t, metrics.Init())

	ctx := context.Background()
	metrics.RecordLedgerRejected(ctx, "unbalanced_entry")
	metrics.RecordLedgerRejected(ctx, "unbalanced_entry")
	metrics.RecordHTTPRequest(ctx, "POST", "/command/journal-entries", 422)

	sums := collectSums(t, reader)

	got, ok := sums["reckonna.ledger.rejected"]
	require.True(t, ok, "instrument reckonna.ledger.rejected must exist")
	assert.Equal(t, int64(2), got, "counter incremented twice")

	_, ok = sums["reckonna.http.server.requests"]
	require.True(t, ok, "instrument reckonna.http.server.requests must exist")
}
