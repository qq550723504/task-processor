package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

func TestEnabledTraceRuntimeExportsConfiguredServiceName(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		want        string
	}{
		{name: "default", want: "task-processor"},
		{name: "custom", serviceName: "listing-api-test", want: "listing-api-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := &recordingSpanExporter{}
			runtime, err := newTraceRuntime(t.Context(), Config{
				Enabled:     true,
				ServiceName: tt.serviceName,
			}, func(context.Context, Config) (sdktrace.SpanExporter, error) {
				return exporter, nil
			})
			if err != nil {
				t.Fatalf("newTraceRuntime() error = %v", err)
			}

			_, span := runtime.provider.Tracer("resource-test").Start(t.Context(), "resource-span")
			span.End()
			if err := runtime.Shutdown(t.Context()); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}

			spans := exporter.spans()
			if len(spans) != 1 {
				t.Fatalf("exported spans = %d, want 1", len(spans))
			}
			got, ok := resourceAttribute(spans[0], attribute.Key("service.name"))
			if !ok {
				t.Fatal("exported span resource has no service.name attribute")
			}
			if got != tt.want {
				t.Fatalf("service.name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTraceRuntimeConstructionAndWrappingDoNotReplaceGlobalProvider(t *testing.T) {
	globalBefore := otel.GetTracerProvider()
	runtime, err := newTraceRuntime(t.Context(), Config{Enabled: true}, func(context.Context, Config) (sdktrace.SpanExporter, error) {
		return &recordingSpanExporter{}, nil
	})
	if err != nil {
		t.Fatalf("newTraceRuntime() error = %v", err)
	}
	if got := otel.GetTracerProvider(); got != globalBefore {
		t.Fatalf("global provider changed during construction: before=%T(%p), after=%T(%p)", globalBefore, globalBefore, got, got)
	}

	_ = runtime.WrapHTTPHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "global-provider-test")
	if got := otel.GetTracerProvider(); got != globalBefore {
		t.Fatalf("global provider changed during wrapping: before=%T(%p), after=%T(%p)", globalBefore, globalBefore, got, got)
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestTraceRuntimeBatchesExportsWithoutBlockingSpanEndAndFlushesOnShutdown(t *testing.T) {
	exporter := newBlockingSpanExporter()
	runtime, err := newTraceRuntime(t.Context(), Config{Enabled: true}, func(context.Context, Config) (sdktrace.SpanExporter, error) {
		return exporter, nil
	})
	if err != nil {
		t.Fatalf("newTraceRuntime() error = %v", err)
	}

	_, span := runtime.provider.Tracer("batch-test").Start(t.Context(), "batch-span")
	endReturned := make(chan struct{})
	go func() {
		span.End()
		close(endReturned)
	}()

	exportStarted := false
	select {
	case <-endReturned:
	case <-exporter.exportStarted:
		exportStarted = true
		select {
		case <-endReturned:
		case <-time.After(time.Second):
			close(exporter.release)
			t.Fatal("Span.End waited for the blocked exporter; want batch processing")
		}
	case <-time.After(time.Second):
		close(exporter.release)
		t.Fatal("Span.End did not return")
	}

	shutdownReturned := make(chan error, 1)
	go func() {
		shutdownReturned <- runtime.Shutdown(t.Context())
	}()
	if !exportStarted {
		select {
		case <-exporter.exportStarted:
		case err := <-shutdownReturned:
			close(exporter.release)
			t.Fatalf("Shutdown() returned before exporting the queued span: %v", err)
		case <-time.After(time.Second):
			close(exporter.release)
			t.Fatal("Shutdown() did not flush the queued span")
		}
	}
	select {
	case err := <-shutdownReturned:
		close(exporter.release)
		t.Fatalf("Shutdown() returned while exporter was blocked: %v", err)
	default:
	}
	close(exporter.release)
	select {
	case err := <-shutdownReturned:
		if err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown() did not return after exporter was released")
	}
}

func TestTraceRuntimeShutdownIsConcurrentIdempotentAndPreservesError(t *testing.T) {
	wantErr := errors.New("shutdown failed")
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	runtime := &TraceRuntime{shutdown: func(context.Context) error {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return wantErr
	}}

	const callers = 32
	start := make(chan struct{})
	results := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			results <- runtime.Shutdown(t.Context())
		}()
	}
	close(start)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("underlying shutdown was not called")
	}
	close(release)
	for range callers {
		select {
		case err := <-results:
			if err != wantErr {
				t.Fatalf("Shutdown() error = %v, want the same sentinel %v", err, wantErr)
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent Shutdown() call did not return")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("underlying shutdown calls = %d, want 1", got)
	}
}

func TestTraceRuntimeShutdownIsSafeForNilAndZeroValues(t *testing.T) {
	var nilRuntime *TraceRuntime
	if err := nilRuntime.Shutdown(t.Context()); err != nil {
		t.Fatalf("nil runtime Shutdown() error = %v", err)
	}
	if err := (&TraceRuntime{}).Shutdown(t.Context()); err != nil {
		t.Fatalf("zero runtime Shutdown() error = %v", err)
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

func TestWrappedHandlerContinuesW3CTraceContext(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	runtime := &TraceRuntime{provider: provider, shutdown: provider.Shutdown}
	handler := runtime.WrapHTTPHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "product-listing-api")

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	wantTraceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	wantParentSpanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	if got := ended[0].SpanContext().TraceID(); got != wantTraceID {
		t.Fatalf("trace ID = %s, want continued %s", got, wantTraceID)
	}
	parent := ended[0].Parent()
	if parent.SpanID() != wantParentSpanID {
		t.Fatalf("parent span ID = %s, want %s", parent.SpanID(), wantParentSpanID)
	}
	if !parent.IsRemote() {
		t.Fatal("parent span context is local, want remote upstream context")
	}
}

type recordingSpanExporter struct {
	mu            sync.Mutex
	exportedSpans int
	exported      []sdktrace.ReadOnlySpan
	shutdowns     int
}

func (e *recordingSpanExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exportedSpans += len(spans)
	e.exported = append(e.exported, spans...)
	return nil
}

func (e *recordingSpanExporter) spans() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdktrace.ReadOnlySpan(nil), e.exported...)
}

func resourceAttribute(span sdktrace.ReadOnlySpan, key attribute.Key) (string, bool) {
	for _, value := range span.Resource().Attributes() {
		if value.Key == key {
			return value.Value.AsString(), true
		}
	}
	return "", false
}

type blockingSpanExporter struct {
	exportStarted chan struct{}
	release       chan struct{}
	once          sync.Once
}

func newBlockingSpanExporter() *blockingSpanExporter {
	return &blockingSpanExporter{
		exportStarted: make(chan struct{}),
		release:       make(chan struct{}),
	}
}

func (e *blockingSpanExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	e.once.Do(func() { close(e.exportStarted) })
	<-e.release
	return nil
}

func (*blockingSpanExporter) Shutdown(context.Context) error { return nil }

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
