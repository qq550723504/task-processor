// Package service 提供业务逻辑层
package service

import (
	"fmt"
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// ConfigService 配置服务
type ConfigService struct {
	config *config.Config
}

// NewConfigService 创建配置服务实例
func NewConfigService() *ConfigService {
	return &ConfigService{}
}

// LoadFromFile 从文件加载配置
func (s *ConfigService) LoadFromFile(configFile string) error {
	logrus.Infof("从文件加载配置: %s", configFile)

	cfg := config.LoadConfigFromFile(configFile)
	if cfg == nil {
		return fmt.Errorf("配置加载失败")
	}

	s.config = cfg
	logrus.Info("配置加载成功")
	return nil
}

// GetConfig 获取配置
func (s *ConfigService) GetConfig() *config.Config {
	if s.config == nil {
		logrus.Warn("配置未加载，使用默认配置")
		s.config = config.LoadConfig()
	}
	return s.config
}

// LoadConfig 加载配置（兼容旧接口）
func (s *ConfigService) LoadConfig(configFile string) *config.Config {
	if configFile != "" {
		if err := s.LoadFromFile(configFile); err != nil {
			logrus.Errorf("加载配置文件失败: %v，使用默认配置", err)
		}
	}

	return s.GetConfig()
}
