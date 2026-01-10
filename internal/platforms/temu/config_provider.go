// Package temu 提供配置访问接口，避免循环导入
package temu

import (
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
)

// ConfigProvider 配置提供者接口
type ConfigProvider interface {
	GetAmazonConfig() *config.AmazonConfig
	GetAmazonProcessor() *amazon.AmazonProcessor
	GetPlatformConfig() *config.PlatformConfig // 改为获取平台配置
}

// DefaultConfigProvider 默认配置提供者
type DefaultConfigProvider struct {
	amazonConfig    *config.AmazonConfig
	amazonProcessor *amazon.AmazonProcessor
	platformConfig  *config.PlatformConfig // 改为平台配置
}

// NewDefaultConfigProvider 创建默认配置提供者
func NewDefaultConfigProvider(amazonConfig *config.AmazonConfig, amazonProcessor *amazon.AmazonProcessor, platformConfig *config.PlatformConfig) *DefaultConfigProvider {
	return &DefaultConfigProvider{
		amazonConfig:    amazonConfig,
		amazonProcessor: amazonProcessor,
		platformConfig:  platformConfig,
	}
}

// GetAmazonConfig 获取Amazon配置
func (p *DefaultConfigProvider) GetAmazonConfig() *config.AmazonConfig {
	return p.amazonConfig
}

// GetAmazonProcessor 获取Amazon处理器
func (p *DefaultConfigProvider) GetAmazonProcessor() *amazon.AmazonProcessor {
	return p.amazonProcessor
}

// GetPlatformConfig 获取平台配置
func (p *DefaultConfigProvider) GetPlatformConfig() *config.PlatformConfig {
	return p.platformConfig
}
