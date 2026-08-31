package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestDisabledTraceRuntimeDoesNotCallExporterFactory(t *testing.T) {
	configType := reflect.TypeOf(Config{})
	if configType.NumField() != 4 {
		t.Fatalf("Config fields = %d, want the four approved transport values", configType.NumField())
	}
	factoryCalled := false
	runtime, err := newTraceRuntime(t.Context(), Config{
		Enabled:     false,
		ServiceName: "disabled-service",
		Endpoint:    "127.0.0.1:1",
	}, func(context.Context, Config) (sdktrace.SpanExporter, error) {
		factoryCalled = true
		return nil, errors.New("disabled tracing must not construct an exporter")
	})
	if err != nil {
		t.Fatalf("newTraceRuntime() error = %v", err)
	}
	if factoryCalled {
		t.Fatal("disabled tracing called the exporter factory")
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestEnabledTraceRuntimeDefaultsServiceNameAndOwnsExporterLifecycle(t *testing.T) {
	exporter := &recordingSpanExporter{}
	var factoryConfig Config
	runtime, err := newTraceRuntime(t.Context(), Config{
		Enabled:  true,
		Endpoint: "collector.internal:4317",
		Insecure: true,
	}, func(_ context.Context, cfg Config) (sdktrace.SpanExporter, error) {
		factoryConfig = cfg
		return exporter, nil
	})
	if err != nil {
		t.Fatalf("newTraceRuntime() error = %v", err)
	}
	if factoryConfig.ServiceName != "task-processor" {
		t.Fatalf("exporter factory service name = %q, want task-processor", factoryConfig.ServiceName)
	}
	if factoryConfig.Endpoint != "collector.internal:4317" || !factoryConfig.Insecure {
		t.Fatalf("exporter factory config = %#v, want endpoint and insecure preserved", factoryConfig)
	}

	_, span := runtime.provider.Tracer("test").Start(t.Context(), "owned-span")
	span.End()
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("first Shutdown() error = %v", err)
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}

	exported, shutdowns := exporter.counts()
	if exported != 1 {
		t.Fatalf("exported spans = %d, want 1", exported)
	}
	if shutdowns != 1 {
		t.Fatalf("exporter shutdown calls = %d, want idempotent 1", shutdowns)
	}
}

func TestEnabledTraceRuntimeReturnsExporterConstructionError(t *testing.T) {
	wantErr := errors.New("invalid exporter configuration")
	_, err := newTraceRuntime(t.Context(), Config{Enabled: true}, func(context.Context, Config) (sdktrace.SpanExporter, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("newTraceRuntime() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestWrappedHandlerRecordsServerSpanAndPreservesResponse(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	runtime := &TraceRuntime{provider: provider, shutdown: provider.Shutdown}
	handler := runtime.WrapHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("preserved-body"))
	}), "product-listing-api")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/listings", nil))

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if response.Body.String() != "preserved-body" {
		t.Fatalf("body = %q, want preserved-body", response.Body.String())
	}
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if ended[0].SpanKind() != trace.SpanKindServer {
		t.Fatalf("span kind = %s, want server", ended[0].SpanKind())
	}
}

type recordingSpanExporter struct {
	mu            sync.Mutex
	exportedSpans int
	shutdowns     int
}

func (e *recordingSpanExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exportedSpans += len(spans)
	return nil
}

func (e *recordingSpanExporter) Shutdown(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdowns++
	return nil
}

func (e *recordingSpanExporter) counts() (int, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exportedSpans, e.shutdowns
}
