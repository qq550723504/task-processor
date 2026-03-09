// Package config 提供配置管理器实现
package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// ConfigSource 配置源接口
type ConfigSource interface {
	// Read 读取配置数据
	Read() ([]byte, error)

	// Watch 监听配置变化
	Watch(ctx context.Context, callback func([]byte)) error

	// Name 返回配置源名称
	Name() string
}

// ConfigManager 配置管理器接口
type ConfigManager interface {
	// Load 从配置源加载配置
	Load(source ConfigSource) (*Config, error)

	// Validate 验证配置
	Validate(cfg *Config) error

	// GetCurrent 获取当前配置
	GetCurrent() *Config

	// Reload 重新加载配置
	Reload() error

	// Watch 监听配置变化
	Watch(ctx context.Context, callback func(*Config)) error
}

// managerImpl 配置管理器实现
type managerImpl struct {
	logger        *logrus.Logger
	currentConfig *Config
	currentSource ConfigSource
	mu            sync.RWMutex
}

// NewConfigManager 创建配置管理器
func NewConfigManager(logger *logrus.Logger) ConfigManager {
	return &managerImpl{
		logger: logger,
	}
}

// Load 从配置源加载配置
func (m *managerImpl) Load(source ConfigSource) (*Config, error) {
	if source == nil {
		return nil, fmt.Errorf("配置源不能为空")
	}

	m.logger.Infof("从配置源 %s 加载配置", source.Name())

	// 读取配置数据
	data, err := source.Read()
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	m.logger.Infof("读取到配置数据，长度: %d 字节", len(data))

	// 解析配置
	cfg, err := LoadFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 安全地记录配置信息
	browserPath := "未配置"
	if cfg.Browser.BrowserPath != "" {
		browserPath = cfg.Browser.BrowserPath
	}
	m.logger.Infof("配置解析成功，浏览器路径: %s", browserPath)

	// 验证配置
	if err := m.Validate(cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 更新当前配置
	m.mu.Lock()
	m.currentConfig = cfg
	m.currentSource = source
	m.mu.Unlock()

	m.logger.Info("配置加载成功")
	return cfg, nil
}

// Validate 验证配置
func (m *managerImpl) Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("配置不能为空")
	}

	// 基本验证逻辑
	if cfg.Worker.Concurrency <= 0 {
		return fmt.Errorf("工作池并发数必须大于0")
	}
	if cfg.Worker.BufferSize <= 0 {
		return fmt.Errorf("工作池缓冲区大小必须大于0")
	}

	// 管理系统URL验证 - 如果为空则使用默认值
	if cfg.Management.BaseURL == "" {
		m.logger.Debug("管理系统URL为空，将使用默认值")
		// 这里不返回错误，让默认值处理逻辑来填充
	}

	return nil
}

// GetCurrent 获取当前配置
func (m *managerImpl) GetCurrent() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentConfig
}

// Reload 重新加载配置
func (m *managerImpl) Reload() error {
	m.mu.RLock()
	source := m.currentSource
	m.mu.RUnlock()

	if source == nil {
		return fmt.Errorf("没有配置源可以重新加载")
	}

	_, err := m.Load(source)
	return err
}

// Watch 监听配置变化
func (m *managerImpl) Watch(ctx context.Context, callback func(*Config)) error {
	m.mu.RLock()
	source := m.currentSource
	m.mu.RUnlock()

	if source == nil {
		return fmt.Errorf("没有配置源可以监听")
	}

	return source.Watch(ctx, func(data []byte) {
		m.logger.Info("检测到配置变化，重新加载配置")

		// 解析新配置
		cfg, err := LoadFromBytes(data)
		if err != nil {
			m.logger.Errorf("解析新配置失败: %v", err)
			return
		}

		// 验证新配置
		if err := m.Validate(cfg); err != nil {
			m.logger.Errorf("新配置验证失败: %v", err)
			return
		}

		// 更新当前配置
		m.mu.Lock()
		m.currentConfig = cfg
		m.mu.Unlock()

		// 通知回调
		if callback != nil {
			callback(cfg)
		}

		m.logger.Info("配置重新加载成功")
	})
}
