package listingkit

import (
	"strings"

	"task-processor/internal/sds/design"
)

func buildSDSSyncSummary(options *SDSSyncOptions, designResult *design.PrepareSyncDesignResult) *SDSSyncSummary {
	summary := &SDSSyncSummary{Status: "completed"}
	if options != nil {
		summary.VariantID = options.VariantID
	}
	if designResult == nil {
		return summary
	}
	if designResult.Page != nil {
		product := designResult.Page.Product
		summary.ProductID = product.ID
		summary.ProductName = strings.TrimSpace(product.Name)
		summary.ProductSKU = strings.TrimSpace(firstNonEmptyString(product.ParentSKU, product.SKU))
		summary.VariantSKU = strings.TrimSpace(product.SKU)
		summary.VariantSize = strings.TrimSpace(firstNonEmptyString(product.Size, product.SizeDTO.SizeName))
		summary.VariantColor = strings.TrimSpace(firstNonEmptyString(product.ColorName, product.Color.ColorName, product.Color.ChineseName))
	}
	if designResult.Request != nil {
		summary.PrototypeGroupID = designResult.Request.PrototypeGroupID
		if len(designResult.Request.Prototypes) > 0 {
			prototype := designResult.Request.Prototypes[0]
			summary.MockupImageURLs = uniqueNonEmptyStrings(designResult.RenderedImageURLs)
			if len(prototype.Layers) > 0 {
				summary.LayerID = prototype.Layers[0].LayerID
			}
		}
	}
	if designResult.Material != nil && designResult.Material.Material != nil {
		summary.MaterialID = designResult.Material.Material.ID
	}
	if len(summary.MockupImageURLs) == 0 {
		summary.Status = "render_unavailable"
		summary.Error = "SDS did not return current fused mockup images"
	}
	return summary
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
