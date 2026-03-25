package sale

import (
	"task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
	sheinattr "task-processor/internal/shein/product/attribute"
)

type SaleAttributePreparationHandler struct {
	productDataPreparer *SaleAttributeProductDataPreparer
	metadataBuilder     *SaleAttributeMetadataBuilder
	variantFilter       *SaleAttributeVariantFilter
	contextBuilder      *SaleAttributeContextBuilder
	requestBuilder      *SaleAttributeRequestBuilder
}

func NewSaleAttributePreparationHandler() *SaleAttributePreparationHandler {
	return &SaleAttributePreparationHandler{
		productDataPreparer: NewSaleAttributeProductDataPreparer(),
		metadataBuilder:     NewSaleAttributeMetadataBuilder(),
		variantFilter:       NewSaleAttributeVariantFilter(),
		contextBuilder:      NewSaleAttributeContextBuilder(),
		requestBuilder:      NewSaleAttributeRequestBuilder(),
	}
}

func (h *SaleAttributePreparationHandler) prepareProductsData(ctx *sheinctx.TaskContext) []map[string]string {
	return h.productDataPreparer.PrepareProductsData(ctx)
}

func (h *SaleAttributePreparationHandler) buildAttributeMetadata(ctx *sheinctx.TaskContext, importanceCalc *sheinattr.AttributeImportanceCalculator) []sheinattr.AttributeMetadata {
	return h.metadataBuilder.BuildAttributeMetadata(ctx, importanceCalc)
}

func (h *SaleAttributePreparationHandler) findMappedName(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	return h.metadataBuilder.findMappedName(attrID, attributeTemplates)
}

func (h *SaleAttributePreparationHandler) buildAttributeNameMappings(attributeData sheinattr.BuildAttributeInfo, attributeTemplates *attribute.AttributeTemplateInfo) map[int]string {
	return h.metadataBuilder.BuildAttributeNameMappings(attributeData, attributeTemplates)
}

func (h *SaleAttributePreparationHandler) filterAttributeValuesByActualUsage(candidateValues []sheinattr.GenerateAttributeValue) []sheinattr.GenerateAttributeValue {
	return candidateValues
}

func (h *SaleAttributePreparationHandler) filterVariantsByRules(ctx *sheinctx.TaskContext) {
	h.variantFilter.FilterVariantsByRules(ctx)
}

func (h *SaleAttributePreparationHandler) filterVariantsByRulesAfterGeneration(ctx *sheinctx.TaskContext, saleAttributeData *sheinattr.ResultSaleAttribute) {
	h.variantFilter.FilterVariantsByRulesAfterGeneration(ctx, saleAttributeData)
}

func (h *SaleAttributePreparationHandler) buildGenerationRequest(input *SaleAttributeInput, productsData []map[string]string, attributeMetadata []sheinattr.AttributeMetadata, attributeNameMappings map[int]string) *sheinattr.GenerationRequest {
	return h.requestBuilder.BuildGenerationRequest(input, productsData, attributeMetadata, attributeNameMappings)
}

func (h *SaleAttributePreparationHandler) buildUserPrompt(input *SaleAttributeInput, request *sheinattr.GenerationRequest) string {
	return h.requestBuilder.BuildUserPrompt(input, request)
}
