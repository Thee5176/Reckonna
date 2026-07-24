package config

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// SetupTelemetry installs global OTel tracer AND meter providers. When
// OTLPEndpoint is set, spans + metrics are exported over OTLP/HTTP (the RED
// dashboard needs the meter provider — traces alone leave it empty); otherwise
// no-export providers are installed so instrumentation still functions. Returns
// a single shutdown func that flushes both. Vendor-neutral (OTLP).
func SetupTelemetry(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("reckonna-"+cfg.ServiceName),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	// Exporters read OTEL_EXPORTER_OTLP_ENDPOINT + OTEL_EXPORTER_OTLP_PROTOCOL
	// from the environment themselves, which (per the OTLP spec) appends the
	// signal-specific path — /v1/traces, /v1/metrics — to the base collector
	// URL. We only decide HERE whether to wire a real exporter at all (so dev /
	// tests without a collector install no-export providers and never dial out).
	exportOTLP := cfg.OTLPEndpoint != ""

	// ── Traces ──
	traceOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if exportOTLP {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("otlp trace exporter: %w", err)
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(traceOpts...)
	otel.SetTracerProvider(tp)

	// ── Metrics (RED dashboard) ──
	meterOpts := []metric.Option{metric.WithResource(res)}
	if exportOTLP {
		mexp, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("otlp metric exporter: %w", err)
		}
		meterOpts = append(meterOpts, metric.WithReader(metric.NewPeriodicReader(mexp)))
	}
	mp := metric.NewMeterProvider(meterOpts...)
	otel.SetMeterProvider(mp)

	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}
	return shutdown, nil
}
