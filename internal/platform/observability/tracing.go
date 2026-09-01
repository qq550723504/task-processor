package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"
)

const defaultServiceName = "task-processor"

type exporterFactory func(context.Context, Config) (sdktrace.SpanExporter, error)

// TraceRuntime owns an isolated tracer provider and its shutdown lifecycle.
type TraceRuntime struct {
	provider trace.TracerProvider
	shutdown func(context.Context) error

	shutdownOnce sync.Once
	shutdownErr  error
}

// NewTraceRuntime constructs a tracing runtime without changing global OTel state.
func NewTraceRuntime(ctx context.Context, cfg Config) (*TraceRuntime, error) {
	return newTraceRuntime(ctx, cfg, newOTLPExporter)
}

func newTraceRuntime(ctx context.Context, cfg Config, buildExporter exporterFactory) (*TraceRuntime, error) {
	if strings.TrimSpace(cfg.ServiceName) == "" {
		cfg.ServiceName = defaultServiceName
	}

	if !cfg.Enabled {
		return &TraceRuntime{
			provider: trace.NewNoopTracerProvider(),
			shutdown: func(context.Context) error { return nil },
		}, nil
	}
	if buildExporter == nil {
		return nil, fmt.Errorf("trace exporter factory is nil")
	}

	exporter, err := buildExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build OTLP trace exporter: %w", err)
	}
	if exporter == nil {
		return nil, fmt.Errorf("build OTLP trace exporter: exporter is nil")
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)))
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, fmt.Errorf("build trace resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
	)
	return &TraceRuntime{provider: provider, shutdown: provider.Shutdown}, nil
}

func newOTLPExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	options := make([]otlptracegrpc.Option, 0, 2)
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		options = append(options, otlptracegrpc.WithEndpoint(endpoint))
	}
	if cfg.Insecure {
		options = append(options, otlptracegrpc.WithInsecure())
	}
	return otlptracegrpc.New(ctx, options...)
}

// WrapHTTPHandler instruments handler with this runtime's provider.
func (r *TraceRuntime) WrapHTTPHandler(handler http.Handler, operation string) http.Handler {
	if r == nil || r.provider == nil {
		return handler
	}
	return otelhttp.NewHandler(
		handler,
		operation,
		otelhttp.WithTracerProvider(r.provider),
		otelhttp.WithPropagators(propagation.TraceContext{}),
	)
}

// Shutdown flushes and closes the owned provider once.
func (r *TraceRuntime) Shutdown(ctx context.Context) error {
	if r == nil || r.shutdown == nil {
		return nil
	}
	r.shutdownOnce.Do(func() {
		r.shutdownErr = r.shutdown(ctx)
	})
	return r.shutdownErr
}
