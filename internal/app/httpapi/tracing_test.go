package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	platformobservability "task-processor/internal/platform/observability"
)

func TestBuildRuntimeDepsTranslatesTraceConfigBeforeClosableResources(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	t.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", "true")
	wantErr := errors.New("stop after tracing")
	trace := &stubTraceRuntime{}
	var gotConfig platformobservability.Config
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := "observability:\n" +
		"  tracing:\n" +
		"    enabled: true\n" +
		"    serviceName: listing-api-test\n" +
		"    endpoint: collector.test:4317\n" +
		"    insecure: true\n" +
		"database:\n" +
		"  host: localhost\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	_, err := buildRuntimeDepsWithBuilders(logrus.New(), configPath, runtimeDepsBuilders{
		buildTraceRuntime: func(_ context.Context, cfg platformobservability.Config) (traceRuntime, error) {
			gotConfig = cfg
			return trace, nil
		},
		migrateSchema: func(context.Context, *config.DatabaseConfig, *logrus.Logger) error {
			return wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildRuntimeDepsWithBuilders() error = %v, want %v", err, wantErr)
	}
	wantConfig := platformobservability.Config{
		Enabled:     true,
		ServiceName: "listing-api-test",
		Endpoint:    "collector.test:4317",
		Insecure:    true,
	}
	if gotConfig != wantConfig {
		t.Fatalf("trace config = %#v, want %#v", gotConfig, wantConfig)
	}
	if trace.shutdownCalls != 1 {
		t.Fatalf("trace shutdown calls after later construction failure = %d, want 1", trace.shutdownCalls)
	}
}

func TestBuildRuntimeDepsTraceConstructionFailureStopsBeforeSchemaMigration(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	t.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", "true")
	wantErr := errors.New("trace construction failed")
	migrationCalled := false

	_, err := buildRuntimeDepsWithBuilders(logrus.New(), "../../../config/config-test.yaml", runtimeDepsBuilders{
		buildTraceRuntime: func(context.Context, platformobservability.Config) (traceRuntime, error) {
			return nil, wantErr
		},
		migrateSchema: func(context.Context, *config.DatabaseConfig, *logrus.Logger) error {
			migrationCalled = true
			return nil
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildRuntimeDepsWithBuilders() error = %v, want %v", err, wantErr)
	}
	if migrationCalled {
		t.Fatal("schema migration ran after trace construction failure")
	}
}

func TestBuildBootstrapWrapsFinalHandlerAndPreservesRealHTTPResponse(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	traceRuntime := &recordingTraceRuntime{provider: provider}
	cfg := config.NewDefaultConfig()
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{cfg: cfg, traceRuntime: traceRuntime},
		features: &featureRuntimeState{},
	}

	bootstrap, err := buildBootstrapWithDependencies(logrus.New(), Options{}, bootstrapBuildDependencies{
		buildRuntimeDeps: func(*logrus.Logger, string) (*runtimeDeps, error) { return deps, nil },
		buildComposition: func(*logrus.Logger, *runtimeDeps) (httpFeatureComposition, error) {
			return httpFeatureComposition{}, nil
		},
		buildRuntimeBundle: func(httpFeatureComposition, *config.Config) (runtimeBundle, error) {
			return runtimeBundle{routes: []httproute.Descriptor{{
				Method:     http.MethodGet,
				Path:       "/traced",
				Module:     "test",
				AuthPolicy: httproute.AuthPolicyPublic,
				Handler: func(c *gin.Context) {
					c.String(http.StatusCreated, "preserved-response")
				},
			}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("buildBootstrapWithDependencies() error = %v", err)
	}

	response := httptest.NewRecorder()
	bootstrap.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/traced", nil))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if response.Body.String() != "preserved-response" {
		t.Fatalf("body = %q, want preserved-response", response.Body.String())
	}
	if !traceRuntime.wrapped {
		t.Fatal("final HTTP server handler was not wrapped by the trace runtime")
	}
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if ended[0].SpanKind() != trace.SpanKindServer {
		t.Fatalf("span kind = %s, want server", ended[0].SpanKind())
	}
}

type stubTraceRuntime struct {
	shutdownCalls int
}

func (r *stubTraceRuntime) WrapHTTPHandler(handler http.Handler, _ string) http.Handler {
	return handler
}

func (r *stubTraceRuntime) Shutdown(context.Context) error {
	r.shutdownCalls++
	return nil
}

type recordingTraceRuntime struct {
	provider *sdktrace.TracerProvider
	wrapped  bool
}

func (r *recordingTraceRuntime) WrapHTTPHandler(handler http.Handler, operation string) http.Handler {
	r.wrapped = true
	return otelhttp.NewHandler(handler, operation, otelhttp.WithTracerProvider(r.provider))
}

func (r *recordingTraceRuntime) Shutdown(ctx context.Context) error {
	return r.provider.Shutdown(ctx)
}
