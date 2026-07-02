// Package metrics defines the application-level OTel instruments and their
// recording helpers. It is neutral (imports only OTel) so both the command and
// query sides can use it without breaking compile-time CQRS purity (IT9).
//
// Instrument names use dots (OTel convention). Exported to the collector and on
// to Prometheus they become underscores, and monotonic counters gain a `_total`
// suffix, so:
//
//	reckonna.http.server.requests -> reckonna_http_server_requests_total
//	reckonna.ledger.rejected      -> reckonna_ledger_rejected_total
package metrics

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/thee5176/reckonna"

var (
	httpRequests   metric.Int64Counter
	ledgerRejected metric.Int64Counter
)

// Init creates the instruments from the global meter provider. Call it AFTER
// config.SetupTelemetry has installed the provider, otherwise the instruments
// bind to the no-op provider and never export.
func Init() error {
	m := otel.Meter(meterName)
	var err error
	if httpRequests, err = m.Int64Counter(
		"reckonna.http.server.requests",
		metric.WithDescription("Count of HTTP server requests (RED rate/errors)."),
	); err != nil {
		return err
	}
	if ledgerRejected, err = m.Int64Counter(
		"reckonna.ledger.rejected",
		metric.WithDescription("Journal entries rejected for a broken invariant (e.g. 借方≠貸方)."),
	); err != nil {
		return err
	}
	return nil
}

// RecordHTTPRequest increments the request counter with low-cardinality labels.
func RecordHTTPRequest(ctx context.Context, method, route string, status int) {
	if httpRequests == nil {
		return
	}
	httpRequests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status),
	))
}

// RecordLedgerRejected increments the ledger-reject counter, labelled by the
// RFC 7807 error code (e.g. unbalanced_entry).
func RecordLedgerRejected(ctx context.Context, code string) {
	if ledgerRejected == nil {
		return
	}
	ledgerRejected.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", code)))
}
