package httpapi

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
)

type recordingScorerClient struct{}

func (recordingScorerClient) Generate(_ context.Context, _ string) (string, error) {
	return `{"score": 80}`, nil
}

func (recordingScorerClient) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	return `{"score": 82}`, nil
}

type recordingScorerManager struct {
	requested []string
}

func (m *recordingScorerManager) GetClient(name string) (productenrich.LLMClient, error) {
	m.requested = append(m.requested, name)
	return recordingScorerClient{}, nil
}

func (m *recordingScorerManager) GetDefaultClient() productenrich.LLMClient {
	return recordingScorerClient{}
}

func TestBuildProductLLMScorerConfig_PrefersScorerClientWhenConfigured(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"scorer": {
			Model: "gemini-2.5-flash",
		},
	}

	scorerCfg := buildProductLLMScorerConfig(cfg, nil)
	if scorerCfg.TextClient != productScorerClientName {
		t.Fatalf("TextClient = %q, want %q", scorerCfg.TextClient, productScorerClientName)
	}
	if scorerCfg.VisionClient != productScorerClientName {
		t.Fatalf("VisionClient = %q, want %q", scorerCfg.VisionClient, productScorerClientName)
	}
}

func TestBuildProductLLMScorerConfig_PreservesDefaultFallbackWhenScorerClientMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"fast": {
			Model: "gemini-2.5-flash-lite",
		},
		"vision": {
			Model: "gemini-2.5-flash",
		},
	}

	scorerCfg := buildProductLLMScorerConfig(cfg, nil)
	if scorerCfg.TextClient != "" {
		t.Fatalf("TextClient = %q, want empty for default fallback", scorerCfg.TextClient)
	}
	if scorerCfg.VisionClient != "" {
		t.Fatalf("VisionClient = %q, want empty for default fallback", scorerCfg.VisionClient)
	}
}

func TestBuildProductLLMScorer_UsesScorerClientAtRuntimeWhenConfigured(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"scorer": {
			Model: "gemini-2.5-flash",
		},
	}
	manager := &recordingScorerManager{}
	scorer := buildProductLLMScorer(cfg, manager)

	if _, err := scorer.ScoreText(context.Background(), "sample", 50); err != nil {
		t.Fatalf("ScoreText() error = %v", err)
	}
	if _, err := scorer.ScoreImage(context.Background(), "https://example.com/a.jpg", 60); err != nil {
		t.Fatalf("ScoreImage() error = %v", err)
	}

	if len(manager.requested) != 2 {
		t.Fatalf("requested clients = %v, want 2 calls", manager.requested)
	}
	if manager.requested[0] != productScorerClientName {
		t.Fatalf("text client = %q, want %q", manager.requested[0], productScorerClientName)
	}
	if manager.requested[1] != productScorerClientName {
		t.Fatalf("image client = %q, want %q", manager.requested[1], productScorerClientName)
	}
}
