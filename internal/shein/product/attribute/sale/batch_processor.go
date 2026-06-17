package sale

import (
	"fmt"
	"strings"

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
		batchNumber := batchIndex + 1
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
			VariationAttributeValues: variationAttributeValuesPointer(scopeVariationAttributeValuesToProductsData(request.VariationAttributeValues, batchProductsData)),
			SaleAttributesData:       request.SaleAttributesData,
			AttributeMappings:        request.AttributeMappings,
			RequiredVariantCount:     end - start,
		}

		progressFields := buildBatchProgressFields(batchNumber, totalBatches, batchSize, len(batchProductsData), len(variationData))
		logger.GetGlobalLogger("shein/product").WithFields(progressFields).Info("sale attribute batch started")
		singleProcessor := NewSaleAttributeSingleProcessor(p.handler)
		batchResult := singleProcessor.ProcessSingleBatch(input, batchRequest)
		if err := validateBatchResult(batchResult, batchProductsData); err != nil {
			logger.GetGlobalLogger("shein/product").Errorf("sale attribute batch %d/%d validation failed: %v", batchNumber, totalBatches, err)
			return sheinattr.ResultSaleAttribute{}
		}
		allVariants = append(allVariants, batchResult.Variants...)
		allSaleAttributes = mergeBatchSaleAttributes(allSaleAttributes, batchResult.SaleAttributes)
		logger.GetGlobalLogger("shein/product").WithFields(progressFields).Info("sale attribute batch completed")
	}

	return sheinattr.ResultSaleAttribute{SaleAttributes: allSaleAttributes, Variants: allVariants}
}

func buildBatchProgressFields(batchNumber, totalBatches, batchSize, batchVariantCount, totalVariants int) map[string]any {
	processedVariants := (batchNumber-1)*batchSize + batchVariantCount
	if processedVariants > totalVariants {
		processedVariants = totalVariants
	}

	return map[string]any{
		"batch":              batchNumber,
		"total_batches":      totalBatches,
		"batch_size":         batchSize,
		"batch_variant_count": batchVariantCount,
		"processed_variants": processedVariants,
		"total_variants":     totalVariants,
	}
}

func mergeBatchSaleAttributes(existing []sheinattr.ResultAttribute, incoming []sheinattr.ResultAttribute) []sheinattr.ResultAttribute {
	for _, incomingAttr := range incoming {
		existingIndex := -1
		for i := range existing {
			if existing[i].AttrID == incomingAttr.AttrID {
				existingIndex = i
				break
			}
		}
		if existingIndex == -1 {
			existing = append(existing, incomingAttr)
			continue
		}

		seen := make(map[string]struct{}, len(existing[existingIndex].AttrValue))
		for _, value := range existing[existingIndex].AttrValue {
			seen[normalizeBatchAttributeValue(value.Value)] = struct{}{}
		}
		for _, value := range incomingAttr.AttrValue {
			key := normalizeBatchAttributeValue(value.Value)
			if _, ok := seen[key]; ok {
				continue
			}
			existing[existingIndex].AttrValue = append(existing[existingIndex].AttrValue, value)
			seen[key] = struct{}{}
		}
	}
	return existing
}

func validateBatchResult(result sheinattr.ResultSaleAttribute, batchProductsData []sheinattr.ProductVariantData) error {
	expected := make(map[string]struct{}, len(batchProductsData))
	for _, product := range batchProductsData {
		asin := strings.TrimSpace(product.ASIN)
		if asin == "" {
			continue
		}
		expected[asin] = struct{}{}
	}

	if len(result.Variants) != len(expected) {
		return fmt.Errorf("variant count mismatch: got %d want %d", len(result.Variants), len(expected))
	}

	seen := make(map[string]struct{}, len(result.Variants))
	for _, variant := range result.Variants {
		asin := strings.TrimSpace(variant.ASIN)
		if asin == "" {
			return fmt.Errorf("batch returned variant with empty ASIN")
		}
		if _, ok := expected[asin]; !ok {
			return fmt.Errorf("batch returned unexpected ASIN %q", asin)
		}
		if _, duplicated := seen[asin]; duplicated {
			return fmt.Errorf("batch returned duplicate ASIN %q", asin)
		}
		seen[asin] = struct{}{}
	}

	for asin := range expected {
		if _, ok := seen[asin]; !ok {
			return fmt.Errorf("batch missed expected ASIN %q", asin)
		}
	}

	return nil
}

func normalizeBatchAttributeValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
