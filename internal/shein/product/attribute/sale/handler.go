package sale

import (
	"fmt"

	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein/aicache"
	"task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
	sheinattr "task-processor/internal/shein/product/attribute"
)

type SaleAttributeHandler struct {
	openaiClient       openaiClient.ChatCompleter
	preparationHandler *SaleAttributePreparationHandler
}

func NewSaleAttributeHandler(client openaiClient.ChatCompleter) *SaleAttributeHandler {
	return &SaleAttributeHandler{openaiClient: client, preparationHandler: NewSaleAttributePreparationHandler()}
}

func (h *SaleAttributeHandler) Name() string { return "sale_attribute" }

func (h *SaleAttributeHandler) Handle(ctx *sheinctx.TaskContext) error {
	if ctx.AttributeTemplates == nil {
		return fmt.Errorf("attribute templates are not initialized")
	}
	if ctx.ProductData == nil {
		return fmt.Errorf("product data is not initialized")
	}
	if err := h.generateSaleSpec(ctx); err != nil {
		return fmt.Errorf("generate sale attributes failed: %w", err)
	}
	return nil
}

func (h *SaleAttributeHandler) generateSaleSpec(ctx *sheinctx.TaskContext) error {
	logger.GetGlobalLogger("shein/product").Debug("start generating sale attributes")

	cacheKey := fmt.Sprintf("%s:%d", ctx.AmazonProduct.ParentAsin, ctx.ProductData.CategoryID)
	if ctx.AICache != nil {
		var cached sheinattr.ResultSaleAttribute
		if ctx.AICache.Get(aicache.TypeSaleAttr, cacheKey, &cached) {
			if !h.cachedSaleSpecMatchesCurrentVariants(ctx.Variants, cached) {
				logger.GetGlobalLogger("shein/product").WithFields(map[string]any{
					"cache_key":          cacheKey,
					"category_id":        ctx.ProductData.CategoryID,
					"cached_variants":    len(cached.Variants),
					"requested_variants": len(ctx.FilteredVariants()),
				}).Warn("sale attribute cache stale for current variants, regenerating")
			} else {
				cached = h.filterValidASINs(ctx.Variants, cached)
				output := NewSaleAttributeOutput(cached)
				ctx.SetSaleSpecResult(&output.Result)
				logger.GetGlobalLogger("shein/product").WithFields(map[string]any{
					"source":          "cache",
					"category_id":     ctx.ProductData.CategoryID,
					"variant_count":   output.VariantCount,
					"sale_attr_count": output.SaleAttributeCount,
					"requested_count": len(ctx.FilteredVariants()),
				}).Info("sale attributes ready")
				return nil
			}
		}
	}

	config := h.defaultAttributeConfig()
	importanceCalc := h.newAttributeImportanceCalculator(&config.ImportanceRules)
	h.filterVariantsByRules(ctx)
	input := newSaleAttributeInput(ctx)

	productsData := h.prepareProductsData(ctx)
	if len(productsData) == 0 {
		return fmt.Errorf("products data is empty")
	}
	attributeMetadata := h.buildAttributeMetadata(ctx, importanceCalc)
	if len(attributeMetadata) == 0 {
		return fmt.Errorf("attribute metadata is empty")
	}

	attributeNameMappings := h.buildAttributeNameMappings(*ctx.BuildAttributeData, ctx.AttributeTemplates)
	request := h.buildGenerationRequest(input, productsData, attributeMetadata, attributeNameMappings)
	saleAttributeData := h.callGPTAPI(input, request)
	if saleAttributeData.SaleAttributes == nil {
		return fmt.Errorf("GPT did not return sale attributes")
	}
	if len(saleAttributeData.Variants) == 0 {
		return fmt.Errorf("GPT did not return variants")
	}

	saleAttributeData = h.filterValidASINs(ctx.Variants, saleAttributeData)
	saleAttributeData = h.validateAndFixSaleAttributeData(saleAttributeData, productsData)
	saleAttributeData = h.validateAttributeValueConsistency(*ctx.AmazonProduct, saleAttributeData)
	h.compareAttributeDataDifferences(*ctx.AmazonProduct, saleAttributeData)
	if len(saleAttributeData.Variants) == 0 {
		return fmt.Errorf("no valid variants remain after validation")
	}

	output := NewSaleAttributeOutput(saleAttributeData)
	ctx.SetSaleSpecResult(&output.Result)
	if ctx.AICache != nil {
		ctx.AICache.Set(aicache.TypeSaleAttr, cacheKey, output.Result)
	}
	logger.GetGlobalLogger("shein/product").WithFields(map[string]any{
		"source":          "generated",
		"category_id":     ctx.ProductData.CategoryID,
		"variant_count":   output.VariantCount,
		"sale_attr_count": output.SaleAttributeCount,
		"requested_count": len(productsData),
		"batched":         len(request.VariationData) > 20,
	}).Info("sale attributes ready")
	return nil
}

func (h *SaleAttributeHandler) defaultAttributeConfig() *sheinattr.AttributeConfig {
	return &sheinattr.AttributeConfig{ImportanceRules: sheinattr.ImportanceRules{RemarkListScore: 100, RequiredScore: 80, SampleScore: 40, ActiveScore: 30, DisplayScore: 20}}
}

func (h *SaleAttributeHandler) newAttributeImportanceCalculator(rules *sheinattr.ImportanceRules) *sheinattr.AttributeImportanceCalculator {
	return sheinattr.NewAttributeImportanceCalculatorWithRules(rules)
}

func (h *SaleAttributeHandler) createChatCompletionRequest(systemPrompt, userPrompt string, variantCount int) *openaiClient.ChatCompletionRequest {
	seed := 42
	temperature := float32(0.1)
	return &openaiClient.ChatCompletionRequest{
		Model:          h.openaiClient.GetDefaultModel(),
		Messages:       []openaiClient.ChatCompletionMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}},
		Temperature:    &temperature,
		Seed:           &seed,
		ResponseFormat: "json_object",
	}
}

func (h *SaleAttributeHandler) prepareProductsData(ctx *sheinctx.TaskContext) []map[string]string {
	return h.preparationHandler.prepareProductsData(ctx)
}

func (h *SaleAttributeHandler) buildAttributeMetadata(ctx *sheinctx.TaskContext, importanceCalc *sheinattr.AttributeImportanceCalculator) []sheinattr.AttributeMetadata {
	return h.preparationHandler.buildAttributeMetadata(ctx, importanceCalc)
}

func (h *SaleAttributeHandler) buildAttributeNameMappings(attributeData sheinattr.BuildAttributeInfo, attributeTemplates *attribute.AttributeTemplateInfo) map[int]string {
	return h.preparationHandler.buildAttributeNameMappings(attributeData, attributeTemplates)
}

func (h *SaleAttributeHandler) buildGenerationRequest(input *SaleAttributeInput, productsData []map[string]string, attributeMetadata []sheinattr.AttributeMetadata, attributeNameMappings map[int]string) *sheinattr.GenerationRequest {
	return h.preparationHandler.buildGenerationRequest(input, productsData, attributeMetadata, attributeNameMappings)
}

func (h *SaleAttributeHandler) filterVariantsByRules(ctx *sheinctx.TaskContext) {
	h.preparationHandler.filterVariantsByRules(ctx)
}

func (h *SaleAttributeHandler) buildUserPrompt(input *SaleAttributeInput, request *sheinattr.GenerationRequest) string {
	return h.preparationHandler.buildUserPrompt(input, request)
}
