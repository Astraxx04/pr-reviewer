package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const serviceName = "pr-reviewer"

// Init initialises the global OTel tracer provider, selected by
// OTEL_EXPORTER_OTLP_ENDPOINT:
//   - unset    -> no-op provider (no tracing overhead, nothing exported)
//   - "stdout" -> spans are pretty-printed to the process stdout
//   - anything else -> treated as an OTLP/HTTP collector endpoint
//
// Returns a shutdown function to call on process exit.
func Init(ctx context.Context) (shutdown func(context.Context) error, err error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	var exporter sdktrace.SpanExporter
	if endpoint == "stdout" {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	} else {
		exporter, err = otlptracehttp.New(ctx)
	}
	if err != nil {
		return nil, err
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// Tracer returns the package-level tracer for pr-reviewer.
func Tracer() trace.Tracer {
	return otel.Tracer(serviceName)
}
