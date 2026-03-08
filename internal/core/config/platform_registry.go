// Package config 提供平台配置注册功能
package config

import (
	"fmt"
	"sync"
	"task-processor/internal/core/config/types"
)

// PlatformConfigProvider 平台配置提供者接口
// 每个平台实现此接口以提供自己的配置
type PlatformConfigProvider interface {
	// Name 返回平台名称 (如 "temu", "shein", "amazon")
	Name() string

	// GetConfig 获取平台配置
	GetConfig() *types.PlatformConfig

	// Validate 验证平台配置
	Validate() error

	// GetDefaultConfig 获取默认配置
	GetDefaultConfig() *types.PlatformConfig
}

// PlatformRegistry 平台注册表
// 用于管理所有已注册的平台配置提供者
type PlatformRegistry struct {
	providers map[string]PlatformConfigProvider
	mu        sync.RWMutex
}

// 全局平台注册表
var globalPlatformRegistry = &PlatformRegistry{
	providers: make(map[string]PlatformConfigProvider),
}

// RegisterPlatform 注册平台配置提供者到全局注册表
// 通常在 init() 函数中调用
func RegisterPlatform(provider PlatformConfigProvider) error {
	return globalPlatformRegistry.Register(provider)
}

// GetPlatform 从全局注册表获取平台配置提供者
func GetPlatform(name string) (PlatformConfigProvider, error) {
	return globalPlatformRegistry.Get(name)
}

// ListPlatforms 列出所有已注册的平台名称
func ListPlatforms() []string {
	return globalPlatformRegistry.List()
}

// GetPlatformRegistry 获取全局平台注册表
func GetPlatformRegistry() *PlatformRegistry {
	return globalPlatformRegistry
}

// Register 注册平台配置提供者
func (r *PlatformRegistry) Register(provider PlatformConfigProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	if name == "" {
		return fmt.Errorf("平台名称不能为空")
	}

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("平台 %s 已注册", name)
	}

	r.providers[name] = provider
	return nil
}

// Get 获取平台配置提供者
func (r *PlatformRegistry) Get(name string) (PlatformConfigProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("平台 %s 未注册", name)
	}

	return provider, nil
}

// List 列出所有已注册的平台名称
func (r *PlatformRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// Has 检查平台是否已注册
func (r *PlatformRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.providers[name]
	return ok
}

// Unregister 注销平台配置提供者
func (r *PlatformRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("平台 %s 未注册", name)
	}

	delete(r.providers, name)
	return nil
}

// Clear 清空所有已注册的平台
func (r *PlatformRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers = make(map[string]PlatformConfigProvider)
}

// BasePlatformProvider 基础平台配置提供者
// 提供默认实现,具体平台可以嵌入此结构体
type BasePlatformProvider struct {
	name          string
	config        *types.PlatformConfig
	defaultConfig *types.PlatformConfig
}

// NewBasePlatformProvider 创建基础平台配置提供者
func NewBasePlatformProvider(name string, config *types.PlatformConfig) *BasePlatformProvider {
	return &BasePlatformProvider{
		name:          name,
		config:        config,
		defaultConfig: getDefaultPlatformConfig(),
	}
}

// Name 返回平台名称
func (p *BasePlatformProvider) Name() string {
	return p.name
}

// GetConfig 获取平台配置
func (p *BasePlatformProvider) GetConfig() *types.PlatformConfig {
	return p.config
}

// Validate 验证平台配置
func (p *BasePlatformProvider) Validate() error {
	if p.config == nil {
		return fmt.Errorf("平台配置不能为空")
	}

	// 基本验证
	if p.config.AutoPricing.Enabled && p.config.AutoPricing.Interval <= 0 {
		return fmt.Errorf("自动核价间隔必须大于 0")
	}

	if p.config.AutoPricing.Enabled && p.config.AutoPricing.BatchSize <= 0 {
		return fmt.Errorf("自动核价批量大小必须大于 0")
	}

	return nil
}

// GetDefaultConfig 获取默认配置
func (p *BasePlatformProvider) GetDefaultConfig() *types.PlatformConfig {
	return p.defaultConfig
}

// getDefaultPlatformConfig 获取默认平台配置
func getDefaultPlatformConfig() *types.PlatformConfig {
	return &types.PlatformConfig{
		Enabled:          false,
		SchedulerEnabled: false,
		AutoPricing: types.AutoPricingConfig{
			Enabled:   false,
			Interval:  300,
			BatchSize: 100,
		},
		ProductSync: types.ScheduledTaskConfig{
			Enabled:  false,
			Interval: 3600,
		},
		InventorySync: types.ScheduledTaskConfig{
			Enabled:  false,
			Interval: 1800,
		},
		ActivityRegistration: types.ScheduledTaskConfig{
			Enabled:  false,
			Interval: 7200,
		},
	}
}
