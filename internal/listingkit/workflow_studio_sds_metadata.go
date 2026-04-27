package listingkit

import (
	"strconv"
	"strings"

	"task-processor/internal/productenrich"
)

const studioAIStyleAttributeKey = "ai_style"

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
	addAttribute(attrs, "product_english_name", sds.ProductEnglishName, trace)
	addAttribute(attrs, "material", firstNonEmptyString(sds.Material, sds.MaterialDescription), trace)
	addAttribute(attrs, "production_process", sds.ProductionProcess, trace)
	addAttribute(attrs, "design_area", sds.DesignArea, trace)
	addAttribute(attrs, "picture_request", sds.PictureRequest, trace)
	addAttribute(attrs, "applicable_scenarios", sds.ApplicableScenarios, trace)
	addAttribute(attrs, "washing_instructions", sds.WashingInstructions, trace)
	if sds.IsElectricity != nil {
		addAttribute(attrs, "is_electricity", strconv.Itoa(*sds.IsElectricity), trace)
	}
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
	if len(sds.Variants) > 0 {
		for _, variant := range sds.Variants {
			if specs.Weight == nil && variant.Weight > 0 {
				specs.Weight = &productenrich.Weight{Value: variant.Weight, Unit: "g"}
			}
			if specs.Dimensions == nil && variant.BoxLength > 0 && variant.BoxWidth > 0 && variant.BoxHeight > 0 {
				specs.Dimensions = &productenrich.Dimensions{
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
	styleName := studioStyleName(sds)
	if len(sds.Variants) > 0 {
		variants := make([]productenrich.CanonicalVariant, 0, len(sds.Variants))
		for index, item := range sds.Variants {
			sku := firstNonEmptyString(item.VariantSKU, sds.VariantSKU, sds.ProductSKU)
			if suffix := normalizeStyleIDSuffix(sds.StyleID); suffix != "" && sku != "" {
				sku = sku + "-" + suffix
			}
			attrs := map[string]productenrich.CanonicalAttribute{}
			addAttribute(attrs, "Size", item.Size, trace)
			addAttribute(attrs, "Color", item.Color, trace)
			addAttribute(attrs, "source_sds_sku", item.VariantSKU, trace)
			addAttribute(attrs, studioAIStyleAttributeKey, styleName, trace)

			var price *productenrich.PriceInfo
			if item.Price > 0 {
				price = &productenrich.PriceInfo{Currency: "USD", Amount: item.Price, CostPrice: item.Price}
			}
			variants = append(variants, productenrich.CanonicalVariant{
				SKU:        firstNonEmptyString(sku, "SDS-STUDIO-001"),
				Attributes: attrs,
				Price:      price,
				Stock:      999,
				Images:     append([]productenrich.CanonicalImage(nil), images...),
				IsDefault:  index == 0,
				Trace:      trace,
			})
		}
		return variants
	}
	sku := firstNonEmptyString(sds.VariantSKU, sds.ProductSKU)
	if suffix := normalizeStyleIDSuffix(sds.StyleID); suffix != "" && sku != "" {
		sku = sku + "-" + suffix
	}
	if sku == "" && strings.TrimSpace(sds.VariantSize) == "" && strings.TrimSpace(sds.VariantColor) == "" {
		return nil
	}

	attrs := map[string]productenrich.CanonicalAttribute{}
	addAttribute(attrs, "Size", sds.VariantSize, trace)
	addAttribute(attrs, "Color", sds.VariantColor, trace)
	addAttribute(attrs, "source_sds_sku", sds.VariantSKU, trace)
	addAttribute(attrs, studioAIStyleAttributeKey, styleName, trace)

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

func studioStyleName(sds *SDSSyncOptions) string {
	if sds == nil {
		return ""
	}
	if name := strings.TrimSpace(sds.StyleName); name != "" {
		return name
	}
	if suffix := normalizeStyleIDSuffix(sds.StyleID); suffix != "" {
		return "Style " + suffix
	}
	return ""
}

func applyStudioStyleDimension(canonical *productenrich.CanonicalProduct, sds *SDSSyncOptions) bool {
	if canonical == nil || len(canonical.Variants) == 0 {
		return false
	}
	styleName := studioStyleName(sds)
	if styleName == "" {
		return false
	}
	trace := productenrich.FieldTrace{
		Sources: []productenrich.CanonicalSource{{
			Type:   productenrich.CanonicalSourceDerived,
			Detail: "SDS studio AI style dimension",
		}},
		Confidence:  0.94,
		IsInferred:  false,
		NeedsReview: false,
	}
	changed := false
	for i := range canonical.Variants {
		if canonical.Variants[i].Attributes == nil {
			canonical.Variants[i].Attributes = map[string]productenrich.CanonicalAttribute{}
		}
		current := strings.TrimSpace(canonical.Variants[i].Attributes[studioAIStyleAttributeKey].Value)
		if current == styleName {
			continue
		}
		canonical.Variants[i].Attributes[studioAIStyleAttributeKey] = productenrich.CanonicalAttribute{
			Value: styleName,
			Trace: trace,
		}
		changed = true
	}
	return changed
}

func normalizeStyleIDSuffix(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 8 {
			break
		}
	}
	return b.String()
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
