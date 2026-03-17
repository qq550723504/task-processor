// Package openai 提供OpenAI客户端管理器
package openai

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// Manager OpenAI客户端管理器
type Manager struct {
	clients       map[string]*Client
	defaultClient *Client
	logger        *logrus.Entry
	mu            sync.RWMutex
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	Clients       map[string]*ClientConfig
	DefaultClient string
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
		clients: make(map[string]*Client),
		logger:  logrus.WithField("component", "OpenAIManager"),
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
func (m *Manager) GetClient(name string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, exists := m.clients[name]
	if !exists {
		return nil, fmt.Errorf("client %s not found", name)
	}
	return client, nil
}

// GetDefaultClient 获取默认客户端
func (m *Manager) GetDefaultClient() *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultClient
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
