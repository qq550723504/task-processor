package listingkit

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	sdstemplate "task-processor/internal/sds/template"
)

const defaultStudioBatchProductImageCount = 5

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
			ImageStrategy: normalizeSheinImageStrategy(candidate.ImageStrategy),
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

func buildStudioBatchTaskProductImageRequest(
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	candidate studioBatchTaskCandidate,
	design StudioMaterializedDesignRecord,
) *StudioProductImageRequest {
	selection := candidate.SelectionSnapshot
	styleName := firstNonEmpty(strings.TrimSpace(candidate.Title), strings.TrimSpace(design.ID))
	count := defaultStudioBatchProductImageCount
	customPrompt := ""
	promptMode := ""
	promptText := studioBatchTaskPrompt(session, batch)
	hotReferenceURLs := studioBatchTaskHotReferenceImageURLs(session, batch)
	if len(hotReferenceURLs) == 1 {
		promptText = buildStudioHotReferenceArtworkPrompt(
			studioBatchTaskHotReferencePrompt(session, batch),
			studioBatchTaskHotReferenceBrief(session, batch),
			promptText,
		)
	}
	if batch != nil {
		promptMode = strings.TrimSpace(batch.PromptMode)
	}
	productPrompts := []StudioProductImagePrompt(nil)
	if session != nil {
		if strings.TrimSpace(session.PromptMode) != "" {
			promptMode = strings.TrimSpace(session.PromptMode)
		}
		customPrompt = strings.TrimSpace(session.ProductImagePrompt)
		if parsed, err := strconv.Atoi(strings.TrimSpace(session.ProductImageCount)); err == nil && parsed > 0 {
			count = parsed
		}
		if count > maxStudioProductImageCount {
			count = maxStudioProductImageCount
		}
		for _, item := range session.ProductImagePrompts {
			productPrompts = append(productPrompts, StudioProductImagePrompt{
				Role:   strings.TrimSpace(item.Role),
				Prompt: strings.TrimSpace(item.Prompt),
			})
		}
	}
	return &StudioProductImageRequest{
		Prompt:                    promptText,
		PromptMode:                promptMode,
		ProductName:               strings.TrimSpace(selection.ProductName),
		StyleName:                 styleName,
		SourceDesignURL:           strings.TrimSpace(design.ImageURL),
		ProductReferenceImageURLs: studioBatchTaskProductReferenceImageURLs(selection),
		CustomPrompt:              customPrompt,
		ImagePrompts:              productPrompts,
		Count:                     count,
	}
}

func studioBatchTaskHotReferenceImageURLs(session *SheinStudioSession, batch *StudioBatchRecord) []string {
	if session != nil {
		if references := mergeStudioHotStyleReferenceImageURLs(nil, session.HotStyleReferenceImageURLs); len(references) > 0 {
			return references
		}
	}
	if batch != nil {
		return mergeStudioHotStyleReferenceImageURLs(nil, batch.HotStyleReferenceImageURLs)
	}
	return nil
}

func studioBatchTaskHotReferencePrompt(session *SheinStudioSession, batch *StudioBatchRecord) string {
	if session != nil && strings.TrimSpace(session.HotStyleReferencePrompt) != "" {
		return strings.TrimSpace(session.HotStyleReferencePrompt)
	}
	if batch != nil {
		return strings.TrimSpace(batch.HotStyleReferencePrompt)
	}
	return ""
}

func studioBatchTaskHotReferenceBrief(session *SheinStudioSession, batch *StudioBatchRecord) string {
	if session != nil && strings.TrimSpace(session.HotStyleReferenceBrief) != "" {
		return strings.TrimSpace(session.HotStyleReferenceBrief)
	}
	if batch != nil {
		return strings.TrimSpace(batch.HotStyleReferenceBrief)
	}
	return ""
}

func studioProductImageCategoryPath(detail *sdstemplate.ProductDetail) []string {
	if detail == nil || len(detail.Categories) == 0 {
		return nil
	}
	result := make([]string, 0, len(detail.Categories))
	for _, category := range detail.Categories {
		if name := strings.TrimSpace(category.Name); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func studioBatchTaskProductReferenceImageURLs(selection SheinStudioSelection) []string {
	result := make([]string, 0, 5)
	seen := make(map[string]struct{}, 5)
	add := func(raw string) {
		value := strings.TrimSpace(raw)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, value := range selection.SizeReferenceImageURLs {
		add(value)
	}
	for _, value := range selection.MockupImageURLs {
		add(value)
	}
	add(selection.MockupImageURL)
	add(selection.BlankDesignURL)
	add(selection.TemplateImageURL)
	for _, variant := range selection.Variants {
		for _, value := range variant.SizeReferenceImageURLs {
			add(value)
		}
		for _, value := range variant.MockupImageURLs {
			add(value)
		}
		add(variant.MockupImageURL)
		add(variant.BlankDesignURL)
		add(variant.TemplateImageURL)
	}
	if len(result) > 5 {
		return result[:5]
	}
	return result
}

func studioBatchTaskProductReferenceImageURLsForVariant(selection SheinStudioSelection, variant SheinStudioSelectionVariant) []string {
	result := make([]string, 0, 5)
	seen := make(map[string]struct{}, 5)
	add := func(raw string) {
		value := strings.TrimSpace(raw)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, value := range variant.SizeReferenceImageURLs {
		add(value)
	}
	for _, value := range variant.MockupImageURLs {
		add(value)
	}
	add(variant.MockupImageURL)
	add(variant.BlankDesignURL)
	add(variant.TemplateImageURL)
	for _, value := range studioBatchTaskProductReferenceImageURLs(selection) {
		add(value)
	}
	if len(result) > 5 {
		return result[:5]
	}
	return result
}

func studioBatchTaskColorRepresentatives(selection SheinStudioSelection) []SheinStudioSelectionVariant {
	result := make([]SheinStudioSelectionVariant, 0, len(selection.Variants))
	indexes := make(map[string]int, len(selection.Variants))
	for _, variant := range selection.Variants {
		colorKey := strings.ToLower(strings.TrimSpace(variant.Color))
		if colorKey == "" {
			colorKey = "default"
		}
		if index, ok := indexes[colorKey]; ok {
			mergeStudioBatchTaskColorVariantReferences(&result[index], variant)
			continue
		}
		indexes[colorKey] = len(result)
		result = append(result, cloneStudioBatchTaskColorVariant(variant))
	}
	return result
}

func cloneStudioBatchTaskColorVariant(input SheinStudioSelectionVariant) SheinStudioSelectionVariant {
	input.SizeReferenceImageURLs = append([]string(nil), input.SizeReferenceImageURLs...)
	input.MockupImageURLs = append([]string(nil), input.MockupImageURLs...)
	return input
}

func mergeStudioBatchTaskColorVariantReferences(target *SheinStudioSelectionVariant, source SheinStudioSelectionVariant) {
	if target == nil {
		return
	}
	target.SizeReferenceImageURLs = appendUniqueStudioBatchTaskImageURLs(target.SizeReferenceImageURLs, source.SizeReferenceImageURLs...)
	target.MockupImageURLs = appendUniqueStudioBatchTaskImageURLs(target.MockupImageURLs, source.MockupImageURLs...)
	if strings.TrimSpace(target.MockupImageURL) == "" {
		target.MockupImageURL = strings.TrimSpace(source.MockupImageURL)
	}
	if strings.TrimSpace(target.BlankDesignURL) == "" {
		target.BlankDesignURL = strings.TrimSpace(source.BlankDesignURL)
	}
	if strings.TrimSpace(target.TemplateImageURL) == "" {
		target.TemplateImageURL = strings.TrimSpace(source.TemplateImageURL)
	}
}

func appendUniqueStudioBatchTaskImageURLs(target []string, values ...string) []string {
	seen := make(map[string]struct{}, len(target)+len(values))
	for _, value := range target {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	result := append([]string(nil), target...)
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

func studioGeneratedProductImageURLs(response *StudioProductImageResponse) []string {
	if response == nil {
		return nil
	}
	result := make([]string, 0, len(response.Images))
	seen := make(map[string]struct{}, len(response.Images))
	for _, image := range response.Images {
		value := strings.TrimSpace(image.ImageURL)
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
