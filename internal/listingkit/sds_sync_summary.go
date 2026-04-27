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
				layer := prototype.Layers[0]
				summary.LayerID = layer.LayerID
				summary.Diagnostics = &SDSSyncDiagnostics{
					LayerContent:   strings.TrimSpace(layer.Content),
					LayerImgWidth:  layer.ImgWidth,
					LayerImgHeight: layer.ImgHeight,
					ResizeMode:     layer.ResizeMode,
					FitLevel:       layer.FitLevel,
					RenderedCount:  len(summary.MockupImageURLs),
				}
			}
		}
	}
	if designResult.Material != nil && designResult.Material.Material != nil {
		material := designResult.Material.Material
		summary.MaterialID = material.ID
		if summary.Diagnostics == nil {
			summary.Diagnostics = &SDSSyncDiagnostics{}
		}
		summary.Diagnostics.MaterialImageURL = strings.TrimSpace(firstNonEmptyString(material.ImageURL, material.ImageURLAlt))
		summary.Diagnostics.MaterialFileCode = strings.TrimSpace(material.FileCode)
		summary.Diagnostics.MaterialWidth = int(material.Width)
		summary.Diagnostics.MaterialHeight = int(material.Height)
		if summary.Diagnostics.RenderedCount == 0 {
			summary.Diagnostics.RenderedCount = len(summary.MockupImageURLs)
		}
	}
	if len(summary.MockupImageURLs) == 0 {
		summary.Status = "render_unavailable"
		summary.Error = "SDS did not return current fused mockup images"
	}
	return summary
}

func buildSDSVariantSyncSummaries(options *SDSSyncOptions, variants []SDSSyncVariantOption, designResult *design.PrepareSyncDesignResult) []SDSSyncSummary {
	if len(variants) == 0 {
		if summary := buildSDSSyncSummary(options, designResult); summary != nil {
			return []SDSSyncSummary{*summary}
		}
		return nil
	}
	base := buildSDSSyncSummary(options, designResult)
	byProduct := map[int64][]string{}
	if designResult != nil {
		byProduct = designResult.RenderedImageURLsByProduct
	}
	summaries := make([]SDSSyncSummary, 0, len(variants))
	for _, variant := range variants {
		summary := SDSSyncSummary{
			VariantID:        variant.VariantID,
			ProductID:        variant.VariantID,
			PrototypeGroupID: firstNonZeroInt64(variant.PrototypeGroupID, base.PrototypeGroupID),
			LayerID:          firstNonEmptyString(variant.LayerID, base.LayerID),
			MaterialID:       base.MaterialID,
			ProductName:      base.ProductName,
			ProductSKU:       base.ProductSKU,
			VariantSKU:       strings.TrimSpace(variant.VariantSKU),
			VariantSize:      strings.TrimSpace(variant.Size),
			VariantColor:     strings.TrimSpace(variant.Color),
			MockupImageURLs:  uniqueNonEmptyStrings(byProduct[variant.VariantID]),
			Status:           "completed",
			Diagnostics:      cloneSDSSyncDiagnostics(base.Diagnostics),
		}
		if len(summary.MockupImageURLs) == 0 && variant.VariantID == base.ProductID {
			summary.MockupImageURLs = uniqueNonEmptyStrings(base.MockupImageURLs)
		}
		if summary.Diagnostics != nil {
			summary.Diagnostics.RenderedCount = len(summary.MockupImageURLs)
		}
		if len(summary.MockupImageURLs) == 0 {
			summary.Status = "render_unavailable"
			summary.Error = "SDS did not return current fused mockup images"
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func cloneSDSSyncDiagnostics(input *SDSSyncDiagnostics) *SDSSyncDiagnostics {
	if input == nil {
		return nil
	}
	copy := *input
	return &copy
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
