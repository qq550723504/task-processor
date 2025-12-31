// Package service 提供业务逻辑层
package service

import (
	"log"
	"os"
	"task-processor/internal/core/config"
)

// ConfigService 配置服务
type ConfigService struct{}

// NewConfigService 创建配置服务实例
func NewConfigService() *ConfigService {
	return &ConfigService{}
}

// LoadConfig 加载配置
func (s *ConfigService) LoadConfig(configFile string) *config.Config {
	if configFile != "" {
		// 使用指定的配置文件
		os.Setenv("CONFIG_FILE", configFile)
	}

	cfg := config.LoadConfig()
	if cfg == nil {
		log.Println("警告: 配置加载失败，使用默认配置")
		cfg = s.getDefaultConfig()
	}

	return cfg
}

// getDefaultConfig 获取默认配置
func (s *ConfigService) getDefaultConfig() *config.Config {
	return &config.Config{
		Amazon: config.AmazonConfig{
			Enabled:        true,
			Headless:       true,
			BrowserPath:    "",
			PoolSize:       1,
			ViewportWidth:  1920,
			ViewportHeight: 1080,
			ProxyServer:    "",
		},
	}
}
