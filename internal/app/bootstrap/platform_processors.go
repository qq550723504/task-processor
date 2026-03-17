// Package bootstrap 提供平台处理器注册功能
package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/app/di"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/model"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

// amazonCrawler 定义 bootstrap 包对 Amazon 爬虫的依赖（消费者定义接口原则）。
type amazonCrawler interface {
	Process(url string, zipcode string) (*model.Product, error)
	Shutdown()
}

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

// getDependencies 获取通用依赖
func (p *PlatformProcessorRegistry) getDependencies(c di.Container) (
	*config.Config,
	*logrus.Logger,
	*management.ClientManager,
	amazonCrawler,
	error,
) {
	configInstance, err := c.Get("config")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("获取配置失败: %w", err)
	}

	loggerInstance, err := c.Get("logger")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("获取日志器失败: %w", err)
	}

	managementClientInstance, err := c.Get("managementClient")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("获取管理客户端失败: %w", err)
	}

	amazonProcessorInstance, err := c.Get("amazonCrawler")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("获取Amazon爬虫处理器失败: %w", err)
	}

	return configInstance.(*config.Config),
		loggerInstance.(*logrus.Logger),
		managementClientInstance.(*management.ClientManager),
		amazonProcessorInstance.(*amazon.AmazonProcessor),
		nil
}

// registerTemuProcessor 注册TEMU处理器
func (p *PlatformProcessorRegistry) registerTemuProcessor(container di.Container) error {
	return container.Register("temuProcessor", func(c di.Container) (any, error) {
		config, logger, managementClient, amazonProcessor, err := p.getDependencies(c)
		if err != nil {
			return nil, err
		}

		// 注意：这里传递 nil 作为 rabbitmqClient，因为旧的启动方式不使用 RabbitMQ
		// 如果需要使用分布式爬虫，请使用 cmd/rabbitmq-consumer 启动程序
		return temu.NewTemuProcessor(context.Background(), config, logger, managementClient, amazonProcessor, nil)
	})
}

// registerSheinProcessor 注册SHEIN处理器
func (p *PlatformProcessorRegistry) registerSheinProcessor(container di.Container) error {
	return container.Register("sheinProcessor", func(c di.Container) (any, error) {
		config, logger, managementClient, amazonProcessor, err := p.getDependencies(c)
		if err != nil {
			return nil, err
		}

		// 注意：这里传递 nil 作为 rabbitmqClient，因为旧的启动方式不使用 RabbitMQ
		// 如果需要使用分布式爬虫，请使用 cmd/rabbitmq-consumer 启动程序
		return pipeline.NewSheinProcessor(context.Background(), config, logger, managementClient, amazonProcessor, nil)
	})
}

