// Package temu 提供TEMU平台的管道处理器注册表
package temu

import (
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/pipeline"
	commonPipeline "task-processor/internal/pipeline"
	commonHandlers "task-processor/internal/pipeline/handlers"
	"task-processor/internal/temu/category"
	"task-processor/internal/temu/filter"
	"task-processor/internal/temu/handlerbase"
	"task-processor/internal/temu/image"
	"task-processor/internal/temu/product"
	"task-processor/internal/temu/rules"
	"task-processor/internal/temu/sku"
	"task-processor/internal/temu/store"
	"task-processor/internal/temu/template"
)

// HandlerRegistryEntry 处理器注册表条目
type HandlerRegistryEntry struct {
	Name    string
	Handler pipeline.Handler
}

// PipelineRegistry 管道处理器注册表
type PipelineRegistry struct {
	processor *TemuProcessor
	entries   []HandlerRegistryEntry
}

// NewPipelineRegistry 创建管道处理器注册表
func NewPipelineRegistry(processor *TemuProcessor) *PipelineRegistry {
	return &PipelineRegistry{
		processor: processor,
		entries:   make([]HandlerRegistryEntry, 0, 35),
	}
}

// RegisterAll 注册所有处理器
func (pr *PipelineRegistry) RegisterAll() {
	pr.registerCommonHandlers()
	pr.registerInitHandlers()
	pr.registerFilterHandlers()
	pr.registerCategoryHandlers()
	pr.registerImageHandlers()
	pr.registerContentHandlers()
	pr.registerSubmitHandlers()
}

// GetHandlers 获取所有注册的处理器
func (pr *PipelineRegistry) GetHandlers() []pipeline.Handler {
	handlers := make([]pipeline.Handler, len(pr.entries))
	for i, entry := range pr.entries {
		handlers[i] = entry.Handler
	}
	return handlers
}

// register 注册单个处理器
func (pr *PipelineRegistry) register(name string, handler pipeline.Handler) {
	pr.entries = append(pr.entries, HandlerRegistryEntry{
		Name:    name,
		Handler: handler,
	})
}

// registerCommonHandlers 注册公共处理器
func (pr *PipelineRegistry) registerCommonHandlers() {
	pr.register("init", commonHandlers.NewInitHandler())
	pr.register("logging", commonHandlers.NewLoggingHandler("info"))
	pr.register("validation", commonHandlers.NewValidationHandler(commonHandlers.NewTaskValidator()))
}

// registerInitHandlers 注册初始化阶段处理器
func (pr *PipelineRegistry) registerInitHandlers() {
	managementClient := pr.processor.GetManagementClient()
	cfg := pr.processor.GetConfig()
	rabbitmqClient := pr.processor.rabbitmqClient

	pr.register("init_data", handlerbase.NewInitDataHandler())
	pr.register("store_info", store.NewStoreInfoHandler(managementClient.GetStoreClient()))
	pr.register("raw_json_data", product.NewRawJsonDataHandlerV2(managementClient.GetRawJsonDataAdapter(), cfg, pr.processor.amazonProcessor, rabbitmqClient))
	pr.register("prohibited_items", NewTemuHandlerAdapter("prohibited_items_detector", filter.NewProhibitedItemsDetector()))
	pr.register("cache_product", product.NewCacheProductHandler(managementClient.GetRawJsonDataAdapter(), cfg, pr.processor.amazonProcessor, rabbitmqClient))
	pr.register("product_exists", product.NewProductExistsCheckHandler(managementClient.GetProductImportMappingClient()))
}

// registerFilterHandlers 注册筛选和验证阶段处理器
func (pr *PipelineRegistry) registerFilterHandlers() {
	managementClient := pr.processor.GetManagementClient()
	cfg := pr.processor.GetConfig()
	rabbitmqClient := pr.processor.rabbitmqClient

	pr.register("filter_rule", filter.NewFilterRuleHandler(managementClient.GetFilterRuleClient()))
	pr.register("store_id", store.NewStoreIDHandler(managementClient.GetStoreClient()))
	pr.register("text_check", rules.NewTextCheckHandler())
	pr.register("parallel_variant", sku.NewParallelVariantHandler(managementClient.GetRawJsonDataAdapter(), cfg, pr.processor.amazonProcessor, rabbitmqClient))
	pr.register("cache_variants", sku.NewCacheVariantsHandler(managementClient.GetRawJsonDataAdapter(), cfg, pr.processor.amazonProcessor, rabbitmqClient))
	pr.register("variant_filter", sku.NewVariantFilterHandler(managementClient.GetFilterRuleClient()))
	pr.register("daily_limit", product.NewCheckDailyLimitHandler(pr.processor.GetMemoryManager()))
}

// registerCategoryHandlers 注册分类和SKU处理阶段处理器
func (pr *PipelineRegistry) registerCategoryHandlers() {
	pr.register("category_recommend", NewTemuHandlerAdapter("category_recommend", category.NewCategoryRecommendHandler()))
	pr.register("category_disclaim", NewTemuHandlerAdapter("category_disclaim", category.NewCategoryDisclaimHandler()))
	pr.register("commit_create", NewTemuHandlerAdapter("commit_create", product.NewCommitCreateHandler()))
	pr.register("commit_detail", NewTemuHandlerAdapter("commit_detail", product.NewCommitDetailHandler()))
	pr.register("cost_template", NewTemuHandlerAdapter("cost_template", template.NewCostTemplateHandler()))
	pr.register("out_goods_sn_check", NewTemuHandlerAdapter("out_goods_sn_check", product.NewOutGoodsSnCheckHandler()))
	pr.register("category", NewTemuHandlerAdapter("category", category.NewCategoryHandler()))
}

// registerImageHandlers 注册图片处理阶段处理器
func (pr *PipelineRegistry) registerImageHandlers() {
	openaiConfig := pr.createOpenAIConfig()

	pr.register("image_init", NewTemuHandlerAdapter("image_init", image.NewImageInitHandler()))
	pr.register("image_validator", NewTemuHandlerAdapter("image_validator", image.NewImageValidator()))
	pr.register("image_upload", NewTemuHandlerAdapter("image_upload_processor", image.NewImageUploadProcessor()))
	pr.register("template_query", NewTemuHandlerAdapter("template_query", template.NewTemplateQueryHandler()))
	pr.register("ai_sku_mapping", NewTemuHandlerAdapter("ai_sku_mapping", sku.NewAISkuMappingHandler(openaiConfig)))
}

// registerContentHandlers 注册内容构建和优化阶段处理器
func (pr *PipelineRegistry) registerContentHandlers() {
	managementClient := pr.processor.GetManagementClient()
	openaiConfig := pr.createOpenAIConfig()
	aiClient := openai.NewClient(openaiConfig)

	skuBuilder := sku.NewSkuBuilder(nil, aiClient, managementClient.GetProfitRuleClient())
	specHandler := skuBuilder.GetSpecHandler()

	pr.register("build_spu", NewTemuHandlerAdapter("build_spu", product.NewBuildSpuHandler(openaiConfig, managementClient.GetProfitRuleClient(), skuBuilder, specHandler)))
	pr.register("content_validation", commonPipeline.NewParallelHandler(
		"内容验证并行处理",
		product.NewProductNameValidator(),
		rules.NewBulletPointsValidator(),
		product.NewProductDescriptionValidator(),
		filter.NewSensitiveWordsFilter(),
	))
	pr.register("brand_clear", product.NewBrandClearHandler())
}

// registerSubmitHandlers 注册提交和保存阶段处理器
func (pr *PipelineRegistry) registerSubmitHandlers() {
	managementClient := pr.processor.GetManagementClient()

	pr.register("price_query", product.NewPriceQueryHandler())
	pr.register("product_submit", product.NewProductSubmitHandler(managementClient.GetProductImportMappingClient()))
	pr.register("save_result", product.NewSavePublishResultHandler(managementClient.GetProductImportMappingClient(), pr.processor.GetMemoryManager()))
}

// createOpenAIConfig 创建OpenAI配置
func (pr *PipelineRegistry) createOpenAIConfig() *openai.ClientConfig {
	return pr.processor.GetConfig().OpenAI.ToClientConfig()
}
