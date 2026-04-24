package listingkit

import (
	"strconv"
	"strings"

	"task-processor/internal/productenrich"
)

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

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

func studioAttributes(sds *SDSSyncOptions, trace productenrich.FieldTrace) map[string]productenrich.CanonicalAttribute {
	if sds == nil {
		return nil
	}
	attrs := map[string]productenrich.CanonicalAttribute{}
	addAttribute(attrs, "sku", sds.ProductSKU, trace)
	addAttribute(attrs, "material", firstNonEmptyString(sds.Material, sds.MaterialDescription), trace)
	addAttribute(attrs, "production_process", sds.ProductionProcess, trace)
	addAttribute(attrs, "design_area", sds.DesignArea, trace)
	addAttribute(attrs, "picture_request", sds.PictureRequest, trace)
	addAttribute(attrs, "applicable_scenarios", sds.ApplicableScenarios, trace)
	addAttribute(attrs, "washing_instructions", sds.WashingInstructions, trace)
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

func addAttribute(attrs map[string]productenrich.CanonicalAttribute, key, value string, trace productenrich.FieldTrace) {
	if strings.TrimSpace(value) == "" {
		return
	}
	attrs[key] = productenrich.CanonicalAttribute{Value: strings.TrimSpace(value), Trace: trace}
}

func studioSpecifications(sds *SDSSyncOptions) *productenrich.ProductSpecs {
	if sds == nil {
		return nil
	}
	specs := &productenrich.ProductSpecs{Technical: map[string]string{}}
	if sds.VariantWeight > 0 {
		specs.Weight = &productenrich.Weight{Value: sds.VariantWeight, Unit: "g"}
	}
	addTechnicalSpec(specs.Technical, "size", sds.VariantSize)
	addTechnicalSpec(specs.Technical, "color", sds.VariantColor)
	addTechnicalSpec(specs.Technical, "material", firstNonEmptyString(sds.Material, sds.MaterialDescription))
	addTechnicalSpec(specs.Technical, "production_process", sds.ProductionProcess)
	addTechnicalSpec(specs.Technical, "design_area", sds.DesignArea)
	addTechnicalSpec(specs.Technical, "picture_request", sds.PictureRequest)
	if sds.ProductionCycle > 0 {
		specs.Technical["production_cycle_hours"] = strconv.Itoa(sds.ProductionCycle)
	}
	if specs.Weight == nil && len(specs.Technical) == 0 {
		return nil
	}
	if len(specs.Technical) == 0 {
		specs.Technical = nil
	}
	return specs
}

func addTechnicalSpec(specs map[string]string, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	specs[key] = strings.TrimSpace(value)
}

func studioVariants(sds *SDSSyncOptions, images []productenrich.CanonicalImage, trace productenrich.FieldTrace) []productenrich.CanonicalVariant {
	if sds == nil {
		return nil
	}
	sku := firstNonEmptyString(sds.VariantSKU, sds.ProductSKU)
	if sku == "" && strings.TrimSpace(sds.VariantSize) == "" && strings.TrimSpace(sds.VariantColor) == "" {
		return nil
	}

	attrs := map[string]productenrich.CanonicalAttribute{}
	addAttribute(attrs, "Size", sds.VariantSize, trace)
	addAttribute(attrs, "Color", sds.VariantColor, trace)

	var price *productenrich.PriceInfo
	if sds.VariantPrice > 0 {
		price = &productenrich.PriceInfo{Currency: "USD", Amount: sds.VariantPrice, CostPrice: sds.VariantPrice}
	}

	return []productenrich.CanonicalVariant{{
		SKU:        firstNonEmptyString(sku, "SDS-STUDIO-001"),
		Attributes: attrs,
		Price:      price,
		Stock:      999,
		Images:     append([]productenrich.CanonicalImage(nil), images...),
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

func appendNonEmpty(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
