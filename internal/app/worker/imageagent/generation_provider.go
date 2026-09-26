package imageagentworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/ai"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/tools"
	"task-processor/internal/integration/grsai"
	openai "task-processor/internal/integration/openai"
	productimage "task-processor/internal/product/image"
)

// This factory binds the current synchronous GRSAI consumer explicitly. It
// never asks the OpenAI manager to guess a provider from a compatible URL.
type generationProviderFactory struct {
	resolver openai.ClientConfigResolver
	price    config.ImageAgentGenerationConfig
	profile  ProfileResolver
	logger   *logrus.Entry
	// Explicit in-process test transports only; production leaves these nil.
	providerHTTPClient *http.Client
	generatedFetcher   func(context.Context, string) ([]byte, error)
}

func (f generationProviderFactory) prepare(ctx context.Context, observe func(context.Context, imageagent.GenerationSuccess) error) (imageagent.PreparedGenerationProvider, error) {
	if observe == nil || f.profile == nil {
		return imageagent.PreparedGenerationProvider{}, imageagent.ErrBudgetQuoteUnavailable
	}
	cfg, metadata, err := f.resolve(ctx)
	if err != nil {
		return imageagent.PreparedGenerationProvider{}, err
	}
	client := grsai.NewClient(grsai.Config{APIKey: cfg.APIKey, Model: cfg.Model, SubmitURL: cfg.BaseURL, Timeout: cfg.Timeout, HTTPClient: f.providerHTTPClient, MaxAttempts: 1, Logger: grsai.AdaptLogrus(f.logger)})
	provider, err := grsai.NewSynchronousProductImageAdapter(grsai.ProductImageAdapterConfig{
		Client: client, ImageModel: cfg.Model, RouteReference: metadata.RouteReference, CredentialReference: metadata.CredentialReference, ConfigurationVersion: metadata.ConfigurationVersion,
		Build: func(bound ai.RouteBoundImageGenerator) (grsai.ProductImageProvider, error) {
			return openai.NewProductImageAdapter(openai.ProductImageAdapterConfig{ImageClient: bound, Provider: "grsai", ImageModel: cfg.Model, RouteReference: metadata.RouteReference, CredentialReference: metadata.CredentialReference, ConfigurationVersion: metadata.ConfigurationVersion, Prompts: openai.DefaultProductImagePrompts(), PricingVersion: metadata.PriceVersion, MaximumSceneOutputs: 1, GeneratedImageFetcher: f.generatedFetcher})
		},
	}, func(ctx context.Context, observation grsai.GenerationObservation) error {
		return observe(ctx, imageagent.GenerationSuccess{ResponseID: observation.ResponseID, RequestID: observation.RequestID, ResultDigest: observation.ResultDigest, UsageKnown: observation.UsageKnown, InputTokens: int64(observation.Usage.PromptTokens), OutputTokens: int64(observation.Usage.CompletionTokens), TotalTokens: int64(observation.Usage.TotalTokens)})
	})
	if err != nil {
		return imageagent.PreparedGenerationProvider{}, imageagent.ErrBudgetQuoteUnavailable
	}
	white, err := productimage.NewWhiteBackgroundCapability(provider)
	if err != nil {
		return imageagent.PreparedGenerationProvider{}, err
	}
	executor := tools.NewProductImageSlotExecutor(tools.Dependencies{WhiteBackgroundRenderer: white, UsageQuoter: provider, ProfileResolver: f.profile})
	return imageagent.PreparedGenerationProvider{Metadata: metadata, Executor: executor}, nil
}

func (f generationProviderFactory) revalidate(ctx context.Context, expected imageagent.GenerationProviderMetadata) error {
	_, current, err := f.resolve(ctx)
	if err != nil {
		return err
	}
	if current != expected {
		return imageagent.ErrRevisionConflict
	}
	return nil
}

func (f generationProviderFactory) resolve(ctx context.Context) (*openai.ClientConfig, imageagent.GenerationProviderMetadata, error) {
	invalid := func() (*openai.ClientConfig, imageagent.GenerationProviderMetadata, error) {
		return nil, imageagent.GenerationProviderMetadata{}, imageagent.ErrBudgetQuoteUnavailable
	}
	if f.resolver == nil || !f.price.Configured() {
		return invalid()
	}
	// No process-wide key/model fallback: the organization credential owner
	// must return an enabled, scoped configuration of the selected provider.
	resolved, err := f.resolver.ResolveClientConfig(ctx, imageAgentOpenAIClientName, nil)
	if err != nil || resolved == nil || resolved.Config == nil || resolved.CacheKey == "" || len(resolved.CacheKey) > 192 {
		return invalid()
	}
	cfg := *resolved.Config
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || cfg.APIStyle != "grsai" || cfg.Model != "gpt-image-2.5" || cfg.APIKey == "" || cfg.Timeout <= 0 || cfg.Timeout > 5*time.Minute {
		return invalid()
	}
	switch strings.TrimSuffix(u.Path, "/") {
	case "", "/v1", "/v1/api/generate":
	default:
		return invalid()
	}
	prompts := openai.DefaultProductImagePrompts()
	metadata := imageagent.GenerationProviderMetadata{RouteReference: "grsai-sync:" + generationConfigDigest(cfg.BaseURL), CredentialReference: resolved.CacheKey, ConfigurationVersion: generationConfigDigest(struct {
		Version, Model, Style, URL string
		Timeout                    time.Duration
	}{resolved.CacheKey, cfg.Model, cfg.APIStyle, cfg.BaseURL, cfg.Timeout}), PromptVersion: prompts.Version, PriceVersion: f.price.PriceVersion, Points: f.price.PointsPerImage}
	return &cfg, metadata, nil
}

func generationConfigDigest(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
