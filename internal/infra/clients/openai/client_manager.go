// Package openai 提供OpenAI客户端管理器
package openai

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// Manager OpenAI客户端管理器
type Manager struct {
	clients        map[string]*Client
	dynamicClients map[string]*Client
	configResolver ClientConfigResolver
	defaultClient  *Client
	logger         *logrus.Entry
	mu             sync.RWMutex
}

type ResolvedClientConfig struct {
	CacheKey string
	Config   *ClientConfig
}

type ClientConfigResolver interface {
	ResolveClientConfig(ctx context.Context, clientName string, fallback *ClientConfig) (*ResolvedClientConfig, error)
}

// EffectiveClientRoute is the non-secret execution identity selected by the
// manager after applying tenant/user overrides and the registered static
// fallback. It is safe to use in routing, cache, and invocation metadata.
type EffectiveClientRoute struct {
	ProviderID           string
	ModelID              string
	CredentialReference  string
	ConfigurationVersion string
}

type EffectiveClientRouteResolver interface {
	ResolveEffectiveClientRoute(ctx context.Context, clientName string) (EffectiveClientRoute, error)
}

var (
	ErrClientConfigurationUnavailable = errors.New("client configuration is unavailable")
	ErrClientConfigurationChanged     = errors.New("client configuration changed")
	ErrClientConfigurationUnsupported = errors.New("client configuration provider is unsupported")
)

// ManagerConfig 管理器配置
type ManagerConfig struct {
	Clients        map[string]*ClientConfig
	ConfigResolver ClientConfigResolver
	DefaultClient  string
}

// NewManager 创建OpenAI客户端管理器
func NewManager(config *ManagerConfig) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if len(config.Clients) == 0 {
		return nil, fmt.Errorf("at least one client must be configured")
	}

	manager := &Manager{
		clients:        make(map[string]*Client),
		dynamicClients: make(map[string]*Client),
		configResolver: config.ConfigResolver,
		logger:         logger.GetGlobalLogger("OpenAIManager"),
	}

	// 创建所有配置的客户端
	for name, clientConfig := range config.Clients {
		client := NewClient(clientConfig)
		if client == nil {
			return nil, fmt.Errorf("failed to create client: %s", name)
		}
		manager.clients[name] = client
		manager.logger.Infof("OpenAI客户端已注册: %s", name)
	}

	// 设置默认客户端
	if config.DefaultClient != "" {
		defaultClient, exists := manager.clients[config.DefaultClient]
		if !exists {
			return nil, fmt.Errorf("default client not found")
		}
		manager.defaultClient = defaultClient
	} else {
		for _, client := range manager.clients {
			manager.defaultClient = client
			break
		}
	}

	return manager, nil
}

// GetClient 获取指定名称的客户端
func (m *Manager) GetClient(name string) (ChatCompleter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.clients[name]; !exists {
		return nil, fmt.Errorf("client %s not found", name)
	}
	return &contextualChatClient{manager: m, name: name}, nil
}

// GetClientWithRoute returns the concrete client for the exact effective
// configuration selected by the caller. Resolution and version validation
// happen before the provider client is returned, so later rotation cannot run
// under stale route metadata.
func (m *Manager) GetClientWithRoute(ctx context.Context, name string, selection ImageRouteSelection) (ChatCompleter, error) {
	return m.resolveClientWithSelection(ctx, name, &selection)
}

func (m *Manager) GetImageClient(name string) (ImageGenerator, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.clients[name]; !exists && m.configResolver == nil {
		return nil, fmt.Errorf("client %s not found", name)
	}
	return &contextualImageClient{manager: m, name: name}, nil
}

// GetDefaultClient 获取默认客户端
func (m *Manager) GetDefaultClient() ChatCompleter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &contextualChatClient{manager: m, name: m.defaultClientNameLocked()}
}

func (m *Manager) SetConfigResolver(resolver ClientConfigResolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configResolver = resolver
	m.dynamicClients = make(map[string]*Client)
}

// HasConfigResolver reports whether tenant-aware client configuration is wired.
// Governance-enabled callers use this to fail closed instead of silently using
// the manager's static API key and endpoint.
func (m *Manager) HasConfigResolver() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configResolver != nil
}

// RegisterClient 注册新的客户端
func (m *Manager) RegisterClient(name string, client *Client) error {
	if name == "" || client == nil {
		return fmt.Errorf("invalid parameters")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.clients[name]; exists {
		return fmt.Errorf("client already exists")
	}
	m.clients[name] = client
	m.logger.Infof("客户端已注册: %s", name)
	return nil
}

func (m *Manager) defaultClientNameLocked() string {
	for name, client := range m.clients {
		if client == m.defaultClient {
			return name
		}
	}
	return "default"
}

func (m *Manager) resolveClient(ctx context.Context, name string) (*Client, error) {
	return m.resolveClientWithSelection(ctx, name, nil)
}

func (m *Manager) resolveClientWithSelection(ctx context.Context, name string, selection *ImageRouteSelection) (*Client, error) {
	if selection != nil {
		if reference := normalizeClientName(selection.CredentialReference); reference != normalizeClientName(name) {
			return nil, fmt.Errorf("%w: route credential reference %q does not match client %q", ErrClientConfigurationChanged, selection.CredentialReference, name)
		}
	}
	resolved, err := m.resolveEffectiveClientConfiguration(ctx, name)
	if err != nil {
		return nil, err
	}
	selectedVersion := strings.TrimSpace(selectionVersion(selection))
	if selection != nil {
		matchesEffectiveVersion := selectedVersion != "" && selectedVersion == resolved.route.ConfigurationVersion
		matchesLegacyResolverVersion := resolved.resolverVersion != "" && selectedVersion == resolved.resolverVersion
		if !matchesEffectiveVersion && !matchesLegacyResolverVersion {
			return nil, fmt.Errorf("%w: selected version does not match current effective version", ErrClientConfigurationChanged)
		}
	}
	if resolved.staticClient != nil {
		return resolved.staticClient, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if client := m.dynamicClients[resolved.route.ConfigurationVersion]; client != nil {
		return client, nil
	}
	client := NewClient(resolved.config)
	if client == nil {
		return nil, fmt.Errorf("failed to create resolved client: %s", name)
	}
	m.dynamicClients[resolved.route.ConfigurationVersion] = client
	return client, nil
}

type effectiveClientConfiguration struct {
	config       *ClientConfig
	staticClient *Client
	route        EffectiveClientRoute
	// resolverVersion is retained only as a compatibility binding for existing
	// provider consumers that still receive the resolver's non-secret version.
	// New manager-authority callers use route.ConfigurationVersion.
	resolverVersion string
}

// ResolveEffectiveClientRoute is the single manager-owned authority for route
// metadata. It applies the same resolver/static precedence as execution.
func (m *Manager) ResolveEffectiveClientRoute(ctx context.Context, name string) (EffectiveClientRoute, error) {
	resolved, err := m.resolveEffectiveClientConfiguration(ctx, name)
	if err != nil {
		return EffectiveClientRoute{}, err
	}
	return resolved.route, nil
}

func (m *Manager) resolveEffectiveClientConfiguration(ctx context.Context, name string) (effectiveClientConfiguration, error) {
	if m == nil {
		return effectiveClientConfiguration{}, ErrClientConfigurationUnavailable
	}
	name = normalizeClientName(name)
	m.mu.RLock()
	staticClient := m.clients[name]
	resolver := m.configResolver
	m.mu.RUnlock()

	var staticConfig *ClientConfig
	if staticClient != nil {
		staticConfig = cloneClientConfig(staticClient.config)
	}
	effectiveConfig := staticConfig
	credentialVersion := "static:" + name
	usesStatic := staticConfig != nil
	if resolver != nil {
		resolved, err := resolver.ResolveClientConfig(ctx, name, cloneClientConfig(staticConfig))
		if err != nil {
			return effectiveClientConfiguration{}, err
		}
		if resolved != nil && resolved.Config != nil {
			if strings.TrimSpace(resolved.CacheKey) == "" {
				return effectiveClientConfiguration{}, fmt.Errorf("%w: resolved credential version is blank", ErrClientConfigurationUnavailable)
			}
			effectiveConfig = cloneClientConfig(resolved.Config)
			credentialVersion = "resolved:" + strings.TrimSpace(resolved.CacheKey)
			usesStatic = false
		}
	}
	if !validEffectiveClientConfig(effectiveConfig) {
		return effectiveClientConfiguration{}, fmt.Errorf("image credential configuration is unavailable: %w: client %q", ErrClientConfigurationUnavailable, name)
	}
	providerID, ok := effectiveProviderID(effectiveConfig.APIStyle)
	if !ok {
		return effectiveClientConfiguration{}, fmt.Errorf("%w: api style %q", ErrClientConfigurationUnsupported, effectiveConfig.APIStyle)
	}
	route := EffectiveClientRoute{
		ProviderID: providerID, ModelID: strings.TrimSpace(effectiveConfig.Model), CredentialReference: name,
		ConfigurationVersion: effectiveConfigurationVersion(name, credentialVersion, effectiveConfig),
	}
	result := effectiveClientConfiguration{config: effectiveConfig, route: route}
	if !usesStatic {
		result.resolverVersion = strings.TrimPrefix(credentialVersion, "resolved:")
	}
	if usesStatic {
		result.staticClient = staticClient
	}
	return result, nil
}

func selectionVersion(selection *ImageRouteSelection) string {
	if selection == nil {
		return ""
	}
	return selection.ConfigurationVersion
}

func validEffectiveClientConfig(config *ClientConfig) bool {
	return config != nil && strings.TrimSpace(config.APIKey) != "" && strings.TrimSpace(config.BaseURL) != "" && strings.TrimSpace(config.Model) != ""
}

func effectiveProviderID(apiStyle string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(apiStyle)) {
	case "", "openai", "openai-compatible":
		return "openai", true
	case "gemini":
		return "gemini", true
	default:
		return "", false
	}
}

func effectiveConfigurationVersion(name, credentialVersion string, config *ClientConfig) string {
	// APIKey is deliberately absent. credentialVersion is supplied by the
	// resolver and must identify its credential row/version without containing
	// the credential secret (the GORM resolver uses ID + UpdatedAt + name).
	canonical := fmt.Sprintf("v1|%s|%s|%s|%s|%s|%d|%d|%d|%d|%d",
		normalizeClientName(name), credentialVersion, strings.ToLower(strings.TrimSpace(config.APIStyle)),
		strings.TrimSpace(config.Model), strings.TrimSpace(config.BaseURL), config.Timeout,
		config.MaxRetries, config.RetryDelay, config.MaxReferenceMaterializedBytes, config.MaxReferenceMaterializationConcurrency,
	)
	digest := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("effective:v1:%x", digest[:])
}

func (m *Manager) resolveStaticClient(name string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if client := m.clients[name]; client != nil {
		return client, nil
	}
	return nil, fmt.Errorf("client %s not found", name)
}

// ListClients 列出所有已注册的客户端名称
func (m *Manager) ListClients() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// Close 关闭所有客户端
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			m.logger.Errorf("关闭客户端失败 %s: %v", name, err)
		}
	}
	m.logger.Info("管理器已关闭")
	return nil
}
