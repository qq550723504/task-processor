// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"context"

	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// amazonCrawler 定义 runner 包对 Amazon 爬虫的依赖（消费者定义接口原则）。
// 包含抓取和关闭两个能力，满足 TEMU/SHEIN 处理器的需求。
type amazonCrawler interface {
	Process(url string, zipcode string) (*model.Product, error)
	ProcessWithContext(ctx context.Context, url string, zipcode string) (*model.Product, error)
	Shutdown()
}

// ProcessorService 处理器服务接口
type ProcessorService interface {
	// StartProcessors 启动所有处理器
	StartProcessors(ctx context.Context, cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error
	// StopProcessors 停止所有处理器
	StopProcessors() error
	// GetStatus 获取处理器状态
	GetStatus() map[string]any
}

// NewProcessorService 创建处理器服务
func NewProcessorService(logger *logrus.Logger) ProcessorService {
	return &processorServiceImpl{
		logger:           logger,
		lifecycleManager: lifecycle.NewLifecycleManager(logger),
	}
}

// NewProcessorServiceWithDependencies 创建处理器服务（带依赖注入）
func NewProcessorServiceWithDependencies(
	logger *logrus.Logger,
	managementClient *management.ClientManager,
	amazonProcessor amazonCrawler,
) ProcessorService {
	return &processorServiceImpl{
		logger:                logger,
		lifecycleManager:      lifecycle.NewLifecycleManager(logger),
		managementClient:      managementClient,
		amazonProcessor:       amazonProcessor,
		temuProcessorCreator:  BuildDefaultProcessorDependencies().TemuProcessorCreator,
		sheinProcessorCreator: BuildDefaultProcessorDependencies().SheinProcessorCreator,
	}
}

func NewProcessorServiceWithCreators(
	logger *logrus.Logger,
	managementClient *management.ClientManager,
	amazonProcessor amazonCrawler,
	deps ProcessorDependencies,
) ProcessorService {
	defaultDeps := BuildDefaultProcessorDependencies()
	if deps.TemuProcessorCreator == nil {
		deps.TemuProcessorCreator = defaultDeps.TemuProcessorCreator
	}
	if deps.SheinProcessorCreator == nil {
		deps.SheinProcessorCreator = defaultDeps.SheinProcessorCreator
	}

	return &processorServiceImpl{
		logger:                logger,
		lifecycleManager:      lifecycle.NewLifecycleManager(logger),
		managementClient:      managementClient,
		amazonProcessor:       amazonProcessor,
		temuProcessorCreator:  deps.TemuProcessorCreator,
		sheinProcessorCreator: deps.SheinProcessorCreator,
	}
}
