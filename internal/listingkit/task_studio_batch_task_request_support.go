package listingkit

import (
	"strconv"
	"strings"
)

func buildStudioBatchTaskGenerateRequest(
	session *SheinStudioSession,
	groupedSelection SheinStudioGroupedSelection,
	design StudioMaterializedDesignRecord,
	sessionDesign SheinStudioDesign,
) *GenerateRequest {
	if session == nil {
		return &GenerateRequest{}
	}
	selection := groupedSelection.Selection
	storeID := parseStudioBatchTaskStoreID(groupedSelection.SheinStoreID)
	if storeID <= 0 {
		storeID = parseStudioBatchTaskStoreID(session.SheinStoreID)
	}

	styleID := buildStudioBatchTaskStyleID(design.ID)
	styleName := firstNonEmpty(
		strings.TrimSpace(design.TargetGroupLabel),
		strings.TrimSpace(selection.ProductName),
		strings.TrimSpace(design.ID),
	)
	req := &GenerateRequest{
		TenantID:     strings.TrimSpace(session.TenantID),
		UserID:       strings.TrimSpace(session.UserID),
		Text:         strings.TrimSpace(session.Prompt),
		ImageURLs:    []string{strings.TrimSpace(design.ImageURL)},
		Platforms:    []string{"shein"},
		SheinStoreID: storeID,
		Options: &GenerateOptions{
			ImageStrategy: strings.TrimSpace(session.ImageStrategy),
			ProcessImages: false,
			SheinStudio: &SheinStudioOptions{
				StyleID:                 styleID,
				StyleName:               styleName,
				SourceDesignURLs:        []string{strings.TrimSpace(design.ImageURL)},
				ProductImageURLs:        append([]string(nil), sessionDesign.ProductImageURLs...),
				SelectedSDSImages:       toGenerateRequestSelectedSDSImages(session.SelectedSDSImages),
				SizeReferenceImageURLs:  append([]string(nil), selection.SizeReferenceImageURLs...),
				RenderSizeImagesWithSDS: session.RenderSizeImagesWithSDS,
			},
			SDS: buildStudioBatchTaskSDSOptions(selection, styleID, styleName),
		},
	}
	return req
}

func buildStudioBatchTaskSDSOptions(
	selection SheinStudioSelection,
	styleID string,
	styleName string,
) *SDSSyncOptions {
	return &SDSSyncOptions{
		VariantID:        selection.VariantID,
		ParentProductID:  selection.ParentProductID,
		PrototypeGroupID: selection.PrototypeGroupID,
		LayerID:          selection.LayerID,
		DesignType:       "material", // Default design type
		ProductName:      selection.ProductName,
		BlankDesignURL:   selection.BlankDesignURL,
		TemplateImageURL: selection.TemplateImageURL,
		MaskImageURL:     selection.MaskImageURL,
		PrintableWidth:   selection.PrintableWidth,
		PrintableHeight:  selection.PrintableHeight,
		MockupImageURLs:  append([]string(nil), selection.MockupImageURLs...),
		StyleID:          styleID,
		StyleName:        styleName,
		Variants:         buildStudioBatchTaskVariantOptions(selection.Variants),
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
	compact := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - ('a' - 'A')
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, strings.TrimSpace(designID))
	if len(compact) > 8 {
		return compact[:8]
	}
	if compact == "" {
		return "STYLE001"
	}
	return compact
}
