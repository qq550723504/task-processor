package listingkit

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"
)

func buildStudioBatchTaskGenerateRequest(
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	candidate studioBatchTaskCandidate,
	design StudioMaterializedDesignRecord,
) *GenerateRequest {
	if batch == nil {
		return &GenerateRequest{}
	}
	selection := candidate.SelectionSnapshot
	storeID := studioBatchTaskStoreID(session, batch, candidate.Selection.SheinStoreID)

	styleID := buildStudioBatchTaskScopedStyleID(batch.ID, candidate.Item.ID, design.ID, candidate.SelectionID)
	styleName := firstNonEmpty(strings.TrimSpace(candidate.Title), strings.TrimSpace(design.ID))
	req := &GenerateRequest{
		TenantID:     strings.TrimSpace(batch.TenantID),
		UserID:       strings.TrimSpace(batch.UserID),
		Text:         studioBatchTaskPrompt(session, batch),
		ImageURLs:    []string{strings.TrimSpace(design.ImageURL)},
		Platforms:    []string{"shein"},
		SheinStoreID: storeID,
		Options: &GenerateOptions{
			ProcessImages: false,
			SheinStudio: &SheinStudioOptions{
				StyleID:                styleID,
				StyleName:              styleName,
				SourceDesignURLs:       []string{strings.TrimSpace(design.ImageURL)},
				SelectedSDSImages:      toGenerateRequestSelectedSDSImages(batch.SelectedSDSImages),
				SizeReferenceImageURLs: append([]string(nil), selection.SizeReferenceImageURLs...),
			},
			SDS: buildStudioBatchTaskSDSOptions(selection, styleID, styleName),
		},
	}
	return req
}

func studioBatchTaskPrompt(session *SheinStudioSession, batch *StudioBatchRecord) string {
	if session != nil && strings.TrimSpace(session.Prompt) != "" {
		return strings.TrimSpace(session.Prompt)
	}
	if batch == nil {
		return ""
	}
	return strings.TrimSpace(batch.Prompt)
}

func studioBatchTaskStoreID(session *SheinStudioSession, batch *StudioBatchRecord, groupedStoreID string) int64 {
	if storeID := parseStudioBatchTaskStoreID(groupedStoreID); storeID > 0 {
		return storeID
	}
	if session != nil {
		if storeID := parseStudioBatchTaskStoreID(session.SheinStoreID); storeID > 0 {
			return storeID
		}
	}
	if batch == nil {
		return 0
	}
	return batch.SheinStoreID
}

func buildStudioBatchTaskSDSOptions(
	selection SheinStudioSelection,
	styleID string,
	styleName string,
) *SDSSyncOptions {
	return &SDSSyncOptions{
		VariantID:              selection.VariantID,
		ParentProductID:        selection.ParentProductID,
		PrototypeGroupID:       selection.PrototypeGroupID,
		LayerID:                selection.LayerID,
		DesignType:             "material", // Default design type
		ProductSize:            selection.ProductSize,
		PackagingSpecification: selection.PackagingSpecification,
		ProductName:            selection.ProductName,
		BlankDesignURL:         selection.BlankDesignURL,
		TemplateImageURL:       selection.TemplateImageURL,
		MaskImageURL:           selection.MaskImageURL,
		PrintableWidth:         selection.PrintableWidth,
		PrintableHeight:        selection.PrintableHeight,
		MockupImageURLs:        append([]string(nil), selection.MockupImageURLs...),
		StyleID:                styleID,
		StyleName:              styleName,
		Variants:               buildStudioBatchTaskVariantOptions(selection.Variants),
	}
}

func buildStudioBatchTaskVariantOptions(
	variants []SheinStudioSelectionVariant,
) []SDSSyncVariantOption {
	if len(variants) == 0 {
		return nil
	}
	result := make([]SDSSyncVariantOption, 0, len(variants))
	for _, variant := range variants {
		result = append(result, SDSSyncVariantOption{
			VariantID:              variant.VariantID,
			VariantSKU:             variant.VariantSKU,
			Size:                   variant.Size,
			Color:                  variant.Color,
			Price:                  variant.Price,
			Weight:                 variant.Weight,
			BoxLength:              variant.BoxLength,
			BoxWidth:               variant.BoxWidth,
			BoxHeight:              variant.BoxHeight,
			ProductionCycle:        variant.ProductionCycle,
			PrototypeGroupID:       variant.PrototypeGroupID,
			LayerID:                variant.LayerID,
			TemplateImageURL:       variant.TemplateImageURL,
			MaskImageURL:           variant.MaskImageURL,
			BlankDesignURL:         variant.BlankDesignURL,
			MockupImageURL:         variant.MockupImageURL,
			MockupImageURLs:        append([]string(nil), variant.MockupImageURLs...),
			SizeReferenceImageURLs: append([]string(nil), variant.SizeReferenceImageURLs...),
		})
	}
	return result
}

func toGenerateRequestSelectedSDSImages(
	input SheinStudioSelectedSDSImageList,
) []SheinStudioSelectedSDSImage {
	if len(input) == 0 {
		return nil
	}
	result := make([]SheinStudioSelectedSDSImage, 0, len(input))
	for _, item := range input {
		result = append(result, SheinStudioSelectedSDSImage{
			ImageURL:   item.ImageURL,
			VariantSKU: item.VariantSKU,
			Color:      item.Color,
		})
	}
	return result
}

func parseStudioBatchTaskStoreID(raw string) int64 {
	storeID, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return storeID
}

func buildStudioBatchTaskStyleID(designID string) string {
	return buildStudioBatchTaskScopedStyleID("", "", designID, "")
}

func buildStudioBatchTaskScopedStyleID(batchID string, itemID string, designID string, selectionID string) string {
	raw := strings.Join([]string{batchID, itemID, designID, selectionID}, "|")
	sum := sha1.Sum([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(sum[:]))[:10]
}
