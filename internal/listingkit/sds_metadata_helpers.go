package listingkit

import "strings"

func sdsStyleName(options *SDSSyncOptions) string {
	if options == nil {
		return ""
	}
	if name := strings.TrimSpace(options.StyleName); name != "" {
		return name
	}
	suffix := normalizedSDSStyleID(options.StyleID)
	if suffix == "" {
		return ""
	}
	return "Style " + suffix
}

func normalizedSDSStyleID(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	var result strings.Builder
	for _, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') {
			result.WriteRune(character)
		}
		if result.Len() == 8 {
			break
		}
	}
	return result.String()
}

func sdsCanonicalAttributes(options *SDSSyncOptions) map[string]string {
	if options == nil {
		return nil
	}
	attributes := map[string]string{}
	values := map[string]string{
		"sku":                     options.ProductSKU,
		"product_sku":             options.ProductSKU,
		"product_english_name":    options.ProductEnglishName,
		"material":                firstNonEmptyString(options.Material, options.MaterialDescription),
		"material_description":    options.MaterialDescription,
		"production_process":      options.ProductionProcess,
		"product_performance":     options.ProductPerformance,
		"design_area":             options.DesignArea,
		"picture_request":         options.PictureRequest,
		"applicable_scenarios":    options.ApplicableScenarios,
		"washing_instructions":    options.WashingInstructions,
		"special_description":     options.SpecialDescription,
		"product_size":            options.ProductSize,
		"packaging_specification": options.PackagingSpecification,
		"variant_sku":             options.VariantSKU,
		"variant_size":            options.VariantSize,
		"variant_color":           options.VariantColor,
	}
	for key, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			attributes[key] = value
		}
	}
	if len(attributes) == 0 {
		return nil
	}
	return attributes
}
