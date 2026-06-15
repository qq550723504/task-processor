package listingkit

import (
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
)

const studioAIStyleAttributeKey = "ai_style"

func studioCategoryPath(sds *SDSSyncOptions) []string {
	if sds == nil {
		return nil
	}
	result := make([]string, 0, len(sds.CategoryPath))
	for _, item := range sds.CategoryPath {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func studioAttributes(sds *SDSSyncOptions, trace canonical.FieldTrace) map[string]canonical.Attribute {
	if sds == nil {
		return nil
	}
	attrs := map[string]canonical.Attribute{}
	addAttribute(attrs, "sku", sds.ProductSKU, trace)
	addAttribute(attrs, "product_sku", sds.ProductSKU, trace)
	addAttribute(attrs, "product_english_name", sds.ProductEnglishName, trace)
	addAttribute(attrs, "material", firstNonEmptyString(sds.Material, sds.MaterialDescription), trace)
	addAttribute(attrs, "material_description", sds.MaterialDescription, trace)
	addAttribute(attrs, "production_process", sds.ProductionProcess, trace)
	addAttribute(attrs, "product_performance", sds.ProductPerformance, trace)
	addAttribute(attrs, "design_area", sds.DesignArea, trace)
	addAttribute(attrs, "picture_request", sds.PictureRequest, trace)
	addAttribute(attrs, "applicable_scenarios", sds.ApplicableScenarios, trace)
	addAttribute(attrs, "washing_instructions", sds.WashingInstructions, trace)
	addAttribute(attrs, "special_description", sds.SpecialDescription, trace)
	addAttribute(attrs, "product_size", sds.ProductSize, trace)
	addAttribute(attrs, "packaging_specification", sds.PackagingSpecification, trace)
	addAttribute(attrs, "variant_sku", sds.VariantSKU, trace)
	addAttribute(attrs, "variant_size", sds.VariantSize, trace)
	addAttribute(attrs, "variant_color", sds.VariantColor, trace)
	if sds.IsElectricity != nil {
		addAttribute(attrs, "is_electricity", strconv.Itoa(*sds.IsElectricity), trace)
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

func studioSpecifications(sds *SDSSyncOptions) *canonical.ProductSpecs {
	if sds == nil {
		return nil
	}
	specs := &canonical.ProductSpecs{Technical: map[string]string{}}
	if sds.VariantWeight > 0 {
		specs.Weight = &canonical.Weight{Value: sds.VariantWeight, Unit: "g"}
	}
	if len(sds.Variants) > 0 {
		for _, variant := range sds.Variants {
			if specs.Weight == nil && variant.Weight > 0 {
				specs.Weight = &canonical.Weight{Value: variant.Weight, Unit: "g"}
			}
			if specs.Dimensions == nil && variant.BoxLength > 0 && variant.BoxWidth > 0 && variant.BoxHeight > 0 {
				specs.Dimensions = &canonical.Dimensions{
					Length: variant.BoxLength,
					Width:  variant.BoxWidth,
					Height: variant.BoxHeight,
					Unit:   "cm",
				}
			}
			if specs.Weight != nil && specs.Dimensions != nil {
				break
			}
		}
	}
	addTechnicalSpec(specs.Technical, "size", sds.VariantSize)
	addTechnicalSpec(specs.Technical, "color", sds.VariantColor)
	addTechnicalSpec(specs.Technical, "material", firstNonEmptyString(sds.Material, sds.MaterialDescription))
	addTechnicalSpec(specs.Technical, "production_process", sds.ProductionProcess)
	addTechnicalSpec(specs.Technical, "product_performance", sds.ProductPerformance)
	addTechnicalSpec(specs.Technical, "applicable_scenarios", sds.ApplicableScenarios)
	addTechnicalSpec(specs.Technical, "special_description", sds.SpecialDescription)
	addTechnicalSpec(specs.Technical, "product_size", sds.ProductSize)
	addTechnicalSpec(specs.Technical, "packaging_specification", sds.PackagingSpecification)
	addTechnicalSpec(specs.Technical, "design_area", sds.DesignArea)
	addTechnicalSpec(specs.Technical, "picture_request", sds.PictureRequest)
	if sds.ProductionCycle > 0 {
		specs.Technical["production_cycle_hours"] = strconv.Itoa(sds.ProductionCycle)
	}
	if specs.Weight == nil && specs.Dimensions == nil && len(specs.Technical) == 0 {
		return nil
	}
	if len(specs.Technical) == 0 {
		specs.Technical = nil
	}
	return specs
}

func studioVariants(sds *SDSSyncOptions, images []canonical.Image, trace canonical.FieldTrace) []canonical.Variant {
	if sds == nil {
		return nil
	}
	styleName := studioStyleName(sds)
	if len(sds.Variants) > 0 {
		variants := make([]canonical.Variant, 0, len(sds.Variants))
		baseSKUCounts := studioVariantBaseSKUCounts(sds)
		seenSKUs := map[string]int{}
		for index, item := range sds.Variants {
			baseSKU := firstNonEmptyString(item.VariantSKU, sds.VariantSKU, sds.ProductSKU)
			sku := buildStudioVariantSKU(baseSKU, sds.StyleID, studioVariantDiscriminator(item, index), baseSKUCounts[baseSKU] > 1, seenSKUs)
			attrs := map[string]canonical.Attribute{}
			addAttribute(attrs, "Size", item.Size, trace)
			addAttribute(attrs, "Color", item.Color, trace)
			addAttribute(attrs, "source_sds_sku", item.VariantSKU, trace)
			addAttribute(attrs, studioAIStyleAttributeKey, styleName, trace)

			var price *canonical.PriceInfo
			if item.Price > 0 {
				price = &canonical.PriceInfo{Currency: "CNY", Amount: item.Price, CostPrice: item.Price}
			}
			var dimensions *canonical.Dimensions
			if item.BoxLength > 0 && item.BoxWidth > 0 && item.BoxHeight > 0 {
				dimensions = &canonical.Dimensions{
					Length: item.BoxLength,
					Width:  item.BoxWidth,
					Height: item.BoxHeight,
					Unit:   "cm",
				}
			}
			var weight *canonical.Weight
			if item.Weight > 0 {
				weight = &canonical.Weight{Value: item.Weight, Unit: "g"}
			}
			variants = append(variants, canonical.Variant{
				SKU:        firstNonEmptyString(sku, "SDS-STUDIO-001"),
				Attributes: attrs,
				Price:      price,
				Stock:      999,
				Images:     append([]canonical.Image(nil), images...),
				Dimensions: dimensions,
				Weight:     weight,
				IsDefault:  index == 0,
				Trace:      trace,
			})
		}
		return variants
	}
	sku := buildStudioVariantSKU(firstNonEmptyString(sds.VariantSKU, sds.ProductSKU), sds.StyleID, studioFallbackVariantDiscriminator(sds), strings.TrimSpace(sds.VariantSKU) == "", nil)
	if sku == "" && strings.TrimSpace(sds.VariantSize) == "" && strings.TrimSpace(sds.VariantColor) == "" {
		return nil
	}

	attrs := map[string]canonical.Attribute{}
	addAttribute(attrs, "Size", sds.VariantSize, trace)
	addAttribute(attrs, "Color", sds.VariantColor, trace)
	addAttribute(attrs, "source_sds_sku", sds.VariantSKU, trace)
	addAttribute(attrs, studioAIStyleAttributeKey, styleName, trace)

	var price *canonical.PriceInfo
	if sds.VariantPrice > 0 {
		price = &canonical.PriceInfo{Currency: "CNY", Amount: sds.VariantPrice, CostPrice: sds.VariantPrice}
	}

	return []canonical.Variant{{
		SKU:        firstNonEmptyString(sku, "SDS-STUDIO-001"),
		Attributes: attrs,
		Price:      price,
		Stock:      999,
		Images:     append([]canonical.Image(nil), images...),
		IsDefault:  true,
		Trace:      trace,
	}}
}

func studioSellingPoints(sds *SDSSyncOptions) []string {
	points := []string{}
	if sds != nil {
		points = appendNonEmpty(points,
			sds.MaterialDescription,
			sds.ProductPerformance,
			sds.ApplicableScenarios,
			sds.SpecialDescription,
		)
	}
	if len(points) == 0 {
		points = []string{
			"Studio-generated design candidate",
			"SDS-backed product selection",
			"Ready for SHEIN review workflow",
		}
	}
	return points
}
