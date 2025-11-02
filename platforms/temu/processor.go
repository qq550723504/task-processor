package temu

import (
	"context"
	"fmt"
	"task-processor/common/amazon"
	"task-processor/common/config"
	"task-processor/common/management"
	"task-processor/common/pipeline"
	"task-processor/common/processor"
	"task-processor/common/temu"
	"task-processor/common/types"

	"github.com/sirupsen/logrus"
)

// TemuProcessor TEMU平台处理器
type TemuProcessor struct {
	*processor.BaseProcessor
	config           *config.Config
	amazonProcessor  *amazon.AmazonProcessor
	taskHandler      *TaskHandler
	pipeline         *pipeline.Pipeline
	managementClient *management.Client
	apiClient        *temu.APIClient
}

// NewTemuProcessor 创建TEMU处理器
func NewTemuProcessor(cfg *config.Config) *TemuProcessor {
	baseProcessor := processor.NewBaseProcessor(cfg)

	// 初始化Amazon处理器（如果启用）
	var amazonProcessor *amazon.AmazonProcessor
	if cfg.Amazon.Enabled {
		amazonProcessor = amazon.NewAmazonProcessor(&cfg.Amazon)
		logrus.Info("[TEMU] Amazon爬虫已启用")
	}

	processor := &TemuProcessor{
		BaseProcessor:    baseProcessor,
		config:           cfg,
		amazonProcessor:  amazonProcessor,
		managementClient: management.NewClient(cfg),
	}

	// 初始化任务处理器和管道
	processor.taskHandler = NewTaskHandler(processor)

	// 为TEMU平台确保Amazon爬虫可用（因为TEMU需要处理Amazon产品）
	amazonConfig := cfg.Amazon
	if !amazonConfig.Enabled {
		logrus.Info("[TEMU] 自动启用Amazon爬虫以支持Amazon产品处理")
		amazonConfig.Enabled = true
		// 设置默认配置
		if amazonConfig.PoolSize == 0 {
			amazonConfig.PoolSize = 1
		}
		if amazonConfig.ViewportWidth == 0 {
			amazonConfig.ViewportWidth = 1920
		}
		if amazonConfig.ViewportHeight == 0 {
			amazonConfig.ViewportHeight = 1080
		}
		amazonConfig.Headless = true // TEMU处理器默认使用无头模式
	}

	processor.pipeline = CreateTEMUPipeline(processor.managementClient, processor.managementClient, &amazonConfig)

	return processor
}

// SetUserToken 设置用户访问令牌
func (p *TemuProcessor) SetUserToken(accessToken, tenantID string) {
	if p.managementClient != nil {
		p.managementClient.SetUserToken(accessToken, tenantID)
		logrus.Infof("[TEMU] 已设置用户令牌到管理系统客户端 (租户: %s)", tenantID)
	}
}

// GetManagementClient 获取管理系统客户端
func (p *TemuProcessor) GetManagementClient() *management.Client {
	return p.managementClient
}

// ProcessTask 处理TEMU任务
func (p *TemuProcessor) ProcessTask(ctx context.Context, task types.Task) error {
	logrus.Infof("[TEMU] 开始处理任务: ID=%s, ProductID=%s, StoreID=%d",
		task.ID, task.ProductID, task.StoreID)

	// TEMU特定的任务处理逻辑
	if err := p.processTemuProduct(ctx, task); err != nil {
		logrus.Errorf("[TEMU] 处理产品失败: %v", err)
		return err
	}

	logrus.Infof("[TEMU] 任务处理完成: ID=%s", task.ID)
	return nil
}

// processTemuProduct 处理TEMU产品
func (p *TemuProcessor) processTemuProduct(ctx context.Context, task types.Task) error {
	logrus.Infof("[TEMU] 处理产品: %s", task.ProductID)

	// 创建动态管道
	dynamicPipeline := p.createDynamicPipeline(task)

	// 使用任务处理器处理任务
	if err := p.taskHandler.ProcessTask(ctx, task, dynamicPipeline); err != nil {
		return fmt.Errorf("任务处理失败: %w", err)
	}

	logrus.Infof("[TEMU] 产品处理完成: %s", task.ProductID)
	return nil
}

// createDynamicPipeline 创建动态管道
func (p *TemuProcessor) createDynamicPipeline(task types.Task) *pipeline.Pipeline {
	// 使用管道构建器创建管道（固定包含Amazon处理）
	builder := NewTemuPipelineBuilder(p.managementClient, p.managementClient, &p.config.Amazon)
	return builder.BuildPipeline()
}

// Close 关闭处理器
func (p *TemuProcessor) Close() {
	logrus.Info("[TEMU] 关闭TEMU任务处理器")

	// 关闭Amazon处理器
	if p.amazonProcessor != nil {
		p.amazonProcessor.Shutdown()
	}

	// 调用基础处理器的关闭方法
	p.BaseProcessor.Close()
}
