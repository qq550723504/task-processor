package httpapi

import (
	"context"
	"fmt"
	"strings"
	"sync"

	openaiclient "task-processor/internal/infra/clients/openai"
)

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

func (c *strictListingKitConfiguredImageClient) SupportsAsyncImageGeneration() bool {
	return false
}

func (c *strictListingKitConfiguredImageClient) SubmitImageGeneration(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.SubmitImageGeneration(ctx, req)
}

func (c *strictListingKitConfiguredImageClient) QueryImageGeneration(ctx context.Context, jobID string) (*openaiclient.ImageAsyncQueryResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.QueryImageGeneration(ctx, jobID)
}

func (c *strictListingKitConfiguredImageClient) resolve(ctx context.Context) (openaiclient.ImageGenerator, error) {
	return resolveStrictListingKitImageClient(ctx, c.clientName, c.resolver, c.fallback, &c.mu, c.cache, c.build)
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
