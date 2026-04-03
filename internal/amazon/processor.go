// Package amazon 提供Amazon平台主处理器
package amazon

import (
	"context"
	"fmt"

	"task-processor/internal/amazon/api"
	"task-processor/internal/amazon/listing"
	"task-processor/internal/amazon/llm"
	amazonModel "task-processor/internal/amazon/model"
	"task-processor/internal/amazon/pipeline"
	"task-processor/internal/app/processor"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/worker"
	"task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"

	"github.com/sirupsen/logrus"
)

// Processor Amazon平台处理器
type Processor struct {
	*processor.BaseProcessor                       // 继承基础处理器
	services                 *amazonModel.Services // Amazon特定：服务容器
	apiClient                *api.Client           // Amazon特定：API客户端
}

// NewProcessor 创建Amazon处理器
func NewProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger) *Processor {
	// 创建基础处理器
	baseProcessor := processor.NewBaseProcessor(ctx, &processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: nil, // Amazon处理器可能不需要管理客户端
		Logger:           logger,
		Platform:         "Amazon",
	})

	// 创建服务容器
	services := amazonModel.NewServices()
	services.SetManagementClient(baseProcessor.GetManagementClient())
	services.SetMemoryManager(baseProcessor.GetMemoryManager())

	// 创建 API 客户端
	apiClient := createAPIClient(cfg)
	services.SetAPIClient(apiClient)

	// 创建产品类型推荐服务
	productTypeService := listing.NewProductTypeRecommendationService(apiClient)
	services.SetProductTypeService(productTypeService)

	// 初始化 LLM 属性映射器
	openaiClient := openai.NewClient(cfg.OpenAI.ToClientConfig())
	services.SetLLMAttributeMapper(llm.NewLLMAttributeMapper(llm.NewOpenAILLMClient(openaiClient)))

	p := &Processor{
		BaseProcessor: baseProcessor,
		services:      services,
		apiClient:     apiClient,
	}

	logger.Info("[Amazon] 处理器初始化完成，LLM服务已集成")
	return p
}

// Start 启动处理器
func (p *Processor) Start(ctx context.Context) error {
	// 启动基础组件
	if err := p.StartBase(ctx); err != nil {
		return err
	}

	p.GetLogger().Info("[Amazon] 处理器启动完成")
	return nil
}

// ProcessTask 处理任务 - 实现worker.Processor接口
func (p *Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	// 解析任务数据
	var task model.Task
	if err := jsonx.UnmarshalString(job.TaskData, &task, "解析任务数据失败"); err != nil {
		return err
	}

	logger := p.GetLogger()
	logger.Infof("[Amazon] 开始处理任务: ID=%d, ProductID=%s", task.ID, task.ProductID)

	// 将任务转换为处理所需的数据格式
	taskData := map[string]any{
		"taskId":     task.ID,
		"tenantId":   task.TenantID,
		"storeId":    task.StoreID,
		"platform":   task.Platform,
		"region":     task.Region,
		"categoryId": task.CategoryID,
		"productId":  task.ProductID,
		"priority":   task.Priority,
	}

	// 使用现有的管道处理逻辑
	taskContext := p.createTaskContext(taskData)
	defer p.rollbackReservedDailyQuota(taskContext)

	err := p.ProcessTaskWithPipeline(ctx, taskContext)
	if err != nil {
		p.handleTaskFailure(&task, err)
		logger.Errorf("[Amazon] 任务处理失败: ID=%d, Error=%v", task.ID, err)
		return fmt.Errorf("Amazon任务处理失败: %w", err)
	}

	taskContext.ClearDailyQuotaReservation()
	p.handleTaskSuccess(&task)
	logger.Infof("[Amazon] 任务处理成功: ID=%d", task.ID)
	return nil
}

// Close 关闭处理器
func (p *Processor) Close(ctx context.Context) {
	p.GetLogger().Info("[Amazon] 关闭处理器")

	// 关闭基础组件
	p.CloseBase(ctx)

	p.GetLogger().Info("[Amazon] 处理器已关闭")
}

// createAPIClient 创建API客户端
func createAPIClient(cfg *config.Config) *api.Client {
	apiConfig := &api.Config{
		Region:         cfg.Amazon.SPAPI.Region,
		MarketplaceID:  config.ResolveAmazonMarketplaceID(cfg.Amazon.SPAPI),
		ClientID:       cfg.Amazon.SPAPI.ClientID,
		ClientSecret:   cfg.Amazon.SPAPI.ClientSecret,
		RefreshToken:   cfg.Amazon.SPAPI.RefreshToken,
		AWSAccessKeyID: cfg.Amazon.SPAPI.AWSAccessKeyID,
		AWSSecretKey:   cfg.Amazon.SPAPI.AWSSecretKey,
	}

	return api.NewClient(apiConfig)
}

// GetStatus 获取处理器状态
func (p *Processor) GetStatus() map[string]any {
	return map[string]any{
		"name":   "Amazon处理器",
		"status": "running",
	}
}

// ProcessTaskWithPipeline 使用完整管道处理任务并显示详细流程
func (p *Processor) ProcessTaskWithPipeline(ctx context.Context, taskContext *amazonModel.TaskContext) error {
	logger := p.GetLogger()
	logger.Info("🔧 开始管道流程详细处理")

	// 创建流水线管理器
	manager := pipeline.NewHandlerManager(p.services)

	// 执行完整处理流程
	logger.Info("🚀 开始执行管道处理流程:")

	err := manager.ProcessProduct(ctx, taskContext)

	if err != nil {
		return fmt.Errorf("管道处理失败: %w", err)
	}

	return nil
}

// createTaskContext 创建任务上下文
func (p *Processor) createTaskContext(taskData map[string]any) *amazonModel.TaskContext {
	taskID := "pipeline-task-001"
	if rawTaskID, ok := taskData["taskId"]; ok {
		taskID = fmt.Sprintf("%v", rawTaskID)
	}
	return &amazonModel.TaskContext{
		TaskID:        taskID,
		MarketplaceID: "ATVPDKIKX0DER",
		LanguageTag:   "en_US",
		Currency:      "USD",
		Data:          taskData,
	}
}

func (p *Processor) rollbackReservedDailyQuota(taskContext *amazonModel.TaskContext) {
	if taskContext == nil || !taskContext.DailyQuotaReserved {
		return
	}
	if p.GetMemoryManager() == nil {
		return
	}

	if listingSKU, exists := taskContext.GetResult("listing_sku"); exists && listingSKU != nil {
		return
	}

	tenantIDValue, tenantExists := taskContext.Data["tenantId"]
	storeIDValue, storeExists := taskContext.Data["storeId"]
	if !tenantExists || !storeExists {
		return
	}

	tenantID, okTenant := toInt64(tenantIDValue)
	storeID, okStore := toInt64(storeIDValue)
	if !okTenant || !okStore {
		return
	}

	if _, err := p.GetMemoryManager().DailyCountManager.RollbackReservedQuota(
		tenantID,
		storeID,
		taskContext.DailyQuotaDate,
		taskContext.DailyQuotaIncrement,
	); err != nil {
		p.GetLogger().WithError(err).Warnf("[Amazon] 回滚预占额度失败: tenant=%d, store=%d", tenantID, storeID)
		return
	}

	taskContext.ClearDailyQuotaReservation()
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}
