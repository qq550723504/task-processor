// Package amazon 提供Amazon平台主处理器
package amazon

import (
	"context"
	"fmt"

	"task-processor/internal/config"
	"task-processor/platforms/amazon/api"
	"task-processor/platforms/amazon/internal/handler"
	"task-processor/platforms/amazon/internal/model"
	"task-processor/platforms/amazon/internal/service"

	"github.com/sirupsen/logrus"
)

// Processor Amazon平台处理器
type Processor struct {
	config    *config.Config
	services  *model.Services
	apiClient *api.Client
	logger    *logrus.Logger
}

// NewProcessor 创建Amazon处理器
func NewProcessor(cfg *config.Config, logger *logrus.Logger) *Processor {
	// 创建服务容器
	services := model.NewServices()

	// 创建 API 客户端
	apiClient := createAPIClient(cfg)
	services.SetAPIClient(apiClient)

	// 创建产品类型推荐服务
	productTypeService := service.NewProductTypeRecommendationService(apiClient)
	services.SetProductTypeService(productTypeService)

	// 创建服务工厂并初始化LLM服务
	serviceFactory := service.NewServiceFactory(cfg)
	serviceFactory.UpdateServices(services)

	p := &Processor{
		config:    cfg,
		services:  services,
		apiClient: apiClient,
		logger:    logger,
	}

	p.logger.Info("[Amazon] 处理器初始化完成，LLM服务已集成")
	return p
}

// Start 启动处理器
func (p *Processor) Start(ctx context.Context) error {
	p.logger.Info("[Amazon] 启动处理器")
	return nil
}

// Stop 停止处理器
func (p *Processor) Stop(ctx context.Context) error {
	p.logger.Info("[Amazon] 停止处理器")
	return nil
}

// createAPIClient 创建API客户端
func createAPIClient(cfg *config.Config) *api.Client {
	apiConfig := &api.Config{
		Region:         cfg.Amazon.SPAPI.Region,
		MarketplaceID:  cfg.Amazon.SPAPI.DefaultMarketplace,
		SellerID:       cfg.Amazon.SPAPI.SellerID,
		ClientID:       cfg.Amazon.SPAPI.ClientID,
		ClientSecret:   cfg.Amazon.SPAPI.ClientSecret,
		RefreshToken:   cfg.Amazon.SPAPI.RefreshToken,
		AWSAccessKeyID: cfg.Amazon.SPAPI.AWSAccessKeyID,
		AWSSecretKey:   cfg.Amazon.SPAPI.AWSSecretKey,
	}

	return api.NewClient(apiConfig)
}

// GetStatus 获取处理器状态
func (p *Processor) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"name":   "Amazon处理器",
		"status": "running",
	}
}

// ProcessTaskWithPipeline 使用完整管道处理任务并显示详细流程
func (p *Processor) ProcessTaskWithPipeline(ctx context.Context, taskData map[string]interface{}) error {
	p.logger.Info("🔧 开始管道流程详细处理")

	// 创建Handler管理器
	manager := handler.NewHandlerManager(p.services)

	// 创建任务上下文
	taskContext := p.createTaskContext(taskData)

	// 执行完整处理流程
	p.logger.Info("🚀 开始执行管道处理流程:")

	err := manager.ProcessProduct(ctx, taskContext)

	if err != nil {
		return fmt.Errorf("管道处理失败: %w", err)
	}

	return nil
}

// createTaskContext 创建任务上下文
func (p *Processor) createTaskContext(taskData map[string]interface{}) *model.TaskContext {
	return &model.TaskContext{
		TaskID:        "pipeline-task-001",
		MarketplaceID: "ATVPDKIKX0DER",
		LanguageTag:   "en_US",
		Currency:      "USD",
		Data:          taskData,
	}
}
