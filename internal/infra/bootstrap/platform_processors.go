// Package bootstrap 提供平台处理器注册功能
package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/di"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/shein/service/pipeline"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

// PlatformProcessorRegistry 平台处理器注册器
type PlatformProcessorRegistry struct {
	logger *logrus.Logger
}

// NewPlatformProcessorRegistry 创建平台处理器注册器
func NewPlatformProcessorRegistry(logger *logrus.Logger) *PlatformProcessorRegistry {
	return &PlatformProcessorRegistry{
		logger: logger,
	}
}

// RegisterPlatformProcessors 注册所有平台处理器
func (p *PlatformProcessorRegistry) RegisterPlatformProcessors(container di.Container) error {
	p.logger.Debug("注册平台处理器...")

	// 注册TEMU处理器
	if err := p.registerTemuProcessor(container); err != nil {
		return fmt.Errorf("注册TEMU处理器失败: %w", err)
	}

	// 注册SHEIN处理器
	if err := p.registerSheinProcessor(container); err != nil {
		return fmt.Errorf("注册SHEIN处理器失败: %w", err)
	}

	p.logger.Debug("✅ 平台处理器注册完成")
	return nil
}

// registerTemuProcessor 注册TEMU处理器
func (p *PlatformProcessorRegistry) registerTemuProcessor(container di.Container) error {
	return container.Register("temuProcessor", func(c di.Container) (any, error) {
		configInstance, err := c.Get("config")
		if err != nil {
			return nil, fmt.Errorf("获取配置失败: %w", err)
		}
		loggerInstance, err := c.Get("logger")
		if err != nil {
			return nil, fmt.Errorf("获取日志器失败: %w", err)
		}
		managementClientInstance, err := c.Get("managementClient")
		if err != nil {
			return nil, fmt.Errorf("获取管理客户端失败: %w", err)
		}
		amazonProcessorInstance, err := c.Get("amazonProcessor")
		if err != nil {
			return nil, fmt.Errorf("获取Amazon处理器失败: %w", err)
		}

		config := configInstance.(*config.Config)
		logger := loggerInstance.(*logrus.Logger)
		managementClient := managementClientInstance.(*management.ClientManager)
		amazonProcessor := amazonProcessorInstance.(*amazon.AmazonProcessor)

		// 使用原有的构造函数，不修改任何业务逻辑
		return temu.NewTemuProcessor(context.Background(), config, logger, managementClient, amazonProcessor), nil
	})
}

// registerSheinProcessor 注册SHEIN处理器
func (p *PlatformProcessorRegistry) registerSheinProcessor(container di.Container) error {
	return container.Register("sheinProcessor", func(c di.Container) (any, error) {
		configInstance, err := c.Get("config")
		if err != nil {
			return nil, fmt.Errorf("获取配置失败: %w", err)
		}
		loggerInstance, err := c.Get("logger")
		if err != nil {
			return nil, fmt.Errorf("获取日志器失败: %w", err)
		}
		managementClientInstance, err := c.Get("managementClient")
		if err != nil {
			return nil, fmt.Errorf("获取管理客户端失败: %w", err)
		}
		amazonProcessorInstance, err := c.Get("amazonProcessor")
		if err != nil {
			return nil, fmt.Errorf("获取Amazon处理器失败: %w", err)
		}

		config := configInstance.(*config.Config)
		logger := loggerInstance.(*logrus.Logger)
		managementClient := managementClientInstance.(*management.ClientManager)
		amazonProcessor := amazonProcessorInstance.(*amazon.AmazonProcessor)

		// 使用原有的构造函数，不修改任何业务逻辑
		return pipeline.NewSheinProcessor(context.Background(), config, logger, managementClient, amazonProcessor), nil
	})
}
