// Package modules 提供SHEIN平台销售属性准备的核心处理功能
package sale

import (
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/model"
)

// SaleAttributePreparationHandler 销售属性准备处理器（专门用于属性准备功能）
type SaleAttributePreparationHandler struct {
	productDataPreparer *SaleAttributeProductDataPreparer
	metadataBuilder     *SaleAttributeMetadataBuilder
	variantFilter       *SaleAttributeVariantFilter
	contextBuilder      *SaleAttributeContextBuilder
	requestBuilder      *SaleAttributeRequestBuilder
}

// NewSaleAttributePreparationHandler 创建销售属性准备处理器实例
func NewSaleAttributePreparationHandler() *SaleAttributePreparationHandler {
	return &SaleAttributePreparationHandler{
		productDataPreparer: NewSaleAttributeProductDataPreparer(),
		metadataBuilder:     NewSaleAttributeMetadataBuilder(),
		variantFilter:       NewSaleAttributeVariantFilter(),
		contextBuilder:      NewSaleAttributeContextBuilder(),
		requestBuilder:      NewSaleAttributeRequestBuilder(),
	}
}

// prepareProductsData 准备产品数据（保持原有接口）
func (h *SaleAttributePreparationHandler) prepareProductsData(ctx *model.TaskContext) []map[string]string {
	return h.productDataPreparer.PrepareProductsData(ctx)
}

// buildAttributeMetadata 构建属性元数据（保持原有接口）
func (h *SaleAttributePreparationHandler) buildAttributeMetadata(ctx *model.TaskContext, importanceCalc *model.AttributeImportanceCalculator) []model.AttributeMetadata {
	return h.metadataBuilder.BuildAttributeMetadata(ctx, importanceCalc)
}

// findMappedName 查找映射的属性名称（保持原有接口）
func (h *SaleAttributePreparationHandler) findMappedName(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	return h.metadataBuilder.findMappedName(attrID, attributeTemplates)
}

// buildAttributeNameMappings 构建属性名称映射（保持原有接口）
func (h *SaleAttributePreparationHandler) buildAttributeNameMappings(
	attributeData model.BuildAttributeInfo,
	attributeTemplates *attribute.AttributeTemplateInfo,
) map[int]string {
	return h.metadataBuilder.BuildAttributeNameMappings(attributeData, attributeTemplates)
}

// filterAttributeValuesByActualUsage 根据实际变体值过滤属性候选列表（保持原有接口）
func (h *SaleAttributePreparationHandler) filterAttributeValuesByActualUsage(
	candidateValues []model.GenerateAttributeValue,
) []model.GenerateAttributeValue {
	// 由于新的方法需要更多参数，这里返回原始值以保持向后兼容
	// 实际的筛选逻辑已经在 BuildAttributeMetadata 中处理
	return candidateValues
}

// filterVariantsByRules 在生成销售属性之前过滤变体（保持原有接口）
func (h *SaleAttributePreparationHandler) filterVariantsByRules(ctx *model.TaskContext) {
	h.variantFilter.FilterVariantsByRules(ctx)
}

// filterVariantsByRulesAfterGeneration 在生成销售属性之后过滤变体（保持原有接口）
func (h *SaleAttributePreparationHandler) filterVariantsByRulesAfterGeneration(ctx *model.TaskContext, saleAttributeData *model.ResultSaleAttribute) {
	h.variantFilter.FilterVariantsByRulesAfterGeneration(ctx, saleAttributeData)
}

// buildCompactProductContext 构建精简的产品上下文信息（保持原有接口）
func (h *SaleAttributePreparationHandler) buildCompactProductContext(ctx *model.TaskContext) string {
	return h.contextBuilder.BuildCompactProductContext(*ctx.AmazonProduct, *ctx.Variants)
}

// buildExtraContext 构建额外上下文信息（保持原有接口）
func (h *SaleAttributePreparationHandler) buildExtraContext(ctx *model.TaskContext, productsData []model.ProductVariantData) string {
	return h.contextBuilder.BuildExtraContext(*ctx.AmazonProduct, *ctx.Variants, productsData)
}

// buildGenerationRequest 构建生成请求（保持原有接口）
func (h *SaleAttributePreparationHandler) buildGenerationRequest(
	ctx *model.TaskContext,
	productsData []map[string]string,
	attributeMetadata []model.AttributeMetadata,
	attributeNameMappings map[int]string) *model.GenerationRequest {
	return h.requestBuilder.BuildGenerationRequest(ctx, productsData, attributeMetadata, attributeNameMappings)
}

// buildUserPrompt 构建用户提示词（保持原有接口）
func (h *SaleAttributePreparationHandler) buildUserPrompt(ctx *model.TaskContext, request *model.GenerationRequest) string {
	return h.requestBuilder.BuildUserPrompt(ctx, request)
}

// 为了保持向后兼容性，提供原有SaleAttributeHandler的方法接口
// 这些方法将委托给SaleAttributePreparationHandler实例

var defaultPreparationHandler = NewSaleAttributePreparationHandler()

// prepareProductsData 全局函数，保持向后兼容
func prepareProductsData(ctx *model.TaskContext) []map[string]string {
	return defaultPreparationHandler.prepareProductsData(ctx)
}

// buildAttributeMetadata 全局函数，保持向后兼容
func buildAttributeMetadata(ctx *model.TaskContext, importanceCalc *model.AttributeImportanceCalculator) []model.AttributeMetadata {
	return defaultPreparationHandler.buildAttributeMetadata(ctx, importanceCalc)
}

// findMappedName 全局函数，保持向后兼容
func findMappedName(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	return defaultPreparationHandler.findMappedName(attrID, attributeTemplates)
}

// buildAttributeNameMappings 全局函数，保持向后兼容
func buildAttributeNameMappings(
	attributeData model.BuildAttributeInfo,
	attributeTemplates *attribute.AttributeTemplateInfo,
) map[int]string {
	return defaultPreparationHandler.buildAttributeNameMappings(attributeData, attributeTemplates)
}

// filterAttributeValuesByActualUsage 全局函数，保持向后兼容
func filterAttributeValuesByActualUsage(
	candidateValues []model.GenerateAttributeValue,
) []model.GenerateAttributeValue {
	return defaultPreparationHandler.filterAttributeValuesByActualUsage(candidateValues)
}

// filterVariantsByRules 全局函数，保持向后兼容
func filterVariantsByRules(ctx *model.TaskContext) {
	defaultPreparationHandler.filterVariantsByRules(ctx)
}

// filterVariantsByRulesAfterGeneration 全局函数，保持向后兼容
func filterVariantsByRulesAfterGeneration(ctx *model.TaskContext, saleAttributeData *model.ResultSaleAttribute) {
	defaultPreparationHandler.filterVariantsByRulesAfterGeneration(ctx, saleAttributeData)
}

// buildCompactProductContext 全局函数，保持向后兼容
func buildCompactProductContext(ctx *model.TaskContext) string {
	return defaultPreparationHandler.buildCompactProductContext(ctx)
}

// buildExtraContext 全局函数，保持向后兼容
func buildExtraContext(ctx *model.TaskContext, productsData []model.ProductVariantData) string {
	return defaultPreparationHandler.buildExtraContext(ctx, productsData)
}

// buildGenerationRequest 全局函数，保持向后兼容
func buildGenerationRequest(
	ctx *model.TaskContext,
	productsData []map[string]string,
	attributeMetadata []model.AttributeMetadata,
	attributeNameMappings map[int]string) *model.GenerationRequest {
	return defaultPreparationHandler.buildGenerationRequest(ctx, productsData, attributeMetadata, attributeNameMappings)
}

// buildUserPrompt 全局函数，保持向后兼容
func buildUserPrompt(ctx *model.TaskContext, request *model.GenerationRequest) string {
	return defaultPreparationHandler.buildUserPrompt(ctx, request)
}
