package httpapi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
	nanobanana "task-processor/internal/infra/clients/nanobanana"
	openaiclient "task-processor/internal/infra/clients/openai"
)

const (
	listingKitImageClientName             = "image"
	listingKitImageClientNameGPTImage2    = "image_gpt_image_2"
	listingKitImageClientNameNanobanana   = "image_nanobanana"
	listingKitImageModelSelectorGPTImage2 = "gpt-image-2"
	listingKitImageModelSelectorNano      = "nanobanana"
	sheinSaleAttributeClientName          = "scorer"
	listingKitStudioImageMinTimeout       = 300 * time.Second
)

func BuildSheinCategoryLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
	return buildStrictListingKitChatClient(cfg, resolver, "default")
}

func BuildSheinSaleAttributeLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
	return buildStrictListingKitChatClient(cfg, resolver, sheinSaleAttributeClientName)
}

func BuildStudioImageGenerator(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {
	return buildListingKitRoutedImageClient(cfg, resolver)
}

func buildStrictListingKitChatClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) openaiclient.ChatCompleter {
	return &strictListingKitChatClient{
		clientName: clientName,
		resolver:   resolver,
		fallback:   buildListingKitClientFallback(cfg, clientName),
		cache:      make(map[string]*openaiclient.Client),
	}
}

func buildStrictListingKitImageClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) openaiclient.ImageGenerator {
	return &strictListingKitConfiguredImageClient{
		clientName: clientName,
		resolver:   resolver,
		fallback:   buildListingKitClientFallback(cfg, clientName),
		cache:      make(map[string]openaiclient.ImageGenerator),
		build: func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error) {
			client := openaiclient.NewClient(cfg)
			if client == nil {
				return nil, fmt.Errorf("failed to initialize openai image client")
			}
			return client, nil
		},
	}
}

func buildStrictListingKitNanobananaImageClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) openaiclient.ImageGenerator {
	return &strictListingKitConfiguredImageClient{
		clientName: clientName,
		resolver:   resolver,
		fallback:   buildListingKitClientFallback(cfg, clientName),
		cache:      make(map[string]openaiclient.ImageGenerator),
		build: func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error) {
			return nanobanana.NewClient(nanobanana.Config{
				APIKey:       cfg.APIKey,
				Model:        cfg.Model,
				SubmitURL:    cfg.BaseURL,
				PollInterval: time.Second,
				Timeout:      cfg.Timeout,
			}), nil
		},
	}
}

func buildListingKitRoutedImageClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {
	nanoClient := buildStrictListingKitNanobananaImageClient(cfg, resolver, listingKitImageClientNameNanobanana)
	gptClient := buildStrictListingKitImageClient(cfg, resolver, listingKitImageClientNameGPTImage2)
	defaultClient := nanoClient
	if resolver == nil {
		defaultClient = buildStrictListingKitImageClient(cfg, resolver, listingKitImageClientName)
	}
	return &listingKitRoutedImageClient{
		defaultModel: listingKitImageModelSelectorGPTImage2,
		defaultImage: defaultClient,
		gptImage2:    gptClient,
		nanobanana:   nanoClient,
	}
}

func errListingKitAIClientNotConfigured(clientName string) error {
	return fmt.Errorf("listingkit ai client %q is not configured for current tenant/user", normalizeListingKitClientName(clientName))
}

type strictListingKitChatClient struct {
	clientName string
	resolver   openaiclient.ClientConfigResolver
	fallback   *openaiclient.ClientConfig
	mu         sync.Mutex
	cache      map[string]*openaiclient.Client
}

func (c *strictListingKitChatClient) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.CreateChatCompletion(ctx, req)
}

func (c *strictListingKitChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return "", err
	}
	return client.Generate(ctx, prompt)
}

func (c *strictListingKitChatClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return "", err
	}
	return client.AnalyzeImage(ctx, imageURL, prompt)
}

func (c *strictListingKitChatClient) GetDefaultModel() string {
	return ""
}

func (c *strictListingKitChatClient) resolve(ctx context.Context) (*openaiclient.Client, error) {
	return resolveStrictListingKitClient(ctx, c.clientName, c.resolver, c.fallback, &c.mu, c.cache)
}

type strictListingKitConfiguredImageClient struct {
	clientName string
	resolver   openaiclient.ClientConfigResolver
	fallback   *openaiclient.ClientConfig
	mu         sync.Mutex
	cache      map[string]openaiclient.ImageGenerator
	build      func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error)
}

func (c *strictListingKitConfiguredImageClient) GenerateImage(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.GenerateImage(ctx, req)
}

func (c *strictListingKitConfiguredImageClient) EditImage(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.EditImage(ctx, req)
}

func (c *strictListingKitConfiguredImageClient) GetDefaultModel() string {
	return ""
}

func (c *strictListingKitConfiguredImageClient) resolve(ctx context.Context) (openaiclient.ImageGenerator, error) {
	return resolveStrictListingKitImageClient(ctx, c.clientName, c.resolver, c.fallback, &c.mu, c.cache, c.build)
}

func resolveStrictListingKitClient(
	ctx context.Context,
	clientName string,
	resolver openaiclient.ClientConfigResolver,
	fallback *openaiclient.ClientConfig,
	mu *sync.Mutex,
	cache map[string]*openaiclient.Client,
) (*openaiclient.Client, error) {
	if resolver == nil {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	resolved, err := resolver.ResolveClientConfig(ctx, clientName, fallback)
	if err != nil {
		return nil, err
	}
	if resolved == nil || resolved.Config == nil {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	config := resolved.Config
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.Model) == "" {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	cacheKey := strings.TrimSpace(resolved.CacheKey)
	if cacheKey == "" {
		cacheKey = fmt.Sprintf("%s:%s:%s:%s", normalizeListingKitClientName(clientName), config.APIKey, config.BaseURL, config.Model)
	}
	mu.Lock()
	defer mu.Unlock()
	if client := cache[cacheKey]; client != nil {
		return client, nil
	}
	client := openaiclient.NewClient(config)
	if client == nil {
		return nil, fmt.Errorf("create listingkit ai client %q: failed to initialize", normalizeListingKitClientName(clientName))
	}
	cache[cacheKey] = client
	return client, nil
}

func resolveStrictListingKitImageClient(
	ctx context.Context,
	clientName string,
	resolver openaiclient.ClientConfigResolver,
	fallback *openaiclient.ClientConfig,
	mu *sync.Mutex,
	cache map[string]openaiclient.ImageGenerator,
	build func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error),
) (openaiclient.ImageGenerator, error) {
	if resolver == nil {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	resolved, err := resolver.ResolveClientConfig(ctx, clientName, fallback)
	if err != nil {
		return nil, err
	}
	if resolved == nil || resolved.Config == nil {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	config := enforceListingKitImageClientTimeout(normalizeListingKitClientName(clientName), resolved.Config)
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.Model) == "" {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	cacheKey := strings.TrimSpace(resolved.CacheKey)
	if cacheKey == "" {
		cacheKey = fmt.Sprintf("%s:%s:%s:%s", normalizeListingKitClientName(clientName), config.APIKey, config.BaseURL, config.Model)
	}
	mu.Lock()
	defer mu.Unlock()
	if client := cache[cacheKey]; client != nil {
		return client, nil
	}
	client, err := build(config)
	if err != nil {
		return nil, fmt.Errorf("create listingkit ai client %q: %w", normalizeListingKitClientName(clientName), err)
	}
	cache[cacheKey] = client
	return client, nil
}
