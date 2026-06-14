package workspace

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

type SourceProductSummary struct {
	Title           string            `json:"title,omitempty"`
	SKU             string            `json:"sku,omitempty"`
	CategoryPath    []string          `json:"category_path,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
	VariantSKU      string            `json:"variant_sku,omitempty"`
	VariantSize     string            `json:"variant_size,omitempty"`
	VariantColor    string            `json:"variant_color,omitempty"`
	VariantPrice    float64           `json:"variant_price,omitempty"`
	VariantWeight   float64           `json:"variant_weight,omitempty"`
	ProductionCycle string            `json:"production_cycle,omitempty"`
	ImageURLs       []string          `json:"image_urls,omitempty"`
}

func BuildSourceProductSummary(product *canonical.Product) *SourceProductSummary {
	if product == nil {
		return nil
	}
	summary := &SourceProductSummary{
		Title:        product.Title,
		CategoryPath: append([]string(nil), product.CategoryPath...),
		Attributes:   map[string]string{},
	}
	for key, attr := range product.Attributes {
		if strings.TrimSpace(attr.Value) != "" {
			summary.Attributes[key] = attr.Value
		}
	}
	if len(summary.Attributes) == 0 {
		summary.Attributes = nil
	}
	if product.Specifications != nil {
		if product.Specifications.Weight != nil {
			summary.VariantWeight = product.Specifications.Weight.Value
		}
		if product.Specifications.Technical != nil {
			summary.VariantSize = product.Specifications.Technical["size"]
			summary.VariantColor = product.Specifications.Technical["color"]
			summary.ProductionCycle = product.Specifications.Technical["production_cycle_hours"]
		}
	}
	for _, image := range product.Images {
		if strings.TrimSpace(image.URL) != "" {
			summary.ImageURLs = append(summary.ImageURLs, image.URL)
		}
	}
	if len(product.Variants) > 0 {
		variant := product.Variants[0]
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
