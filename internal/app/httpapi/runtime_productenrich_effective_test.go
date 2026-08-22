package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

func TestProductEnrichRuntimeStaticOnlyLegacyTextAndVisionUseEffectiveBoundRoutes(t *testing.T) {
	server := newProductEnrichEffectiveServer(t)
	manager := newProductEnrichEffectiveManager(t, server.URL)
	recorder := &runtimeInvocationRecorder{}
	deps, err := buildProductEnrichRuntimeDeps(logrus.New(), &config.Config{AICapability: config.AICapabilityConfig{
		ProductEnrichTextEnabled: true, ProductEnrichTextAllowedTenantIDs: []string{"tenant-active"},
		ProductEnrichVisionEnabled: true, ProductEnrichVisionAllowedTenantIDs: []string{"tenant-active"},
	}}, manager, nil, recorder)
	if err != nil {
		t.Fatalf("buildProductEnrichRuntimeDeps: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	if _, err := deps.understanding.ExtractTextAttributes(ctx, "legacy text"); err != nil {
		t.Fatalf("ExtractTextAttributes: %v", err)
	}
	if _, err := deps.imageAnalyzer.AnalyzeImage(ctx, "https://example.test/image.jpg", "analyze product image"); err != nil {
		t.Fatalf("AnalyzeImage: %v", err)
	}
	if len(recorder.records) != 2 {
		t.Fatalf("records = %d, want 2", len(recorder.records))
	}
	for index, want := range []struct{ ref, model string }{{"fast", "fast-model"}, {"vision", "vision-model"}} {
		record := recorder.records[index]
		if record.RouteMode != aicapability.RoutingModeLegacy || record.CredentialReference != want.ref || record.ModelID != want.model || record.ConfigurationVersion == "" || record.Outcome != aicapability.InvocationSucceeded {
			t.Fatalf("legacy record[%d] = %+v", index, record)
		}
	}
}

func TestProductEnrichMockRuntimeExercisesAllowlistedActiveCapabilities(t *testing.T) {
	server := newProductEnrichEffectiveServer(t)
	manager := newProductEnrichEffectiveManager(t, server.URL)
	recorder := &runtimeInvocationRecorder{}
	deps, err := buildProductEnrichRuntimeDeps(logrus.New(), &config.Config{
		Debug: config.DebugConfig{ProductEnrichMockLLM: true},
		AICapability: config.AICapabilityConfig{
			ProductEnrichTextEnabled: true, ProductEnrichTextAllowedTenantIDs: []string{"tenant-active"},
			ProductEnrichVisionEnabled: true, ProductEnrichVisionAllowedTenantIDs: []string{"tenant-active"},
			ProductEnrichListingEnabled: true, ProductEnrichListingAllowedTenantIDs: []string{"tenant-active"},
		},
	}, manager, nil, recorder)
	if err != nil {
		t.Fatalf("buildProductEnrichRuntimeDeps: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-active", UserID: "user-a"})
	invocations := []struct {
		name string
		call func() error
	}{
		{name: "text", call: func() error { _, err := deps.understanding.ExtractTextAttributes(ctx, "active text"); return err }},
		{name: "vision", call: func() error {
			_, err := deps.imageAnalyzer.AnalyzeImage(ctx, "https://example.test/image.jpg", "analyze product image")
			return err
		}},
		{name: "listing", call: func() error {
			_, err := deps.contentGenerator.Generate(ctx, "Generate a complete product JSON")
			return err
		}},
		{name: "text scoring", call: func() error {
			_, err := deps.scoringTextGenerator.Generate(ctx, "请以 JSON 格式返回评分结果")
			return err
		}},
		{name: "vision scoring", call: func() error {
			_, err := deps.scoringImageAnalyzer.AnalyzeImage(ctx, "https://example.test/image.jpg", "产品图片质量评估专家")
			return err
		}},
	}
	for _, invocation := range invocations {
		t.Run(invocation.name, func(t *testing.T) {
			before := len(recorder.records)
			if err := invocation.call(); err != nil {
				t.Fatalf("active mock invocation: %v", err)
			}
			if len(recorder.records) != before+1 {
				t.Fatalf("records = %d, want %d", len(recorder.records), before+1)
			}
			record := recorder.records[before]
			if record.RouteMode != aicapability.RoutingModeActive || record.RouteOutcome != aicapability.RouteOutcomeActive || record.ProviderID == "" || record.ModelID == "" || record.CredentialReference == "" || record.ConfigurationVersion == "" || record.Outcome != aicapability.InvocationSucceeded {
				t.Fatalf("active mock record = %+v", record)
			}
		})
	}
}

func TestProductEnrichRuntimeDBOverrideKeepsPlanBoundClientCacheAndLedgerVersionAligned(t *testing.T) {
	server := newProductEnrichEffectiveServer(t)
	manager := newProductEnrichEffectiveManager(t, "https://static.test/v1")
	manager.SetConfigResolver(runtimeStaticProductEnrichResolver{config: &openaiclient.ClientConfig{
		APIKey: "db-secret", BaseURL: server.URL, Model: "db-model", APIStyle: "openai", Timeout: time.Second,
	}})
	recorder := &runtimeInvocationRecorder{}
	deps, err := buildProductEnrichRuntimeDeps(logrus.New(), &config.Config{AICapability: config.AICapabilityConfig{
		ProductEnrichTextEnabled: true, ProductEnrichTextAllowedTenantIDs: []string{"tenant-active"},
	}}, manager, nil, recorder)
	if err != nil {
		t.Fatalf("buildProductEnrichRuntimeDeps: %v", err)
	}
	preparer, ok := deps.scoringTextGenerator.(productenrich.TextExecutionPreparer)
	if !ok {
		t.Fatal("scoring generator does not expose governed preparation")
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-active", UserID: "user-a"})
	execution, err := preparer.PrepareText(ctx, "score this", productenrich.ScorePromptIdentity{PromptKey: "score", PromptVersion: "v1", PromptScope: "product_enrich"})
	if err != nil {
		t.Fatalf("PrepareText: %v", err)
	}
	cacheIdentity := execution.ScoreCacheIdentity("80", "input-hash")
	if _, err := execution.Invoke(ctx, aicapability.CacheStatusMiss); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("records = %d, want 1", len(recorder.records))
	}
	record := recorder.records[0]
	if record.ModelID != "db-model" || record.ConfigurationVersion == "" || record.ConfigurationVersion != cacheIdentity.ConfigurationVersion ||
		record.ProviderID != cacheIdentity.ProviderID || record.ModelID != cacheIdentity.ModelID || record.RoutingKey != cacheIdentity.RoutingKey || record.CredentialReference != "fast" {
		t.Fatalf("plan/cache/ledger route mismatch = %+v / %+v", cacheIdentity, record)
	}
}

func newProductEnrichEffectiveManager(t *testing.T, baseURL string) *openaiclient.Manager {
	t.Helper()
	manager, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients: map[string]*openaiclient.ClientConfig{
			"default": {APIKey: "default-secret", BaseURL: baseURL, Model: "default-model", APIStyle: "openai", Timeout: time.Second},
			"fast":    {APIKey: "fast-secret", BaseURL: baseURL, Model: "fast-model", APIStyle: "openai", Timeout: time.Second},
			"vision":  {APIKey: "vision-secret", BaseURL: baseURL, Model: "vision-model", APIStyle: "openai", Timeout: time.Second},
		},
		DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	return manager
}

func newProductEnrichEffectiveServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-effective-test", "object": "chat.completion", "created": 1, "model": "test-model",
			"choices": []map[string]any{{"index": 0, "message": map[string]string{"role": "assistant", "content": `{"product_type":"Mock Product","attributes":{"color":"Black"},"features":["feature"]}`}, "finish_reason": "stop"}},
		})
	}))
	t.Cleanup(server.Close)
	return server
}
