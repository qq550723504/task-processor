package sale

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
)

type SaleAttributeRequestBuilder struct {
	contextBuilder *SaleAttributeContextBuilder
	valueFilter    *SaleAttributeValueFilter
}

func NewSaleAttributeRequestBuilder() *SaleAttributeRequestBuilder {
	return &SaleAttributeRequestBuilder{contextBuilder: NewSaleAttributeContextBuilder(), valueFilter: NewSaleAttributeValueFilter()}
}

func (r *SaleAttributeRequestBuilder) BuildGenerationRequest(input *SaleAttributeInput, productsData []map[string]string, attributeMetadata []sheinattr.AttributeMetadata, attributeNameMappings map[int]string) *sheinattr.GenerationRequest {
	var attributeMappings []sheinattr.AttributeNameMapping
	for attrID, attrName := range attributeNameMappings {
		attributeMappings = append(attributeMappings, sheinattr.AttributeNameMapping{AttrID: attrID, VariantAttributeName: attrName})
	}

	var productVariantData []sheinattr.ProductVariantData
	for _, product := range productsData {
		attributes := make(map[string]string)
		for key, value := range product {
			if key != "asin" && key != "title" && key != "price" && key != "currency" && key != "productdimensions" && key != "product_details" && key != "weight" {
				if value != "" {
					attributes[key] = value
				}
			}
		}
		price := 0.0
		if priceStr, ok := product["price"]; ok {
			price, _ = strconv.ParseFloat(priceStr, 64)
		}
		productVariantData = append(productVariantData, sheinattr.ProductVariantData{
			ASIN:       product["asin"],
			Title:      product["title"],
			Attributes: attributes,
			Price:      price,
			Dimensions: product["productdimensions"],
			Weight:     product["weight"],
		})
	}

	variationAttributeValues := &input.AmazonProduct.VariationsValues
	if !input.HasVariants() {
		emptyVariations := []model.VariationValue{}
		variationAttributeValues = &emptyVariations
	}

	variationData := scopeVariationsToProductsData(input.AmazonProduct.Variations, productVariantData)

	return &sheinattr.GenerationRequest{
		ProductsData:             productVariantData,
		VariationData:            variationData,
		VariationAttributeValues: variationAttributeValues,
		SaleAttributesData:       attributeMetadata,
		AttributeMappings:        attributeMappings,
		RequiredVariantCount:     len(productsData),
	}
}

func (r *SaleAttributeRequestBuilder) BuildUserPrompt(input *SaleAttributeInput, request *sheinattr.GenerationRequest) string {
	saleAttributeDataBytes, _ := json.Marshal(request.SaleAttributesData)
	productsDataBytes, _ := json.Marshal(request.ProductsData)
	attributeMappingBytes, _ := json.Marshal(request.AttributeMappings)
	batchAmazonProduct := scopedAmazonProductForRequest(input, request)
	batchVariants := scopedVariantsForRequest(input, request)
	productContext := r.contextBuilder.BuildCompactProductContext(batchAmazonProduct, batchVariants)
	isSingleVariant := !input.HasVariants()

	var originalAttributeValues string
	var attributeValueHint string
	if request.VariationAttributeValues != nil && len(*request.VariationAttributeValues) > 0 {
		originalAttributeValuesBytes, _ := json.Marshal(*request.VariationAttributeValues)
		originalAttributeValues = string(originalAttributeValuesBytes)
		attributeValueHint = "Use original variation values exactly as-is. saleAttributes should include only values actually used by variants."
		logger.GetGlobalLogger("shein/product").Debug("loaded original variation values for prompt")
	} else {
		originalAttributeValues = "[]"
		if isSingleVariant {
			attributeValueHint = "This is a single-variant product. Infer the most reasonable values from the product context."
		} else {
			attributeValueHint = "Original variation values are unavailable. Infer them conservatively from product information."
		}
	}

	var productTypeHint string
	if isSingleVariant {
		productTypeHint = "This is a single-variant product. Generate exactly one variant."
	} else {
		productTypeHint = fmt.Sprintf("This is a multi-variant product. Generate %d variants.", request.RequiredVariantCount)
	}

	extraContextSection := r.contextBuilder.BuildExtraContext(batchAmazonProduct, batchVariants, request.ProductsData)

	return fmt.Sprintf(`Task: generate SHEIN sale attributes for %d Amazon products.

%s

Product context:
%s

Products data:
%s

Original variation values:
%s
%s

Available sale attributes:
%s

Attribute mappings:
%s

Extra context:
%s`,
		request.RequiredVariantCount,
		productTypeHint,
		productContext,
		string(productsDataBytes),
		originalAttributeValues,
		attributeValueHint,
		string(saleAttributeDataBytes),
		string(attributeMappingBytes),
		extraContextSection,
	)
}

func scopedAmazonProductForRequest(input *SaleAttributeInput, request *sheinattr.GenerationRequest) model.Product {
	if input == nil || input.AmazonProduct == nil {
		return model.Product{}
	}

	scoped := *input.AmazonProduct
	scoped.Variations = append([]model.Variation(nil), request.VariationData...)
	if request.VariationAttributeValues != nil {
		scoped.VariationsValues = append([]model.VariationValue(nil), (*request.VariationAttributeValues)...)
	} else {
		scoped.VariationsValues = nil
	}
	return scoped
}

func scopedVariantsForRequest(input *SaleAttributeInput, request *sheinattr.GenerationRequest) []model.Product {
	if input == nil || len(input.Variants) == 0 {
		return nil
	}

	expected := make(map[string]struct{}, len(request.ProductsData))
	for _, product := range request.ProductsData {
		asin := strings.TrimSpace(product.ASIN)
		if asin != "" {
			expected[asin] = struct{}{}
		}
	}

	scoped := make([]model.Product, 0, len(expected))
	for _, variant := range input.Variants {
		if _, ok := expected[strings.TrimSpace(variant.Asin)]; ok {
			scoped = append(scoped, variant)
		}
	}
	return scoped
}

func scopeVariationAttributeValuesToProductsData(values *[]model.VariationValue, productsData []sheinattr.ProductVariantData) []model.VariationValue {
	if values == nil {
		return nil
	}

	scoped := make([]model.VariationValue, 0, len(*values))
	for _, variation := range *values {
		used := usedVariationValuesForName(variation.VariantName, productsData)
		if len(used) == 0 {
			scoped = append(scoped, variation)
			continue
		}

		filtered := model.VariationValue{
			VariantName: variation.VariantName,
			Values:      make([]string, 0, len(variation.Values)),
		}
		for _, value := range variation.Values {
			if _, ok := used[normalizeVariationField(value)]; ok {
				filtered.Values = append(filtered.Values, value)
			}
		}
		if len(filtered.Values) == 0 {
			filtered.Values = append(filtered.Values, variation.Values...)
		}
		scoped = append(scoped, filtered)
	}

	return scoped
}

func scopeVariationsToProductsData(variations []model.Variation, productsData []sheinattr.ProductVariantData) []model.Variation {
	if len(variations) == 0 || len(productsData) == 0 {
		return append([]model.Variation(nil), variations...)
	}

	expected := make(map[string]struct{}, len(productsData))
	for _, product := range productsData {
		asin := strings.TrimSpace(product.ASIN)
		if asin != "" {
			expected[asin] = struct{}{}
		}
	}

	scoped := make([]model.Variation, 0, len(productsData))
	for _, variation := range variations {
		if _, ok := expected[strings.TrimSpace(variation.Asin)]; ok {
			scoped = append(scoped, variation)
		}
	}

	if len(scoped) == 0 {
		return append([]model.Variation(nil), variations...)
	}

	return scoped
}

func usedVariationValuesForName(name string, productsData []sheinattr.ProductVariantData) map[string]struct{} {
	normalizedName := normalizeVariationField(name)
	used := make(map[string]struct{})
	for _, product := range productsData {
		for attrName, attrValue := range product.Attributes {
			if normalizeVariationField(attrName) != normalizedName {
				continue
			}
			value := normalizeVariationField(attrValue)
			if value != "" {
				used[value] = struct{}{}
			}
		}
	}
	return used
}

func normalizeVariationField(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func variationAttributeValuesPointer(values []model.VariationValue) *[]model.VariationValue {
	if values == nil {
		return nil
	}
	copied := append([]model.VariationValue(nil), values...)
	return &copied
}
