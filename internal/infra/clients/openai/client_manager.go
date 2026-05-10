// Package openai 提供OpenAI客户端管理器
package openai

import (
	"context"
	"fmt"
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

func (m *Manager) GetImageClient(name string) (ImageGenerator, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.clients[name]; !exists {
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
	fallback, err := m.resolveStaticClient(name)
	if err != nil {
		return nil, err
	}
	if m.configResolver == nil {
		return fallback, nil
	}
	resolved, err := m.configResolver.ResolveClientConfig(ctx, name, fallback.config)
	if err != nil {
		return nil, err
	}
	if resolved == nil || resolved.Config == nil {
		return fallback, nil
	}
	cacheKey := resolved.CacheKey
	if cacheKey == "" {
		cacheKey = name + ":" + resolved.Config.APIKey + ":" + resolved.Config.BaseURL + ":" + resolved.Config.Model
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if client := m.dynamicClients[cacheKey]; client != nil {
		return client, nil
	}
	client := NewClient(resolved.Config)
	if client == nil {
		return nil, fmt.Errorf("failed to create resolved client: %s", name)
	}
	m.dynamicClients[cacheKey] = client
	return client, nil
}

func (m *Manager) resolveStaticClient(name string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if client := m.clients[name]; client != nil {
		return client, nil
	}
	if name != "default" {
		if client := m.clients["default"]; client != nil {
			return client, nil
		}
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
