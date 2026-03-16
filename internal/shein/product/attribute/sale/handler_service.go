package sale

import (
	"fmt"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// SaleAttributeHandler 销售属性处理器
type SaleAttributeHandler struct {
	openaiClient       *openaiClient.Client
	preparationHandler *SaleAttributePreparationHandler
}

// NewSaleAttributeHandler 创建新的销售属性处理器
func NewSaleAttributeHandler(config *openaiClient.ClientConfig) *SaleAttributeHandler {
	return &SaleAttributeHandler{
		openaiClient:       openaiClient.NewClient(config),
		preparationHandler: NewSaleAttributePreparationHandler(),
	}
}

// Name 返回处理器名称
func (h *SaleAttributeHandler) Name() string {
	return "生成销售规格"
}

// Handle 执行生成销售规格处理
func (h *SaleAttributeHandler) Handle(ctx *shein.TaskContext) error {
	// 检查是否已获取属性模板
	if ctx.AttributeTemplates == nil {
		return fmt.Errorf("属性模板未获取，请先执行获取属性模板步骤")
	}

	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}

	// 为每个SKC生成销售属性
	if err := h.generateSaleSpec(ctx); err != nil {
		return fmt.Errorf("生成销售属性失败: %w", err)
	}

	return nil
}

// generateSaleSpec 生成销售规格
func (h *SaleAttributeHandler) generateSaleSpec(ctx *shein.TaskContext) error {
	logrus.Info("🚀 开始生成销售规格")

	// 1. 初始化配置
	config := h.defaultAttributeConfig()
	importanceCalc := h.newAttributeImportanceCalculator(&config.ImportanceRules)

	// 2. 过滤变体（只执行一次）
	h.filterVariantsByRules(ctx)

	// 3. 准备产品数据
	productsData := h.prepareProductsData(ctx)
	if len(productsData) == 0 {
		return fmt.Errorf("产品数据为空，无法生成销售属性")
	}
	logrus.Infof("✅ 准备了 %d 个产品数据", len(productsData))

	// 4. 构建属性元数据
	attributeMetadata := h.buildAttributeMetadata(ctx, importanceCalc)
	if len(attributeMetadata) == 0 {
		return fmt.Errorf("属性元数据为空，无法生成销售属性")
	}
	logrus.Infof("✅ 构建了 %d 个属性元数据", len(attributeMetadata))

	// 5. 构建属性名称映射
	attributeNameMappings := h.buildAttributeNameMappings(*ctx.BuildAttributeData, ctx.AttributeTemplates)

	// 6. 生成AI请求
	request := h.buildGenerationRequest(ctx, productsData, attributeMetadata, attributeNameMappings)

	// 8. 调用GPT API
	logrus.Info("📡 调用GPT API生成销售属性...")
	saleAttributeData := h.callGPTAPI(ctx, request)

	if saleAttributeData.SaleAttributes == nil {
		return fmt.Errorf("GPT API生成销售属性失败")
	}

	// 9. 验证AI返回的数据
	if len(saleAttributeData.Variants) == 0 {
		return fmt.Errorf("AI未生成任何变体数据")
	}
	logrus.Infof("✅ AI生成了 %d 个变体", len(saleAttributeData.Variants))

	// 10. 过滤有效ASIN（移除AI生成的多余ASIN）
	saleAttributeData = h.filterValidASINs(ctx.Variants, saleAttributeData)

	// 11. 验证并修复数据质量
	saleAttributeData = h.validateAndFixSaleAttributeData(saleAttributeData, productsData)

	// 12. 验证属性值一致性
	saleAttributeData = h.validateAttributeValueConsistency(*ctx.AmazonProduct, saleAttributeData)

	// 13. 对比AI生成前后的属性数据差异（用于质量监控）
	h.compareAttributeDataDifferences(*ctx.AmazonProduct, saleAttributeData)

	// 14. 最终验证
	if len(saleAttributeData.Variants) == 0 {
		return fmt.Errorf("经过验证和修复后，没有有效的变体数据")
	}

	// 15. 保存结果
	ctx.SaleSpecResult = &saleAttributeData
	logrus.Infof("✅ 销售规格生成完成，共 %d 个变体，%d 个销售属性",
		len(saleAttributeData.Variants), len(saleAttributeData.SaleAttributes))

	return nil
}

// defaultAttributeConfig 返回默认属性配置
func (h *SaleAttributeHandler) defaultAttributeConfig() *shein.AttributeConfig {
	return &shein.AttributeConfig{
		ImportanceRules: shein.ImportanceRules{
			RemarkListScore: 100,
			RequiredScore:   80,
			SampleScore:     40,
			ActiveScore:     30,
			DisplayScore:    20,
		},
	}
}

// newAttributeImportanceCalculator 创建属性重要性计算器
func (h *SaleAttributeHandler) newAttributeImportanceCalculator(rules *shein.ImportanceRules) *shein.AttributeImportanceCalculator {
	// 使用 model 包提供的带自定义规则的构造函数
	return shein.NewAttributeImportanceCalculatorWithRules(rules)
}

// createChatCompletionRequest 创建聊天完成请求
func (h *SaleAttributeHandler) createChatCompletionRequest(systemPrompt, userPrompt string, variantCount int) *openaiClient.ChatCompletionRequest {
	seed := 42
	temperature := float32(0.1)

	logrus.Infof("📊 创建请求 (变体数=%d, 不限制MaxTokens)", variantCount)

	return &openaiClient.ChatCompletionRequest{
		Model: h.openaiClient.GetDefaultModel(),
		Messages: []openaiClient.ChatCompletionMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: &temperature,
		Seed:        &seed,
	}
}

// 以下方法委托给preparationHandler，保持原有业务逻辑

// prepareProductsData 准备产品数据
func (h *SaleAttributeHandler) prepareProductsData(ctx *shein.TaskContext) []map[string]string {
	return h.preparationHandler.prepareProductsData(ctx)
}

// buildAttributeMetadata 构建属性元数据
func (h *SaleAttributeHandler) buildAttributeMetadata(ctx *shein.TaskContext, importanceCalc *shein.AttributeImportanceCalculator) []shein.AttributeMetadata {
	return h.preparationHandler.buildAttributeMetadata(ctx, importanceCalc)
}

// buildAttributeNameMappings 构建属性名称映射
func (h *SaleAttributeHandler) buildAttributeNameMappings(
	attributeData shein.BuildAttributeInfo,
	attributeTemplates *attribute.AttributeTemplateInfo,
) map[int]string {
	return h.preparationHandler.buildAttributeNameMappings(attributeData, attributeTemplates)
}

// buildGenerationRequest 构建生成请求
func (h *SaleAttributeHandler) buildGenerationRequest(
	ctx *shein.TaskContext,
	productsData []map[string]string,
	attributeMetadata []shein.AttributeMetadata,
	attributeNameMappings map[int]string) *shein.GenerationRequest {
	return h.preparationHandler.buildGenerationRequest(ctx, productsData, attributeMetadata, attributeNameMappings)
}

// filterVariantsByRules 在生成销售属性之前过滤变体
func (h *SaleAttributeHandler) filterVariantsByRules(ctx *shein.TaskContext) {
	h.preparationHandler.filterVariantsByRules(ctx)
}

// buildUserPrompt 构建用户提示词
func (h *SaleAttributeHandler) buildUserPrompt(ctx *shein.TaskContext, request *shein.GenerationRequest) string {
	return h.preparationHandler.buildUserPrompt(ctx, request)
}
