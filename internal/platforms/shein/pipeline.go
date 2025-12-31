package shein

import (
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/platforms/shein/modules"

	"github.com/sirupsen/logrus"
)

// Pipeline 任务处理管道
type Pipeline struct {
	handlers []modules.StepHandler
}

// NewPipeline 创建新的处理管道
func NewPipeline() *Pipeline {
	return &Pipeline{
		handlers: make([]modules.StepHandler, 0),
	}
}

// AddHandler 添加处理器到管道
func (p *Pipeline) AddHandler(handler modules.StepHandler) *Pipeline {
	p.handlers = append(p.handlers, handler)
	return p
}

// Handlers 返回管道中的所有处理器（只读）
func (p *Pipeline) Handlers() []modules.StepHandler {
	// 返回一个副本以防止外部修改
	handlers := make([]modules.StepHandler, len(p.handlers))
	copy(handlers, p.handlers)
	return handlers
}

// Process 执行管道处理
func (p *Pipeline) Process(ctx *modules.TaskContext) error {
	logrus.Infof("开始执行任务处理管道，共 %d 个步骤", len(p.handlers))

	for i, handler := range p.handlers {
		stepNum := i + 1
		logrus.Infof("开始执行步骤 [%d/%d]: %s", stepNum, len(p.handlers), handler.Name())

		if err := handler.Handle(ctx); err != nil {
			// 区分业务过滤和真正的错误
			if modules.IsFilteredError(err) {
				logrus.Infof("✓ 步骤过滤 [%d/%d] [%s]: %v", stepNum, len(p.handlers), handler.Name(), err)
			} else {
				logrus.Errorf("步骤执行失败 [%d/%d] [%s]: %v", stepNum, len(p.handlers), handler.Name(), err)
			}
			return err
		}

		logrus.Infof("步骤执行完成 [%d/%d]: %s", stepNum, len(p.handlers), handler.Name())
	}

	logrus.Info("任务处理管道执行完成")
	return nil
}

// CreateTaskProcessingPipeline 创建任务处理管道
func CreateTaskProcessingPipeline(processor *SheinProcessor, cfg *config.Config) *Pipeline {
	pipeline := NewPipeline()
	openaiConfig := openai.NewClientConfig(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.Model,
		cfg.OpenAI.BaseURL,
		cfg.OpenAI.Timeout,
	)
	// 添加处理步骤
	storeClient := processor.GetManagementClient().GetStoreClient()
	imageDownloder := processor.GetManagementClient().GetImageDownloader()

	// SHEIN平台上架流程（数据可能来自Amazon爬虫或管理系统API）
	// 获取店铺信息
	pipeline.AddHandler(modules.NewStoreInfoHandler(storeClient))
	// 重新上架任务处理器
	//pipeline.AddHandler(modules.NewReListingHandler())
	// 获取原始数据（支持从Amazon爬虫抓取，使用共享的Amazon处理器）
	pipeline.AddHandler(modules.NewRawJsonDataHandler(processor.GetManagementClient().GetRawJsonDataClient(), &cfg.Amazon, processor.amazonProcessor))
	// 早期检查产品是否已上架（避免后续无效处理）
	pipeline.AddHandler(modules.NewProductExistsCheckHandler())
	// 验证图片数量（SHEIN要求至少3张：1张主图+2张细节图）
	pipeline.AddHandler(modules.NewImageValidationHandler(3))
	// 提交原始JSON数据到服务器缓存（使用公共缓存逻辑）
	pipeline.AddHandler(modules.NewSubmitRawJsonDataHandler(processor.GetManagementClient().GetRawJsonDataClient(), &cfg.Amazon, processor.amazonProcessor))
	// 获取店铺API客户端（直接使用具体类型）
	pipeline.AddHandler(modules.NewStoreInfoHandler(
		processor.GetManagementClient().GetStoreClient(),
	))
	// 初始化产品数据
	pipeline.AddHandler(modules.NewInitProductDataHandler())
	// 获取店铺ID
	pipeline.AddHandler(modules.NewSupplierInfoHandler(storeClient))
	// 处理店铺ID
	pipeline.AddHandler(modules.NewStoreIDHandler(storeClient))
	// 检查发品额度
	pipeline.AddHandler(modules.NewSpuLimitHandler())
	// 验证任务（筛选规则和利润规则）
	pipeline.AddHandler(modules.NewTaskValidatorHandler(processor.GetManagementClient()))
	// 应用筛选规则
	pipeline.AddHandler(modules.NewApplyFilterRuleHandler())
	// 查询是否有发品记录
	pipeline.AddHandler(modules.NewHasSpuRecordHandler())
	// 获取所有变体的Json数据（支持从Amazon爬虫抓取，使用共享的Amazon处理器）
	pipeline.AddHandler(modules.NewVariantJsonDataHandler(processor.GetManagementClient().GetRawJsonDataClient(), &cfg.Amazon, processor.amazonProcessor))
	// 提交变体原始JSON数据到服务器缓存（使用公共缓存逻辑）
	pipeline.AddHandler(modules.NewSubmitVariantRawJsonDataHandler(processor.GetManagementClient().GetRawJsonDataClient(), &cfg.Amazon, processor.amazonProcessor))
	// 重新应用筛选规则到变体
	pipeline.AddHandler(modules.NewReapplyFilterRuleHandler())
	// 检查每日上架限制（在获取变体数据后检查，以便准确计算SKC/SKU数量）
	pipeline.AddHandler(modules.NewCheckDailyLimitHandler())
	// 获取分类树
	pipeline.AddHandler(modules.NewGetCategoryTreeHandler())
	// AI选择分类
	pipeline.AddHandler(modules.NewAICategorySelectorHandler(openaiConfig))
	// 获取仓库信息
	pipeline.AddHandler(modules.NewWarehouseInfoHandler())
	// 翻译标题描述
	pipeline.AddHandler(modules.NewTranslateHandler(openaiConfig))
	// 设置站点信息
	pipeline.AddHandler(modules.NewSiteInfoHandler())
	// 获取属性模板
	pipeline.AddHandler(modules.NewAttributeTemplateHandler())
	// 构建属性信息
	pipeline.AddHandler(modules.NewBuildAttributeHandler())
	// AI选择属性
	pipeline.AddHandler(modules.NewAttributeSelectorHandler(openaiConfig))
	// 填充属性
	pipeline.AddHandler(modules.NewFillAttributeHandler())
	// AI生成销售规格
	pipeline.AddHandler(modules.NewSaleAttributeHandler(openaiConfig))
	// 验证修复销售属性
	pipeline.AddHandler(modules.NewValidateRepairSaleAttributeHandler())
	// 构建SKC列表
	pipeline.AddHandler(modules.NewBuildSkcListHandler(imageDownloder))
	// 构建最终的发品数据
	pipeline.AddHandler(modules.NewBuildSpuHandler())
	// 清理敏感词（集成硬编码敏感词检查）
	sensitiveFilter, err := NewSensitiveWordsFilter("data/sensitive_words_shein.json")
	if err != nil {
		logrus.WithError(err).Warn("初始化敏感词过滤器失败，跳过敏感词处理")
	} else {
		// 使用清理模式 - 自动替换敏感词
		pipeline.AddHandler(modules.NewSensitiveWordsCleanHandler(sensitiveFilter))
	}
	// 发布产品
	pipeline.AddHandler(modules.NewPublishProductHandler())
	// 标记变体构建成功
	pipeline.AddHandler(modules.NewMarkVariantPublishSuccessHandler())
	// 错误时收集分类限制及敏感词
	pipeline.AddHandler(modules.NewCollectCategoryRestrictionsHandler())
	// 保存发品成功后返回的信息
	pipeline.AddHandler(modules.NewSavePublishResultHandler())

	return pipeline
}
