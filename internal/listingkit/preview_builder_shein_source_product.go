package listingkit

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

func buildSheinSourceProductSummary(canonical *canonical.Product) *SheinSourceProductSummary {
	if canonical == nil {
		return nil
	}
	summary := &SheinSourceProductSummary{
		Title:        canonical.Title,
		CategoryPath: append([]string(nil), canonical.CategoryPath...),
		Attributes:   map[string]string{},
	}
	for key, attr := range canonical.Attributes {
		if strings.TrimSpace(attr.Value) != "" {
			summary.Attributes[key] = attr.Value
		}
	}
	if len(summary.Attributes) == 0 {
		summary.Attributes = nil
	}
	if canonical.Specifications != nil {
		if canonical.Specifications.Weight != nil {
			summary.VariantWeight = canonical.Specifications.Weight.Value
		}
		if canonical.Specifications.Technical != nil {
			summary.VariantSize = canonical.Specifications.Technical["size"]
			summary.VariantColor = canonical.Specifications.Technical["color"]
			summary.ProductionCycle = canonical.Specifications.Technical["production_cycle_hours"]
		}
	}
	for _, image := range canonical.Images {
		if strings.TrimSpace(image.URL) != "" {
			summary.ImageURLs = append(summary.ImageURLs, image.URL)
		}
	}
	if len(canonical.Variants) > 0 {
		variant := canonical.Variants[0]
		summary.VariantSKU = variant.SKU
		if variant.Price != nil {
			summary.VariantPrice = variant.Price.Amount
		}
		if value := variant.Attributes["Size"].Value; strings.TrimSpace(value) != "" {
			summary.VariantSize = value
		}
		if value := variant.Attributes["Color"].Value; strings.TrimSpace(value) != "" {
			summary.VariantColor = value
		}
		for _, image := range variant.Images {
			if strings.TrimSpace(image.URL) != "" {
				summary.ImageURLs = append(summary.ImageURLs, image.URL)
			}
		}
	}
	if summary.SKU == "" {
		summary.SKU = summary.Attributes["sku"]
	}
	summary.ImageURLs = uniqueStrings(summary.ImageURLs)
	return summary
}
