package httpapi

import (
	"context"
	"fmt"
	"strings"
	"sync"

	openaiclient "task-processor/internal/infra/clients/openai"
)

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
