package sale

import (
	"task-processor/internal/core/logger"
	sheinattr "task-processor/internal/shein/product/attribute"
)

type SaleAttributeBatchProcessor struct {
	handler *SaleAttributeHandler
}

func NewSaleAttributeBatchProcessor(handler *SaleAttributeHandler) *SaleAttributeBatchProcessor {
	return &SaleAttributeBatchProcessor{handler: handler}
}

func (p *SaleAttributeBatchProcessor) ProcessInBatches(input *SaleAttributeInput, request *sheinattr.GenerationRequest, batchSize int) sheinattr.ResultSaleAttribute {
	variationData := request.VariationData
	productsData := request.ProductsData
	totalBatches := (len(variationData) + batchSize - 1) / batchSize

	var allVariants []sheinattr.Variant
	var allSaleAttributes []sheinattr.ResultAttribute

	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		start := batchIndex * batchSize
		end := start + batchSize
		if end > len(variationData) {
			end = len(variationData)
		}

		var batchProductsData []sheinattr.ProductVariantData
		if start < len(productsData) {
			productsEnd := end
			if productsEnd > len(productsData) {
				productsEnd = len(productsData)
			}
			batchProductsData = productsData[start:productsEnd]
		} else {
			batchProductsData = []sheinattr.ProductVariantData{}
		}

		batchRequest := &sheinattr.GenerationRequest{
			ProductsData:             batchProductsData,
			VariationData:            variationData[start:end],
			VariationAttributeValues: request.VariationAttributeValues,
			SaleAttributesData:       request.SaleAttributesData,
			AttributeMappings:        request.AttributeMappings,
			RequiredVariantCount:     end - start,
		}

		logger.GetGlobalLogger("shein/product").Infof("process sale attribute batch %d/%d", batchIndex+1, totalBatches)
		singleProcessor := NewSaleAttributeSingleProcessor(p.handler)
		batchResult := singleProcessor.ProcessSingleBatch(input, batchRequest)
		allVariants = append(allVariants, batchResult.Variants...)

		for _, saleAttr := range batchResult.SaleAttributes {
			exists := false
			for _, existing := range allSaleAttributes {
				if existing.AttrID == saleAttr.AttrID {
					exists = true
					break
				}
			}
			if !exists {
				allSaleAttributes = append(allSaleAttributes, saleAttr)
			}
		}
	}

	return sheinattr.ResultSaleAttribute{SaleAttributes: allSaleAttributes, Variants: allVariants}
}
