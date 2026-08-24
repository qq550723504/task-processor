package httpapi

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	productenrich "task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
)

type stubLLMClient struct{}

func (stubLLMClient) Generate(_ context.Context, _ string) (string, error) {
	return `{"needs_review":false,"confidence":0.9}`, nil
}

func (stubLLMClient) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	return `{"needs_review":false,"confidence":0.9}`, nil
}

type stubLLMManager struct{}

func (stubLLMManager) GetClient(_ string) (productenrich.LLMClient, error) {
	return stubLLMClient{}, nil
}

func (stubLLMManager) GetDefaultClient() productenrich.LLMClient {
	return stubLLMClient{}
}

func TestBuildModelProviderBuildsConfiguredCapabilities(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenAI.APIKey = "test-key"
	cfg.OpenAI.Model = "gpt-5.1"
	cfg.OpenAI.BaseURL = "http://example.com/v1"
	cfg.OpenAI.Timeout = 30
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"image": {
			APIKey:  "test-key",
			Model:   "nanobanana",
			BaseURL: "http://example.com/v1",
			Timeout: 30,
		},
	}

	openaiMgr, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients:       cfg.OpenAI.ToClientConfigs(),
		DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	provider, err := buildModelProvider(newModelProviderOptions(cfg), stubLLMManager{}, openaiMgr, t.TempDir())
	if err != nil {
		t.Fatalf("buildModelProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("provider = nil")
	}
	if provider.FaithfulEditor() == nil {
		t.Fatal("FaithfulEditor() = nil")
	}
	if provider.ReviewModel() == nil {
		t.Fatal("ReviewModel() = nil")
	}
	if provider.SceneGenerator() == nil {
		t.Fatal("SceneGenerator() = nil")
	}
	if !shouldUseModelBackedImagePipeline(provider) {
		t.Fatal("expected model-backed image pipeline to be enabled")
	}
}

func TestBuildModelProviderBuildsNanobananaCapabilities(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenAI.APIKey = "test-key"
	cfg.OpenAI.Model = "gpt-5.1"
	cfg.OpenAI.BaseURL = "http://example.com/v1"
	cfg.OpenAI.Timeout = 30
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"image": {
			APIKey:   "test-key",
			Model:    "nano-banana-fast",
			BaseURL:  "https://grsai.dakka.com.cn/v1/draw/nano-banana",
			Timeout:  30,
			APIStyle: "nanobanana",
		},
	}

	openaiMgr, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients:       cfg.OpenAI.ToClientConfigs(),
		DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	provider, err := buildModelProvider(newModelProviderOptions(cfg), stubLLMManager{}, openaiMgr, t.TempDir())
	if err != nil {
		t.Fatalf("buildModelProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("provider = nil")
	}
	if provider.FaithfulEditor() == nil {
		t.Fatal("FaithfulEditor() = nil")
	}
	if provider.SceneGenerator() == nil {
		t.Fatal("SceneGenerator() = nil")
	}
}

func TestBuildModelProviderRejectsGovernanceWithoutResolverBackedGPTImage(t *testing.T) {
	cfg := &config.Config{}
	cfg.AICapability.ProductImageSceneEnabled = true
	cfg.OpenAI.APIKey = "test-key"
	cfg.OpenAI.Model = "gpt-5.1"
	cfg.OpenAI.BaseURL = "http://example.com/v1"
	cfg.OpenAI.Timeout = 30
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"image": {
			APIKey:   "test-key",
			Model:    "nano-banana-fast",
			BaseURL:  "https://grsai.dakka.com.cn/v1/draw/nano-banana",
			Timeout:  30,
			APIStyle: "nanobanana",
		},
	}

	openaiMgr, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients:       cfg.OpenAI.ToClientConfigs(),
		DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if _, err := buildModelProvider(newModelProviderOptions(cfg), stubLLMManager{}, openaiMgr, t.TempDir()); err == nil {
		t.Fatal("buildModelProvider() error = nil, want resolver-backed client rejection")
	}
}

func TestBuildModelProviderUsesResolverBackedGPTImageWhenGovernanceEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.AICapability.ProductImageSceneEnabled = true
	cfg.OpenAI.APIKey = "test-key"
	cfg.OpenAI.Model = "gpt-5.1"
	cfg.OpenAI.BaseURL = "http://example.com/v1"
	cfg.OpenAI.Timeout = 30
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"image": {
			APIKey:   "test-key",
			Model:    "nano-banana-fast",
			BaseURL:  "https://grsai.dakka.com.cn/v1/draw/nano-banana",
			Timeout:  30,
			APIStyle: "nanobanana",
		},
	}

	openaiMgr, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients:       cfg.OpenAI.ToClientConfigs(),
		DefaultClient: "default",
		ConfigResolver: staticImageConfigResolver{config: &openaiclient.ClientConfig{
			APIKey:   "tenant-key",
			Model:    "gpt-image-2",
			BaseURL:  "https://image.example.test/v1",
			APIStyle: "openai",
		}},
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	provider, err := buildModelProvider(newModelProviderOptions(cfg), stubLLMManager{}, openaiMgr, t.TempDir())
	if err != nil {
		t.Fatalf("buildModelProvider() error = %v", err)
	}
	if provider == nil || provider.SceneGenerator() == nil || provider.FaithfulEditor() == nil {
		t.Fatalf("provider = %#v, want resolver-backed model components", provider)
	}
}

type staticImageConfigResolver struct {
	config *openaiclient.ClientConfig
}

func (r staticImageConfigResolver) ResolveClientConfig(context.Context, string, *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	return &openaiclient.ResolvedClientConfig{CacheKey: "image-config-v1", Config: r.config}, nil
}

func TestResolveImagePipelineComponentsBackfillsModelBackedDependencies(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenAI.APIKey = "test-key"
	cfg.OpenAI.Model = "gpt-5.1"
	cfg.OpenAI.BaseURL = "http://example.com/v1"
	cfg.OpenAI.Timeout = 30
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"image": {
			APIKey:   "test-key",
			Model:    "nano-banana-fast",
			BaseURL:  "https://grsai.dakka.com.cn/v1/draw/nano-banana",
			Timeout:  30,
			APIStyle: "nanobanana",
		},
	}

	openaiMgr, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients:       cfg.OpenAI.ToClientConfigs(),
		DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	provider, err := buildModelProvider(newModelProviderOptions(cfg), stubLLMManager{}, openaiMgr, t.TempDir())
	if err != nil {
		t.Fatalf("buildModelProvider() error = %v", err)
	}

	resolved := resolveImagePipelineComponents(provider, nil, nil, nil)
	if resolved.subjectExtractor == nil {
		t.Fatal("subjectExtractor = nil")
	}
	if resolved.whiteBgRenderer == nil {
		t.Fatal("whiteBgRenderer = nil")
	}
	if resolved.sceneRenderer == nil {
		t.Fatal("sceneRenderer = nil")
	}
	if _, ok := resolved.subjectExtractor.(productimage.SubjectExtractor); !ok {
		t.Fatal("subjectExtractor does not implement productimage.SubjectExtractor")
	}
}
