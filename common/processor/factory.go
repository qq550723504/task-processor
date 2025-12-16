// Package processor 提供处理器工厂实现
package processor

import (
	"fmt"
	"task-processor/common/management"
	"task-processor/internal/config"

	"github.com/sirupsen/logrus"
)

// ProcessorFactory 处理器工厂接口
type ProcessorFactory interface {
	CreateProcessor(platform string, cfg *config.Config, logger *logrus.Logger) (Processor, error)
	CreateProcessorWithSharedResources(platform string, cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager, sharedResources map[string]any) (Processor, error)
}

// DefaultProcessorFactory 默认处理器工厂实现
type DefaultProcessorFactory struct {
	creators map[string]ProcessorCreator
}

// ProcessorCreator 处理器创建器函数类型
type ProcessorCreator func(cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager, sharedResources map[string]any) (Processor, error)

// NewProcessorFactory 创建处理器工厂
func NewProcessorFactory() ProcessorFactory {
	return &DefaultProcessorFactory{
		creators: make(map[string]ProcessorCreator),
	}
}

// RegisterCreator 注册处理器创建器
func (f *DefaultProcessorFactory) RegisterCreator(platform string, creator ProcessorCreator) {
	f.creators[platform] = creator
}

// CreateProcessor 创建处理器
func (f *DefaultProcessorFactory) CreateProcessor(platform string, cfg *config.Config, logger *logrus.Logger) (Processor, error) {
	return f.CreateProcessorWithSharedResources(platform, cfg, logger, nil, nil)
}

// CreateProcessorWithSharedResources 使用共享资源创建处理器
func (f *DefaultProcessorFactory) CreateProcessorWithSharedResources(platform string, cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager, sharedResources map[string]any) (Processor, error) {
	creator, exists := f.creators[platform]
	if !exists {
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}

	processor, err := creator(cfg, logger, managementClient, sharedResources)
	if err != nil {
		return nil, fmt.Errorf("创建%s处理器失败: %w", platform, err)
	}

	return processor, nil
}

// GetSupportedPlatforms 获取支持的平台列表
func (f *DefaultProcessorFactory) GetSupportedPlatforms() []string {
	platforms := make([]string, 0, len(f.creators))
	for platform := range f.creators {
		platforms = append(platforms, platform)
	}
	return platforms
}

// ProcessorBuilder 处理器构建器
type ProcessorBuilder struct {
	platform         string
	config           *config.Config
	logger           *logrus.Logger
	managementClient *management.ClientManager
	sharedResources  map[string]any
}

// NewProcessorBuilder 创建处理器构建器
func NewProcessorBuilder(platform string) *ProcessorBuilder {
	return &ProcessorBuilder{
		platform:        platform,
		sharedResources: make(map[string]any),
	}
}

// WithConfig 设置配置
func (b *ProcessorBuilder) WithConfig(cfg *config.Config) *ProcessorBuilder {
	b.config = cfg
	return b
}

// WithLogger 设置日志器
func (b *ProcessorBuilder) WithLogger(logger *logrus.Logger) *ProcessorBuilder {
	b.logger = logger
	return b
}

// WithManagementClient 设置管理客户端
func (b *ProcessorBuilder) WithManagementClient(client *management.ClientManager) *ProcessorBuilder {
	b.managementClient = client
	return b
}

// WithSharedResource 添加共享资源
func (b *ProcessorBuilder) WithSharedResource(key string, resource any) *ProcessorBuilder {
	b.sharedResources[key] = resource
	return b
}

// Build 构建处理器
func (b *ProcessorBuilder) Build(factory ProcessorFactory) (Processor, error) {
	if b.config == nil {
		return nil, fmt.Errorf("配置不能为空")
	}
	if b.logger == nil {
		return nil, fmt.Errorf("日志器不能为空")
	}

	return factory.CreateProcessorWithSharedResources(
		b.platform,
		b.config,
		b.logger,
		b.managementClient,
		b.sharedResources,
	)
}
