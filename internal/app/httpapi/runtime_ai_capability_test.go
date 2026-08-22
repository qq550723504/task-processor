package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/prompt"
	"task-processor/internal/shared/aiidentity"
)

func TestBuildAICapabilityRuntimeDepsKeepsLegacyModeDependencyFree(t *testing.T) {
	deps, err := buildAICapabilityRuntimeDeps(&config.Config{
		AICapability: config.AICapabilityConfig{StudioImageRoutingMode: "legacy"},
	}, logrus.New())
	if err != nil {
		t.Fatalf("buildAICapabilityRuntimeDeps() error = %v", err)
	}
	if deps.invocationRecorder != nil {
		t.Fatal("expected legacy mode to omit invocation recorder")
	}
	if deps.asyncJobStore != nil {
		t.Fatal("expected legacy mode to omit async job binding store")
	}
	if len(deps.closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(deps.closers))
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseOutsideLegacy(t *testing.T) {
	for _, mode := range []string{"shadow", "active", "SHADOW", "Active"} {
		t.Run(mode, func(t *testing.T) {
			_, err := buildAICapabilityRuntimeDeps(&config.Config{
				AICapability: config.AICapabilityConfig{StudioImageRoutingMode: mode},
			}, logrus.New())
			if err == nil {
				t.Fatal("expected missing database error")
			}
			if !strings.Contains(err.Error(), "AI capability") {
				t.Fatalf("error = %q, want AI capability resource context", err)
			}
			if strings.Contains(err.Error(), "parse AI capability") {
				t.Fatalf("error = %q, case-insensitive validated mode must reach runtime dependency checks", err)
			}
		})
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseWhenProductImageSceneEnabled(t *testing.T) {
	_, err := buildAICapabilityRuntimeDeps(&config.Config{
		AICapability: config.AICapabilityConfig{
			StudioImageRoutingMode:   "legacy",
			ProductImageSceneEnabled: true,
		},
	}, logrus.New())
	if err == nil {
		t.Fatal("expected missing database error")
	}
	if !strings.Contains(err.Error(), "AI capability") {
		t.Fatalf("error = %q, want AI capability resource context", err)
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseWhenProductEnrichGovernanceEnabled(t *testing.T) {
	_, err := buildAICapabilityRuntimeDeps(&config.Config{
		AICapability: config.AICapabilityConfig{
			StudioImageRoutingMode:   "legacy",
			ProductEnrichTextEnabled: true,
		},
	}, logrus.New())
	if err == nil {
		t.Fatal("expected missing database error")
	}
	if !strings.Contains(err.Error(), "AI capability") {
		t.Fatalf("error = %q, want AI capability resource context", err)
	}
}

func TestProductEnrichInvocationErrorHandlerLogsLedgerFailure(t *testing.T) {
	logger := logrus.New()
	hook := &captureLogHook{}
	logger.AddHook(hook)

	productEnrichInvocationErrorHandler(logger)(aicapability.InvocationRecord{
		InvocationID: "invocation-1",
		Capability:   aicapability.CapabilityProductEnrichListing,
		Operation:    aicapability.OperationProductEnrichJSONGenerate,
	}, errors.New("ledger unavailable"))

	if len(hook.entries) != 1 {
		t.Fatalf("logged entries = %d, want 1", len(hook.entries))
	}
	entry := hook.entries[0]
	if entry.Message != "ai invocation ledger write failed" {
		t.Fatalf("message = %q", entry.Message)
	}
	if entry.Data["invocation_id"] != "invocation-1" || entry.Data["operation"] != string(aicapability.OperationProductEnrichJSONGenerate) {
		t.Fatalf("fields = %#v", entry.Data)
	}
}

func TestBuildProductEnrichRuntimeDepsGovernsFusionOnlyWithListingCapability(t *testing.T) {
	tests := []struct {
		name       string
		capability config.AICapabilityConfig
		wantFusion bool
	}{
		{
			name: "text only keeps legacy default fusion",
			capability: config.AICapabilityConfig{
				ProductEnrichTextEnabled:          true,
				ProductEnrichTextAllowedTenantIDs: []string{"tenant-a"},
			},
		},
		{
			name: "vision only keeps legacy default fusion",
			capability: config.AICapabilityConfig{
				ProductEnrichVisionEnabled:          true,
				ProductEnrichVisionAllowedTenantIDs: []string{"tenant-a"},
			},
		},
		{
			name: "listing enables governed default fusion",
			capability: config.AICapabilityConfig{
				ProductEnrichListingEnabled:          true,
				ProductEnrichListingAllowedTenantIDs: []string{"tenant-a"},
			},
			wantFusion: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
				Clients: map[string]*openaiclient.ClientConfig{
					"default": {APIKey: "static-default", BaseURL: "https://example.test", Model: "default-model"},
					"fast":    {APIKey: "static-fast", BaseURL: "https://example.test", Model: "fast-model"},
					"vision":  {APIKey: "static-vision", BaseURL: "https://example.test", Model: "vision-model"},
				},
				DefaultClient: "default",
			})
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			t.Cleanup(func() { _ = manager.Close() })
			cfg := &config.Config{AICapability: tt.capability}

			deps, err := buildProductEnrichRuntimeDeps(logrus.New(), cfg, manager, runtimeProductEnrichResolver{}, runtimeProductEnrichRecorder{})
			if err != nil {
				t.Fatalf("buildProductEnrichRuntimeDeps() error = %v", err)
			}
			if (deps.fusionGenerator != nil) != tt.wantFusion {
				t.Fatalf("fusion generator present = %v, want %v", deps.fusionGenerator != nil, tt.wantFusion)
			}
		})
	}
}

func TestBuildProductEnrichRuntimeDepsRecordsRegistryPromptKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-runtime-test", "object": "chat.completion", "created": 1, "model": "default-model",
			"choices": []map[string]any{{"index": 0, "message": map[string]string{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	t.Cleanup(server.Close)
	clientConfig := &openaiclient.ClientConfig{APIKey: "static-default", BaseURL: server.URL, Model: "default-model", Timeout: time.Second}
	resolver := runtimeStaticProductEnrichResolver{config: clientConfig}
	manager, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients: map[string]*openaiclient.ClientConfig{"default": clientConfig}, ConfigResolver: resolver, DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	recorder := &capturingRuntimeProductEnrichRecorder{}
	deps, err := buildProductEnrichRuntimeDeps(logrus.New(), &config.Config{AICapability: config.AICapabilityConfig{
		ProductEnrichListingEnabled: true, ProductEnrichListingAllowedTenantIDs: []string{"tenant-a"},
		ProductEnrichTextEnabled: true, ProductEnrichTextAllowedTenantIDs: []string{"tenant-a"},
		ProductEnrichVisionEnabled: true, ProductEnrichVisionAllowedTenantIDs: []string{"tenant-a"},
	}}, manager, resolver, recorder)
	if err != nil {
		t.Fatalf("buildProductEnrichRuntimeDeps() error = %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	tests := []struct {
		name    string
		invoke  func() error
		wantKey string
	}{
		{name: "product json", invoke: func() error { _, err := deps.contentGenerator.Generate(ctx, "rendered registry prompt"); return err }, wantKey: prompt.KProductEnrichGenerationProductJSON},
		{name: "specs", invoke: func() error { _, err := deps.specsGenerator.Generate(ctx, "rendered registry prompt"); return err }, wantKey: prompt.KProductEnrichGenerationSpecs},
		{name: "variants", invoke: func() error { _, err := deps.variantsGenerator.Generate(ctx, "rendered registry prompt"); return err }, wantKey: prompt.KProductEnrichGenerationVariants},
		{name: "text quality", invoke: func() error {
			_, err := deps.scoringTextGenerator.Generate(ctx, "rendered registry prompt")
			return err
		}, wantKey: prompt.KProductEnrichLlmScorerTextScoring},
		{name: "image quality", invoke: func() error {
			_, err := deps.scoringImageAnalyzer.AnalyzeImage(ctx, "https://example.test/image.jpg", "rendered registry prompt")
			return err
		}, wantKey: prompt.KProductEnrichLlmScorerImageScoring},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(recorder.records)
			if err := tt.invoke(); err != nil {
				t.Fatalf("invoke governed capability: %v", err)
			}
			if len(recorder.records) != before+1 {
				t.Fatalf("record count = %d, want %d", len(recorder.records), before+1)
			}
			if got := recorder.records[len(recorder.records)-1].PromptKey; got != tt.wantKey {
				t.Fatalf("PromptKey = %q, want %q", got, tt.wantKey)
			}
		})
	}
}

type runtimeProductEnrichResolver struct{}

func (runtimeProductEnrichResolver) ResolveClientConfig(_ context.Context, name string, fallback *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	if fallback == nil {
		return nil, nil
	}
	return &openaiclient.ResolvedClientConfig{CacheKey: "runtime-test:" + name, Config: fallback}, nil
}

type runtimeStaticProductEnrichResolver struct{ config *openaiclient.ClientConfig }

func (r runtimeStaticProductEnrichResolver) ResolveClientConfig(_ context.Context, name string, _ *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	return &openaiclient.ResolvedClientConfig{CacheKey: "runtime-static:" + name, Config: r.config}, nil
}

type runtimeProductEnrichRecorder struct{}

func (runtimeProductEnrichRecorder) RecordInvocation(context.Context, aicapability.InvocationRecord) error {
	return nil
}

type capturingRuntimeProductEnrichRecorder struct {
	records []aicapability.InvocationRecord
}

func (r *capturingRuntimeProductEnrichRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.records = append(r.records, record)
	return nil
}

type captureLogHook struct {
	entries []*logrus.Entry
}

func (h *captureLogHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *captureLogHook) Fire(entry *logrus.Entry) error {
	h.entries = append(h.entries, entry)
	return nil
}

func TestAutoMigrateProductListingAPIRuntimeSchemaCreatesAIInvocationsTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	if err := AutoMigrateProductListingAPIRuntimeSchema(db); err != nil {
		t.Fatalf("AutoMigrateProductListingAPIRuntimeSchema() error = %v", err)
	}
	if !db.Migrator().HasTable("ai_invocations") {
		t.Fatal("expected ai_invocations table to be created")
	}
	if !db.Migrator().HasTable("ai_async_jobs") {
		t.Fatal("expected ai_async_jobs table to be created")
	}
}
