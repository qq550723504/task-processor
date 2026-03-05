// Package temu 提供TEMU平台的管道构建器
package temu

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/pipeline"
	commonPipeline "task-processor/internal/pipeline"
	commonHandlers "task-processor/internal/pipeline/handlers"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/handlers"
)

// PipelineBuilder TEMU管道构建器
type PipelineBuilder struct {
	processor *TemuProcessor
}

// NewPipelineBuilder 创建TEMU管道构建器
func NewPipelineBuilder(processor *TemuProcessor) *PipelineBuilder {
	return &PipelineBuilder{
		processor: processor,
	}
}

// BuildPipeline 构建完整的TEMU管道（使用强类型上下文）
func (pb *PipelineBuilder) BuildPipeline() *TemuPipelineExecutor {
	p := pipeline.NewPipeline("TEMU产品发布管道")

	// 获取管理客户端
	managementClient := pb.processor.GetManagementClient()
	if managementClient == nil {
		log := logger.GetGlobalLogger("temu.pipeline_builder")
		log.Error("管理客户端未初始化，无法构建管道")
		return NewTemuPipelineExecutor(p)
	}

	// 添加所有处理器（使用强类型适配器）
	pb.addCommonHandlers(p)   // 0: 公共处理器
	pb.addInitHandlers(p)     // 1-5: 初始化和数据获取
	pb.addFilterHandlers(p)   // 6-12: 筛选和验证
	pb.addCategoryHandlers(p) // 13-19: 分类和SKU处理
	pb.addImageHandlers(p)    // 20-26: 图片处理和AI映射
	pb.addContentHandlers(p)  // 27-29: 内容构建和优化
	pb.addSubmitHandlers(p)   // 30-32: 提交和保存

	// 返回TEMU专用执行器
	return NewTemuPipelineExecutor(p)
}

// addCommonHandlers 添加公共处理器
func (pb *PipelineBuilder) addCommonHandlers(p pipeline.Pipeline) {
	p.AddHandler(commonHandlers.NewInitHandler()). // 0.1 通用初始化
							AddHandler(commonHandlers.NewLoggingHandler("info")). // 0.2 任务日志记录
							AddHandler(commonHandlers.NewValidationHandler(       // 0.3 基础验证
			commonHandlers.NewTaskValidator(),
		))
}

// addInitHandlers 添加初始化阶段处理器（1-6）
func (pb *PipelineBuilder) addInitHandlers(p pipeline.Pipeline) {
	managementClient := pb.processor.GetManagementClient()
	cfg := pb.processor.GetConfig() // 获取完整配置

	p.AddHandler(handlers.NewInitDataHandler()). // 1. 初始化产品数据（TEMU特定）
							AddHandler(handlers.NewStoreInfoHandler(managementClient.GetStoreClient())).                                              // 2. 获取店铺信息（使用公共基类）
							AddHandler(handlers.NewRawJsonDataHandlerV2(managementClient.GetRawJsonDataClient(), cfg, pb.processor.amazonProcessor)). // 3. 获取原始JSON数据（支持分布式）
							AddHandler(NewTemuHandlerAdapter("prohibited_items_detector", handlers.NewProhibitedItemsDetector())).                    // 4. 违禁品检测（TEMU特定）
							AddHandler(handlers.NewCacheProductHandler(managementClient.GetRawJsonDataClient(), cfg, pb.processor.amazonProcessor)).  // 5. 缓存产品数据（支持分布式）
							AddHandler(handlers.NewProductExistsCheckHandler(managementClient.GetProductImportMappingClient()))                       // 6. 产品存在性检查（TEMU特定）
}

// addFilterHandlers 添加筛选和验证阶段处理器（7-13）
func (pb *PipelineBuilder) addFilterHandlers(p pipeline.Pipeline) {
	managementClient := pb.processor.GetManagementClient()
	cfg := pb.processor.GetConfig() // 获取完整配置

	p.AddHandler(handlers.NewFilterRuleHandler(managementClient.GetFilterRuleClient())). // 7. 主产品筛选规则检查
												AddHandler(handlers.NewStoreIDHandler(managementClient.GetStoreClient())).                                                  // 8. 店铺ID检查和保存
												AddHandler(handlers.NewTextCheckHandler()).                                                                                 // 9. 文本检查
												AddHandler(handlers.NewParallelVariantHandler(managementClient.GetRawJsonDataClient(), cfg, pb.processor.amazonProcessor)). // 10. 并行获取变体JSON数据（支持分布式）
												AddHandler(handlers.NewCacheVariantsHandler(managementClient.GetRawJsonDataClient(), cfg, pb.processor.amazonProcessor)).   // 11. 缓存变体数据（支持分布式）
												AddHandler(handlers.NewVariantFilterHandler(managementClient.GetFilterRuleClient())).                                       // 12. 变体筛选规则检查
												AddHandler(handlers.NewCheckDailyLimitHandler(pb.processor.GetMemoryManager()))                                             // 13. 检查每日上架限制
}

// addCategoryHandlers 添加分类和SKU处理阶段处理器（14-20）
func (pb *PipelineBuilder) addCategoryHandlers(p pipeline.Pipeline) {
	p.AddHandler(NewTemuHandlerAdapter("category_recommend", handlers.NewCategoryRecommendHandler())). // 14. 分类推荐
														AddHandler(NewTemuHandlerAdapter("category_disclaim", handlers.NewCategoryDisclaimHandler())). // 15. 分类免责声明
														AddHandler(NewTemuHandlerAdapter("commit_create", handlers.NewCommitCreateHandler())).         // 16. 提交创建
														AddHandler(NewTemuHandlerAdapter("commit_detail", handlers.NewCommitDetailHandler())).         // 17. 提交详情查询
														AddHandler(NewTemuHandlerAdapter("cost_template", handlers.NewCostTemplateHandler())).         // 18. 成本模板
														AddHandler(NewTemuHandlerAdapter("out_goods_sn_check", handlers.NewOutGoodsSnCheckHandler())). // 19. SKU编码重复检查
														AddHandler(NewTemuHandlerAdapter("category", handlers.NewCategoryHandler()))                   // 20. 分类处理
}

// addImageHandlers 添加图片处理阶段处理器（21-25）
func (pb *PipelineBuilder) addImageHandlers(p pipeline.Pipeline) {
	// 创建OpenAI客户端配置
	openaiConfig := openai.NewClientConfig(
		pb.processor.GetConfig().OpenAI.APIKey,
		pb.processor.GetConfig().OpenAI.Model,
		pb.processor.GetConfig().OpenAI.BaseURL,
		pb.processor.GetConfig().OpenAI.Timeout,
	)

	p.AddHandler(NewTemuHandlerAdapter("image_init", handlers.NewImageInitHandler())). // 21. 图片初始化
												AddHandler(NewTemuHandlerAdapter("image_validator", handlers.NewImageValidator())).                // 22. 图片验证（包含白边填充）
												AddHandler(NewTemuHandlerAdapter("image_upload_processor", handlers.NewImageUploadProcessor())).   // 23. 图片上传（包含尺寸标注）
												AddHandler(NewTemuHandlerAdapter("template_query", handlers.NewTemplateQueryHandler())).           // 24. 模板查询
												AddHandler(NewTemuHandlerAdapter("ai_sku_mapping", handlers.NewAISkuMappingHandler(openaiConfig))) // 25. AI SKU映射
}

// addContentHandlers 添加内容构建和优化阶段处理器（26-28）
func (pb *PipelineBuilder) addContentHandlers(p pipeline.Pipeline) {
	managementClient := pb.processor.GetManagementClient()

	// 创建OpenAI客户端配置
	openaiConfig := openai.NewClientConfig(
		pb.processor.GetConfig().OpenAI.APIKey,
		pb.processor.GetConfig().OpenAI.Model,
		pb.processor.GetConfig().OpenAI.BaseURL,
		pb.processor.GetConfig().OpenAI.Timeout,
	)

	p.AddHandler(NewTemuHandlerAdapter("build_spu", handlers.NewBuildSpuHandler(openaiConfig, managementClient.GetProfitRuleClient()))). // 26. 构建SPU（内含AI内容重写并行优化）
		// 27. 并行执行内容验证器
		AddHandler(commonPipeline.NewParallelHandler(
			"内容验证并行处理",
			handlers.NewProductNameValidator(),
			handlers.NewBulletPointsValidator(),
			handlers.NewProductDescriptionValidator(),
			handlers.NewSensitiveWordsFilter(),
		)).
		AddHandler(handlers.NewBrandClearHandler()) // 28. 清除品牌名称
}

// addSubmitHandlers 添加提交和保存阶段处理器（29-31）
func (pb *PipelineBuilder) addSubmitHandlers(p pipeline.Pipeline) {
	managementClient := pb.processor.GetManagementClient()

	p.AddHandler(handlers.NewPriceQueryHandler()). // 29. 价格查询
							AddHandler(handlers.NewProductSubmitHandler(managementClient.GetProductImportMappingClient())).                                     // 30. 产品提交
							AddHandler(handlers.NewSavePublishResultHandler(managementClient.GetProductImportMappingClient(), pb.processor.GetMemoryManager())) // 31. 保存发品结果
}

// =============================================================================
// 通用强类型适配器
// =============================================================================

// TemuHandlerInterface 定义TEMU处理器接口
type TemuHandlerInterface interface {
	Name() string
	HandleTemu(*temucontext.TemuTaskContext) error
}

// NewTemuHandlerAdapter 创建通用的TEMU处理器适配器
func NewTemuHandlerAdapter(name string, temuHandler TemuHandlerInterface) pipeline.Handler {
	return &temuHandlerAdapter{
		name:        name,
		temuHandler: temuHandler,
	}
}

// temuHandlerAdapter 通用的TEMU处理器适配器
type temuHandlerAdapter struct {
	name        string
	temuHandler TemuHandlerInterface
}

// Name 返回处理器名称
func (a *temuHandlerAdapter) Name() string {
	return a.name
}

// Handle 实现pipeline.Handler接口
func (a *temuHandlerAdapter) Handle(ctx pipeline.TaskContext) error {
	// 类型断言转换为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return pipeline.NewHandlerError(a.name, "上下文类型错误：期望 *TemuTaskContext")
	}

	return a.temuHandler.HandleTemu(temuCtx)
}
