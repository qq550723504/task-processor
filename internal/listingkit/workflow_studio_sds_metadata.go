package listingkit

import (
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
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

func addAttribute(attrs map[string]canonical.Attribute, key, value string, trace canonical.FieldTrace) {
	if strings.TrimSpace(value) == "" {
		return
	}
	attrs[key] = canonical.Attribute{Value: strings.TrimSpace(value), Trace: trace}
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

func addTechnicalSpec(specs map[string]string, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	specs[key] = strings.TrimSpace(value)
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

			var price *productenrich.PriceInfo
			if item.Price > 0 {
				price = &productenrich.PriceInfo{Currency: "CNY", Amount: item.Price, CostPrice: item.Price}
			}
			var dimensions *productenrich.Dimensions
			if item.BoxLength > 0 && item.BoxWidth > 0 && item.BoxHeight > 0 {
				dimensions = &productenrich.Dimensions{
					Length: item.BoxLength,
					Width:  item.BoxWidth,
					Height: item.BoxHeight,
					Unit:   "cm",
				}
			}
			var weight *productenrich.Weight
			if item.Weight > 0 {
				weight = &productenrich.Weight{Value: item.Weight, Unit: "g"}
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

	var price *productenrich.PriceInfo
	if sds.VariantPrice > 0 {
		price = &productenrich.PriceInfo{Currency: "CNY", Amount: sds.VariantPrice, CostPrice: sds.VariantPrice}
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

func applyStudioStyleDimension(product *canonical.Product, sds *SDSSyncOptions) bool {
	if product == nil || len(product.Variants) == 0 {
		return false
	}
	styleName := studioStyleName(sds)
	if styleName == "" {
		return false
	}
	trace := canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: "SDS studio AI style dimension",
		}},
		Confidence:  0.94,
		IsInferred:  false,
		NeedsReview: false,
	}
	changed := false
	for i := range product.Variants {
		if product.Variants[i].Attributes == nil {
			product.Variants[i].Attributes = map[string]canonical.Attribute{}
		}
		current := strings.TrimSpace(product.Variants[i].Attributes[studioAIStyleAttributeKey].Value)
		if current == styleName {
			continue
		}
		product.Variants[i].Attributes[studioAIStyleAttributeKey] = canonical.Attribute{
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

func buildStudioVariantSKU(baseSKU, styleID, variantDiscriminator string, requireVariantDiscriminator bool, seen map[string]int) string {
	baseSKU = strings.TrimSpace(baseSKU)
	styleSuffix := normalizeStyleIDSuffix(styleID)
	variantDiscriminator = normalizeStudioVariantDiscriminator(variantDiscriminator)

	parts := make([]string, 0, 2)
	if baseSKU != "" {
		parts = append(parts, baseSKU)
	}
	if styleSuffix != "" {
		parts = append(parts, styleSuffix)
	}
	baseCandidate := strings.Join(parts, "-")
	if baseCandidate == "" {
		baseCandidate = "SDS-STUDIO-001"
	}
	if !requireVariantDiscriminator && seen == nil {
		return baseCandidate
	}
	if !requireVariantDiscriminator {
		if _, exists := seen[baseCandidate]; !exists {
			seen[baseCandidate] = 1
			return baseCandidate
		}
	}
	parts = parts[:0]
	if baseSKU != "" {
		parts = append(parts, baseSKU)
	}
	if variantDiscriminator != "" {
		parts = append(parts, variantDiscriminator)
	}
	if styleSuffix != "" {
		parts = append(parts, styleSuffix)
	}
	candidate := strings.Join(parts, "-")
	if candidate == "" {
		candidate = baseCandidate
	}
	if seen == nil {
		return candidate
	}
	if _, exists := seen[candidate]; !exists {
		seen[candidate] = 1
		return candidate
	}
	seen[candidate]++
	return candidate + "-" + strconv.Itoa(seen[candidate])
}

func studioVariantDiscriminator(item SDSSyncVariantOption, index int) string {
	if item.VariantID > 0 {
		return "V" + strconv.FormatInt(item.VariantID, 10)
	}
	return strings.Join([]string{
		strings.TrimSpace(item.Color),
		strings.TrimSpace(item.Size),
		"V" + strconv.Itoa(index+1),
	}, "-")
}

func studioFallbackVariantDiscriminator(sds *SDSSyncOptions) string {
	if sds == nil {
		return ""
	}
	if sds.VariantID > 0 {
		return "V" + strconv.FormatInt(sds.VariantID, 10)
	}
	if strings.TrimSpace(sds.VariantSKU) != "" {
		return ""
	}
	return strings.Join([]string{
		strings.TrimSpace(sds.VariantColor),
		strings.TrimSpace(sds.VariantSize),
	}, "-")
}

func normalizeStudioVariantDiscriminator(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ' || r == '/':
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteRune('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 24 {
		result = result[:24]
		result = strings.TrimRight(result, "-")
	}
	return result
}

func studioVariantBaseSKUCounts(sds *SDSSyncOptions) map[string]int {
	counts := map[string]int{}
	if sds == nil {
		return counts
	}
	for _, item := range sds.Variants {
		key := firstNonEmptyString(item.VariantSKU, sds.VariantSKU, sds.ProductSKU)
		if strings.TrimSpace(key) == "" {
			key = "__empty__"
		}
		counts[key]++
	}
	return counts
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
