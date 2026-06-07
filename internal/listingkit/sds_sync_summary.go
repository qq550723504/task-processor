package listingkit

import (
	"fmt"
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
	if summary != nil && designResult != nil {
		attachRenderedVariantDiagnostics(summary, designResult, summary.ProductID)
		if summary.Diagnostics != nil && len(summary.Diagnostics.SensitiveWords) == 0 {
			for _, hits := range designResult.RenderedSensitiveWords {
				if len(hits) == 0 {
					continue
				}
				summary.Diagnostics.SensitiveWords = convertSensitiveWordHits(hits)
				break
			}
		}
	}
	if len(summary.MockupImageURLs) == 0 {
		summary.Status = "render_unavailable"
		summary.Error = buildRenderedImageUnavailableError(summary.Diagnostics)
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
		}
		if base.Diagnostics != nil {
			diagnostics := *base.Diagnostics
			summary.Diagnostics = &diagnostics
		}
		if summary.Diagnostics != nil && variant.VariantID != base.ProductID {
			summary.Diagnostics.FinishedProduct = nil
			summary.Diagnostics.SensitiveWords = nil
		}
		if len(summary.MockupImageURLs) == 0 && variant.VariantID == base.ProductID {
			summary.MockupImageURLs = uniqueNonEmptyStrings(base.MockupImageURLs)
		}
		if summary.Diagnostics != nil {
			summary.Diagnostics.RenderedCount = len(summary.MockupImageURLs)
		}
		attachRenderedVariantDiagnostics(&summary, designResult, variant.VariantID)
		if len(summary.MockupImageURLs) == 0 {
			summary.Status = "render_unavailable"
			summary.Error = buildRenderedImageUnavailableError(summary.Diagnostics)
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func attachRenderedVariantDiagnostics(summary *SDSSyncSummary, designResult *design.PrepareSyncDesignResult, variantID int64) {
	if summary == nil || designResult == nil || variantID <= 0 {
		return
	}
	observation, ok := designResult.RenderedImageObservations[variantID]
	if !ok {
		return
	}
	if summary.Diagnostics == nil {
		summary.Diagnostics = &SDSSyncDiagnostics{}
	}
	summary.Diagnostics.FinishedProduct = &SDSSyncFinishedProductObservation{
		Found:             observation.Found,
		BuildFinish:       observation.BuildFinish,
		Status:            observation.Status,
		MaterialImageName: observation.MaterialImageName,
		TaskID:            observation.TaskID,
		DesignTaskID:      observation.DesignTaskID,
		ItemID:            observation.ItemID,
		ImageCount:        observation.ImageCount,
		ThumbnailCount:    observation.ThumbnailCount,
	}
	if hits := designResult.RenderedSensitiveWords[observation.ItemID]; len(hits) > 0 {
		summary.Diagnostics.SensitiveWords = convertSensitiveWordHits(hits)
	}
}

func convertSensitiveWordHits(hits []design.SensitiveWordHit) []SDSSyncSensitiveWordHit {
	if len(hits) == 0 {
		return nil
	}
	result := make([]SDSSyncSensitiveWordHit, 0, len(hits))
	for _, hit := range hits {
		result = append(result, SDSSyncSensitiveWordHit{
			SensitiveWord: strings.TrimSpace(hit.SensitiveWord),
			Type:          hit.Type,
			TypeStrs:      strings.TrimSpace(hit.TypeStrs),
			ImgURL:        strings.TrimSpace(hit.ImgURL),
			IsParent:      hit.IsParent,
			PositionStrs:  strings.TrimSpace(hit.PositionStrs),
		})
	}
	return result
}

func buildRenderedImageUnavailableError(diagnostics *SDSSyncDiagnostics) string {
	if diagnostics == nil {
		return "SDS did not create finished product records for the current variant"
	}
	if len(diagnostics.SensitiveWords) > 0 {
		parts := make([]string, 0, len(diagnostics.SensitiveWords))
		for _, hit := range diagnostics.SensitiveWords {
			word := strings.TrimSpace(hit.SensitiveWord)
			position := strings.TrimSpace(hit.PositionStrs)
			if word == "" && position == "" {
				continue
			}
			if position != "" {
				parts = append(parts, fmt.Sprintf("%s（%s）", word, position))
			} else {
				parts = append(parts, word)
			}
		}
		if len(parts) > 0 {
			return "SDS sensitive-word check blocked rendered product export: " + strings.Join(uniqueNonEmptyStrings(parts), ", ")
		}
	}
	if diagnostics.FinishedProduct == nil || !diagnostics.FinishedProduct.Found {
		return "SDS did not create finished product records for the current variant"
	}
	if !diagnostics.FinishedProduct.BuildFinish {
		return "SDS finished product record exists but is not built yet"
	}
	if diagnostics.FinishedProduct.ImageCount == 0 {
		return "SDS finished product record exists but returned no fused mockup images"
	}
	return "SDS did not return current fused mockup images"
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
