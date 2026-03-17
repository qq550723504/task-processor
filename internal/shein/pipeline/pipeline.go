package pipeline

import (
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein"
	"task-processor/internal/shein/category"
	"task-processor/internal/shein/content"
	"task-processor/internal/shein/product"
	"task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/attribute/sale"
	"task-processor/internal/shein/product/build"
	"task-processor/internal/shein/product/image"
	"task-processor/internal/shein/productdata"
	"task-processor/internal/shein/publish"
	"task-processor/internal/shein/store"
	"task-processor/internal/shein/translate"
	"task-processor/internal/shein/validation"

	"github.com/sirupsen/logrus"
)

// Pipeline 任务处理管道
type Pipeline struct {
	handlers []shein.StepHandler
}

// NewPipeline 创建新的处理管道
func NewPipeline() *Pipeline {
	return &Pipeline{
		handlers: make([]shein.StepHandler, 0),
	}
}

// AddHandler 添加处理器到管道
func (p *Pipeline) AddHandler(handler shein.StepHandler) *Pipeline {
	p.handlers = append(p.handlers, handler)
	return p
}

// Handlers 返回管道中的所有处理器（只读）
func (p *Pipeline) Handlers() []shein.StepHandler {
	// 返回一个副本以防止外部修改
	handlers := make([]shein.StepHandler, len(p.handlers))
	copy(handlers, p.handlers)
	return handlers
}

// Process 执行管道处理
func (p *Pipeline) Process(ctx *shein.TaskContext) error {
	logrus.Infof("开始执行任务处理管道，共 %d 个步骤", len(p.handlers))

	for i, handler := range p.handlers {
		stepNum := i + 1
		logrus.Infof("开始执行步骤 [%d/%d]: %s", stepNum, len(p.handlers), handler.Name())

		if err := handler.Handle(ctx); err != nil {
			// 区分业务过滤和真正的错误
			if shein.IsFilteredError(err) {
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
	pipeline.AddHandler(store.NewStoreInfoHandler(storeClient))
	// 重新上架任务处理器
	//pipeline.AddHandler(modules.NewReListingHandler())
	// 获取原始数据（支持从Amazon爬虫抓取，使用共享的Amazon处理器）
	pipeline.AddHandler(productdata.NewRawJsonDataHandler(processor.GetManagementClient().GetRawJsonDataAdapter(), cfg, processor.amazonProcessor, processor.rabbitmqClient))
	// 早期检查产品是否已上架（避免后续无效处理）
	pipeline.AddHandler(publish.NewProductExistsCheckHandler())
	// 验证图片数量（SHEIN要求至少3张：1张主图+2张细节图）
	pipeline.AddHandler(image.NewImageValidationHandler(3))
	// 提交原始JSON数据到服务器缓存（使用公共缓存逻辑）
	pipeline.AddHandler(productdata.NewSubmitRawJsonDataHandler(processor.GetManagementClient().GetRawJsonDataAdapter(), cfg, processor.amazonProcessor, processor.rabbitmqClient))
	// 初始化产品数据
	pipeline.AddHandler(product.NewInitProductDataHandler())
	// 获取店铺ID
	pipeline.AddHandler(store.NewSupplierInfoHandler(storeClient))
	// 处理店铺ID
	pipeline.AddHandler(store.NewStoreIDHandler(storeClient))
	// 检查发品额度
	//pipeline.AddHandler(product.NewSpuLimitHandler())
	// 检查SKC上架额度
	pipeline.AddHandler(product.NewShelfQuotaHandler())
	// 验证任务（筛选规则和利润规则）
	pipeline.AddHandler(validation.NewTaskValidatorHandler(processor.GetManagementClient()))
	// 应用筛选规则
	pipeline.AddHandler(validation.NewApplyFilterRuleHandler())
	// 查询是否有发品记录
	pipeline.AddHandler(product.NewHasSpuRecordHandler())
	// 获取所有变体的Json数据（支持从Amazon爬虫抓取，使用共享的Amazon处理器）
	pipeline.AddHandler(productdata.NewVariantJsonDataHandler(processor.GetManagementClient().GetRawJsonDataAdapter(), &cfg.Amazon, processor.amazonProcessor, processor.rabbitmqClient))
	// 提交变体原始JSON数据到服务器缓存（使用公共缓存逻辑）
	pipeline.AddHandler(productdata.NewSubmitVariantRawJsonDataHandler(processor.GetManagementClient().GetRawJsonDataAdapter(), cfg, processor.amazonProcessor, processor.rabbitmqClient))
	// 重新应用筛选规则到变体
	pipeline.AddHandler(validation.NewReapplyFilterRuleHandler())
	// 检查每日上架限制（在获取变体数据后检查，以便准确计算SKC/SKU数量）
	pipeline.AddHandler(validation.NewCheckDailyLimitHandler())
	// 获取分类树
	pipeline.AddHandler(category.NewGetCategoryTreeHandler())
	// AI选择分类
	pipeline.AddHandler(category.NewAICategorySelectorHandler(openaiConfig))
	// 获取仓库信息
	pipeline.AddHandler(store.NewWarehouseInfoHandler())
	// 翻译标题描述
	pipeline.AddHandler(translate.NewTranslateHandler(openaiConfig))
	// 设置站点信息
	pipeline.AddHandler(store.NewSiteInfoHandler())
	// 获取属性模板
	pipeline.AddHandler(attribute.NewAttributeTemplateHandler())
	// 构建属性信息
	pipeline.AddHandler(build.NewBuildAttributeHandler())
	// AI选择属性
	pipeline.AddHandler(attribute.NewAttributeSelectorHandler(openaiConfig))
	// 填充属性
	pipeline.AddHandler(attribute.NewFillAttributeHandler())
	// AI生成销售规格
	pipeline.AddHandler(sale.NewSaleAttributeHandler(openaiConfig))
	// 验证修复销售属性
	pipeline.AddHandler(attribute.NewValidateRepairSaleAttributeHandler())
	// 构建SKC列表
	pipeline.AddHandler(build.NewBuildSkcListHandler(imageDownloder, openaiConfig))
	// 构建最终的发品数据
	pipeline.AddHandler(build.NewBuildSpuHandler())
	// 清理敏感词（集成硬编码敏感词检查）
	sensitiveFilter, err := shein.NewSensitiveWordsFilter("data/sensitive_words_shein.json")
	if err != nil {
		logrus.WithError(err).Warn("初始化敏感词过滤器失败，跳过敏感词处理")
	} else {
		// 使用清理模式 - 自动替换敏感词
		pipeline.AddHandler(content.NewSensitiveWordsCleanHandler(sensitiveFilter))
	}
	// 发布产品
	pipeline.AddHandler(publish.NewPublishProductHandler())
	// 标记变体构建成功
	pipeline.AddHandler(publish.NewMarkVariantPublishSuccessHandler())
	// 错误时收集分类限制及敏感词
	pipeline.AddHandler(category.NewCollectCategoryRestrictionsHandler())
	// 保存发品成功后返回的信息
	pipeline.AddHandler(publish.NewSavePublishResultHandler())

	return pipeline
}
