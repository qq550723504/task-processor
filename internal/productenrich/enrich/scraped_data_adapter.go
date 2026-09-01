package enrich

import (
	"strings"

	crawler1688 "task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/product/catalog/canonical"
	"task-processor/internal/productenrich"
)

// convert1688ProductToScrapedData is the legacy productenrich consumer adapter.
// A1688 owns source normalization; the old runtime owns its ScrapedData shape.
func convert1688ProductToScrapedData(product *crawler1688.Alibaba1688ProductSnapshot) *productenrich.ScrapedData {
	if product == nil {
		return nil
	}
	images := normalize1688Images(product.Images)

	specs := make(map[string]string, len(product.Specifications))
	for _, specification := range product.Specifications {
		name := strings.TrimSpace(specification.Name)
		value := strings.TrimSpace(specification.Value)
		if name == "" || value == "" {
			continue
		}
		specs[name] = value
	}

	return &productenrich.ScrapedData{
		Title:             product.Title,
		Category:          product.Category,
		Description:       product.NormalizedDescription(),
		Images:            images,
		Price:             product.MinPrice,
		Specs:             specs,
		VariantDimensions: build1688VariantDimensions(product.VariationValues),
		Variants:          build1688ScrapedVariants(product, images),
	}
}

func build1688VariantDimensions(values []crawler1688.Alibaba1688VariationValueSnapshot) []canonical.ScrapedVariantDimension {
	if len(values) == 0 {
		return nil
	}

	dimensions := make([]canonical.ScrapedVariantDimension, 0, len(values))
	for _, item := range values {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}

		dimension := canonical.ScrapedVariantDimension{Name: name}
		seen := make(map[string]struct{}, len(item.Values))
		for _, raw := range item.Values {
			value := strings.TrimSpace(raw)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			dimension.Values = append(dimension.Values, value)
		}
		if len(dimension.Values) == 0 {
			continue
		}
		dimensions = append(dimensions, dimension)
	}

	if len(dimensions) == 0 {
		return nil
	}
	return dimensions
}

func build1688ScrapedVariants(product *crawler1688.Alibaba1688ProductSnapshot, fallbackImages []string) []productenrich.ProductVariant {
	if product == nil || len(product.Variants) == 0 {
		return nil
	}

	variants := make([]productenrich.ProductVariant, 0, len(product.Variants))
	for index, variant := range product.Variants {
		converted := productenrich.ProductVariant{
			SKU:        variant.SourceSKU(index),
			Attributes: variant.NormalizedAttributes(),
			Stock:      variant.Stock,
			Images:     collect1688VariantImages(variant, fallbackImages),
			IsDefault:  index == 0,
		}
		if variant.Price > 0 {
			converted.Price = &canonical.PriceInfo{
				Currency:  product.NormalizedCurrency(),
				Amount:    variant.Price,
				CostPrice: variant.Price,
			}
		}
		variants = append(variants, converted)
	}

	return variants
}

func normalize1688Images(images []string) []string {
	if len(images) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, raw := range images {
		image := strings.TrimSpace(raw)
		if image == "" {
			continue
		}
		if _, exists := seen[image]; exists {
			continue
		}
		seen[image] = struct{}{}
		normalized = append(normalized, image)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func collect1688VariantImages(variant crawler1688.Alibaba1688VariantSnapshot, fallback []string) []string {
	images := make([]string, 0, 1)
	if image := strings.TrimSpace(variant.Image); image != "" {
		images = append(images, image)
	}
	if len(images) == 0 && len(fallback) > 0 {
		if image := strings.TrimSpace(fallback[0]); image != "" {
			images = append(images, image)
		}
	}
	if len(images) == 0 {
		return nil
	}
	return images
}
