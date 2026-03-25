package sale

import (
	"encoding/json"
	"fmt"
	"strconv"

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

	return &sheinattr.GenerationRequest{
		ProductsData:             productVariantData,
		VariationData:            input.AmazonProduct.Variations,
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
	productContext := r.contextBuilder.BuildCompactProductContext(*input.AmazonProduct, input.Variants)
	isSingleVariant := !input.HasVariants()

	var originalAttributeValues string
	var attributeValueHint string
	if len(input.AmazonProduct.VariationsValues) > 0 {
		originalAttributeValuesBytes, _ := json.Marshal(input.AmazonProduct.VariationsValues)
		originalAttributeValues = string(originalAttributeValuesBytes)
		attributeValueHint = "Use original variation values exactly as-is. saleAttributes should include only values actually used by variants."
		logger.GetGlobalLogger("shein/product").Info("loaded original variation values for prompt")
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

	extraContextSection := r.contextBuilder.BuildExtraContext(*input.AmazonProduct, input.Variants, request.ProductsData)

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
