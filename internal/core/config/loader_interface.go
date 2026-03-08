// Package config 提供配置管理功能
package config

// ConfigLoader 统一的配置加载器接口
type ConfigLoader interface {
	// Load 加载配置
	Load(path string) error

	// Reload 重新加载配置(支持热更新)
	Reload() error

	// Validate 验证配置合法性
	Validate() error

	// GetConfig 获取配置对象
	GetConfig() any
}

// BaseConfigLoader 基础配置加载器(提供通用实现)
type BaseConfigLoader struct {
	configPath string
	validator  ConfigValidator
}

// ConfigValidator 配置验证器接口
type ConfigValidator interface {
	Validate(config any) error
}

// NewBaseConfigLoader 创建基础配置加载器
func NewBaseConfigLoader(path string, validator ConfigValidator) *BaseConfigLoader {
	return &BaseConfigLoader{
		configPath: path,
		validator:  validator,
	}
}

// GetConfigPath 获取配置文件路径
func (b *BaseConfigLoader) GetConfigPath() string {
	return b.configPath
}

// ValidateConfig 验证配置
func (b *BaseConfigLoader) ValidateConfig(config any) error {
	if b.validator != nil {
		return b.validator.Validate(config)
	}
	return nil
}
