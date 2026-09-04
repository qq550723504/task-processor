package httpapi

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"task-processor/internal/ai"
	"task-processor/internal/core/config"
	corelogger "task-processor/internal/core/logger"
	openaiclient "task-processor/internal/integration/openai"
	"task-processor/internal/listingkit"
)

const listingKitSheinSaleAttributeClientName = "scorer"

type strictListingKitChatClient struct {
	clientName string
	resolver   openaiclient.ClientConfigResolver
	fallback   *openaiclient.ClientConfig
	mu         sync.Mutex
	cache      map[string]*openaiclient.Client
}

func buildStrictListingKitChatClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) ai.TextChatCompleter {
	return &strictListingKitChatClient{
		clientName: clientName,
		resolver:   resolver,
		fallback:   buildListingKitClientFallback(cfg, clientName),
		cache:      make(map[string]*openaiclient.Client),
	}
}

func (c *strictListingKitChatClient) CreateChatCompletion(ctx context.Context, req *ai.ChatCompletionRequest) (*ai.ChatCompletionResponse, error) {
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

func (c *strictListingKitChatClient) GetDefaultModel() string { return "" }

func (c *strictListingKitChatClient) resolve(ctx context.Context) (*openaiclient.Client, error) {
	return resolveStrictListingKitClient(ctx, c.clientName, c.resolver, c.fallback, &c.mu, c.cache)
}

func resolveStrictListingKitClient(ctx context.Context, clientName string, resolver openaiclient.ClientConfigResolver, fallback *openaiclient.ClientConfig, mu *sync.Mutex, cache map[string]*openaiclient.Client) (*openaiclient.Client, error) {
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
	clientConfig := *resolved.Config
	clientConfig.Logger = openaiclient.AdaptLogrus(corelogger.GetGlobalLogger("listingkit/chat-provider"))
	if strings.TrimSpace(clientConfig.APIKey) == "" || strings.TrimSpace(clientConfig.BaseURL) == "" || strings.TrimSpace(clientConfig.Model) == "" {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	cacheKey := strings.TrimSpace(resolved.CacheKey)
	if cacheKey == "" {
		cacheKey = fmt.Sprintf("%s:%s:%s:%s", normalizeListingKitClientName(clientName), clientConfig.APIKey, clientConfig.BaseURL, clientConfig.Model)
	}
	mu.Lock()
	defer mu.Unlock()
	if client := cache[cacheKey]; client != nil {
		return client, nil
	}
	client := openaiclient.NewClient(&clientConfig)
	if client == nil {
		return nil, fmt.Errorf("create listingkit ai client %q: failed to initialize", normalizeListingKitClientName(clientName))
	}
	cache[cacheKey] = client
	return client, nil
}

func buildListingKitClientFallback(cfg *config.Config, clientName string) *openaiclient.ClientConfig {
	if cfg == nil {
		return nil
	}
	base := cfg.OpenAI.ToClientConfig()
	if named, ok := cfg.OpenAI.ToClientConfigs()[normalizeListingKitClientName(clientName)]; ok && named != nil {
		base = named
	}
	return sanitizeListingKitClientFallback(base)
}

func sanitizeListingKitClientFallback(cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	cloned.APIKey = ""
	cloned.BaseURL = ""
	cloned.Model = ""
	return &cloned
}

func normalizeListingKitClientName(name string) string {
	if name = strings.TrimSpace(name); name == "" {
		return "default"
	}
	return name
}

func errListingKitAIClientNotConfigured(clientName string) error {
	return fmt.Errorf("listingkit ai client %q is not configured for current tenant/user", normalizeListingKitClientName(clientName))
}

type listingKitAICredentialStore struct {
	store *openaiclient.GormCredentialResolver
}

func adaptListingKitAICredentialStore(store *openaiclient.GormCredentialResolver) listingkit.AIClientCredentialStore {
	if store == nil {
		return nil
	}
	return listingKitAICredentialStore{store: store}
}

func (s listingKitAICredentialStore) SaveCredential(ctx context.Context, credential listingkit.AIClientCredential) error {
	return s.store.SaveCredential(ctx, openaiclient.AIClientCredential{
		TenantID: credential.TenantID, UserID: credential.UserID, ClientName: credential.ClientName,
		APIKey: credential.APIKey, BaseURL: credential.BaseURL, Model: credential.Model,
		APIStyle: credential.APIStyle, TimeoutSecond: credential.TimeoutSecond,
		Enabled: credential.Enabled, UpdatedAt: credential.UpdatedAt,
	})
}

func (s listingKitAICredentialStore) GetCredential(ctx context.Context, tenantID, userID, clientName string) (*listingkit.AIClientCredential, error) {
	credential, err := s.store.GetCredential(ctx, tenantID, userID, clientName)
	if err != nil || credential == nil {
		return nil, err
	}
	return &listingkit.AIClientCredential{
		TenantID: credential.TenantID, UserID: credential.UserID, ClientName: credential.ClientName,
		APIKey: credential.APIKey, BaseURL: credential.BaseURL, Model: credential.Model,
		APIStyle: credential.APIStyle, TimeoutSecond: credential.TimeoutSecond,
		Enabled: credential.Enabled, UpdatedAt: credential.UpdatedAt,
	}, nil
}
