package listingkit

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	studiodomain "task-processor/internal/listing/studio"
)

func aggregateStudioBatchStatus(items []StudioBatchItemRecord) StudioBatchStatus {
	return studiodomain.AggregateBatchStatus(items, studioBatchItemStatus, studioBatchStatusSet())
}

func buildStudioBatchItemDesignRequest(batch *StudioBatchRecord, item StudioBatchItemRecord) *StudioDesignRequest {
	selection := firstStudioBatchItemSelection(batch, item)
	hotStyleReferencePrompt := strings.TrimSpace(batch.HotStyleReferencePrompt)
	hotStyleReferenceBrief := strings.TrimSpace(batch.HotStyleReferenceBrief)
	includeHotStyleReference := hotStyleReferencePrompt != "" && hotStyleReferenceBrief != ""
	referenceImageURLs := []string(nil)
	if includeHotStyleReference {
		referenceImageURLs = mergeStudioHotStyleReferenceImageURLs(
			nil,
			batch.HotStyleReferenceImageURLs,
		)
	} else {
		hotStyleReferencePrompt = ""
	}
	artworkGenerationMode := studioArtworkGenerationModeThemePrompt
	if len(referenceImageURLs) == 1 {
		artworkGenerationMode = studioArtworkGenerationModeHotReference
	}
	return &StudioDesignRequest{
		Prompt:                    buildStudioHotStyleGenerationPrompt(batch.Prompt, hotStyleReferencePrompt),
		ArtworkGenerationMode:     artworkGenerationMode,
		PromptMode:                strings.TrimSpace(batch.PromptMode),
		Count:                     parseStudioBatchRunStyleCount(strings.TrimSpace(batch.StyleCount)),
		VariationIntensity:        strings.TrimSpace(batch.VariationIntensity),
		PrintableWidth:            selection.PrintableWidth,
		PrintableHeight:           selection.PrintableHeight,
		ProductReferenceImageURLs: referenceImageURLs,
		ImageModel:                strings.TrimSpace(batch.ArtworkModel),
		TransparentBackground:     batch.TransparentBackground,
	}
}

func firstStudioBatchItemSelection(batch *StudioBatchRecord, item StudioBatchItemRecord) SheinStudioSelection {
	selections := resolveStudioBatchItemSelections(batch, item)
	if len(selections) == 0 {
		return SheinStudioSelection(batch.Selection)
	}
	return selections[0].Selection
}

func studioBatchItemReferenceImageURLs(batch *StudioBatchRecord, item StudioBatchItemRecord) []string {
	selections := resolveStudioBatchItemSelections(batch, item)
	seen := make(map[string]struct{})
	result := make([]string, 0, len(selections)*4+len(batch.SelectedSDSImages))
	appendURL := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	for _, grouped := range selections {
		appendURL(grouped.Selection.MockupImageURL)
		for _, value := range grouped.Selection.MockupImageURLs {
			appendURL(value)
		}
		for _, value := range grouped.Selection.SizeReferenceImageURLs {
			appendURL(value)
		}
	}
	for _, image := range batch.SelectedSDSImages {
		appendURL(image.ImageURL)
	}
	return result
}

func resolveStudioBatchItemSelections(batch *StudioBatchRecord, item StudioBatchItemRecord) []SheinStudioGroupedSelection {
	selectionMap := studioBatchSelectionSnapshotMap(batch)
	selections := make([]SheinStudioGroupedSelection, 0, len(item.SelectionIDs))
	for _, selectionID := range item.SelectionIDs {
		grouped, ok := selectionMap[strings.TrimSpace(selectionID)]
		if !ok {
			continue
		}
		selections = append(selections, grouped)
	}
	return selections
}

func expandStudioBatchItems(batch *StudioBatchRecord) []StudioBatchItemRecord {
	groupedSelections := studioBatchAllGroupedSelections(batch)
	if len(groupedSelections) == 0 {
		return nil
	}

	items := make([]StudioBatchItemRecord, 0, len(groupedSelections))
	if strings.TrimSpace(batch.GroupedImageMode) == "per_product" {
		for index, grouped := range groupedSelections {
			selectionID := strings.TrimSpace(grouped.SelectionID)
			if selectionID == "" {
				continue
			}
			items = append(items, StudioBatchItemRecord{
				ID:               buildStudioBatchItemID(batch.ID, index),
				BatchID:          batch.ID,
				TargetGroupKey:   selectionID,
				TargetGroupLabel: buildStudioBatchPerProductLabel(grouped.Selection),
				SelectionIDs:     SheinStudioStringList{selectionID},
				GroupMode:        "per_product",
				Status:           StudioBatchItemStatusPending,
				SelectionCount:   1,
			})
		}
		return items
	}

	type bucket struct {
		key          string
		label        string
		groupMode    string
		selectionIDs []string
	}
	buckets := make([]bucket, 0)
	bucketIndex := make(map[string]int)
	for _, grouped := range groupedSelections {
		selectionID := strings.TrimSpace(grouped.SelectionID)
		if selectionID == "" {
			continue
		}
		key := buildStudioBatchSharedCompatibilityGroupKey(grouped.Selection)
		label := buildStudioBatchSharedBySizeGroupLabel(grouped.Selection)
		groupMode := "shared_by_size"
		if key == "" {
			key = selectionID
			label = buildStudioBatchPerProductLabel(grouped.Selection)
			groupMode = "per_product"
		}
		index, ok := bucketIndex[key]
		if !ok {
			bucketIndex[key] = len(buckets)
			buckets = append(buckets, bucket{
				key:          key,
				label:        label,
				groupMode:    groupMode,
				selectionIDs: []string{selectionID},
			})
			continue
		}
		buckets[index].selectionIDs = append(buckets[index].selectionIDs, selectionID)
	}
	for index, bucket := range buckets {
		items = append(items, StudioBatchItemRecord{
			ID:               buildStudioBatchItemID(batch.ID, index),
			BatchID:          batch.ID,
			TargetGroupKey:   bucket.key,
			TargetGroupLabel: bucket.label,
			SelectionIDs:     append(SheinStudioStringList(nil), bucket.selectionIDs...),
			GroupMode:        bucket.groupMode,
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   len(bucket.selectionIDs),
		})
	}
	return items
}

func studioBatchAllGroupedSelections(batch *StudioBatchRecord) []SheinStudioGroupedSelection {
	if batch == nil {
		return nil
	}

	result := make([]SheinStudioGroupedSelection, 0, len(batch.GroupedSelections)+1)
	seen := make(map[string]struct{}, len(batch.GroupedSelections)+1)
	appendGrouped := func(grouped SheinStudioGroupedSelection) {
		selectionID := strings.TrimSpace(grouped.SelectionID)
		if selectionID == "" || grouped.Selection.VariantID <= 0 {
			return
		}
		if _, ok := seen[selectionID]; ok {
			return
		}
		seen[selectionID] = struct{}{}
		result = append(result, grouped)
	}

	primary := SheinStudioSelection(batch.Selection)
	if primary.VariantID > 0 {
		appendGrouped(SheinStudioGroupedSelection{
			SelectionID: selectionIDForStudioSelection(primary),
			Selection:   primary,
			Eligible:    true,
		})
	}
	for _, grouped := range batch.GroupedSelections {
		if !grouped.Eligible {
			continue
		}
		appendGrouped(grouped)
	}
	return result
}

func studioBatchSelectionSnapshotMap(batch *StudioBatchRecord) map[string]SheinStudioGroupedSelection {
	selections := studioBatchAllGroupedSelections(batch)
	result := make(map[string]SheinStudioGroupedSelection, len(selections))
	for _, grouped := range selections {
		result[strings.TrimSpace(grouped.SelectionID)] = grouped
	}
	return result
}

func selectionIDForStudioSelection(selection SheinStudioSelection) string {
	if selection.VariantID <= 0 {
		return ""
	}
	selectedVariantIDs := append([]int64(nil), selection.SelectedVariantIDs...)
	if len(selectedVariantIDs) == 0 {
		for _, variant := range selection.Variants {
			if variant.VariantID > 0 {
				selectedVariantIDs = append(selectedVariantIDs, variant.VariantID)
			}
		}
	}
	tokens := make([]string, 0, len(selectedVariantIDs))
	for _, id := range selectedVariantIDs {
		tokens = append(tokens, strconv.FormatInt(id, 10))
	}
	parentProductID := selection.ParentProductID
	if parentProductID <= 0 {
		parentProductID = selection.ProductID
	}
	return strings.Join([]string{
		strconv.FormatInt(parentProductID, 10),
		strconv.FormatInt(selection.PrototypeGroupID, 10),
		strconv.FormatInt(selection.VariantID, 10),
		strings.TrimSpace(selection.LayerID),
		strings.Join(tokens, ","),
	}, ":")
}

func buildStudioBatchSharedBySizeGroupKey(selection SheinStudioSelection) string {
	return fmt.Sprintf("size:%dx%d", selection.PrintableWidth, selection.PrintableHeight)
}

func buildStudioBatchSharedCompatibilityGroupKey(selection SheinStudioSelection) string {
	if !studioBatchCompatibilityFingerprintComplete(selection) {
		return ""
	}
	return "compat:" + buildStudioBatchCompatibilityFingerprint(selection)
}

func buildStudioBatchSharedBySizeGroupLabel(selection SheinStudioSelection) string {
	if selection.PrintableWidth > 0 && selection.PrintableHeight > 0 {
		return fmt.Sprintf("%d x %d", selection.PrintableWidth, selection.PrintableHeight)
	}
	return "自动尺寸"
}

func buildStudioBatchPerProductLabel(selection SheinStudioSelection) string {
	productName := strings.TrimSpace(selection.ProductName)
	if productName == "" {
		productName = "SDS 商品"
	}
	variantLabel := strings.TrimSpace(selection.VariantLabel)
	if variantLabel == "" {
		return productName
	}
	return fmt.Sprintf("%s · %s", productName, variantLabel)
}

func buildStudioBatchItemID(batchID string, index int) string {
	return fmt.Sprintf("%s:item:%d", strings.TrimSpace(batchID), index+1)
}

func buildStudioBatchAttemptID(itemID string, attemptNo int) string {
	return fmt.Sprintf("%s:attempt:%d", strings.TrimSpace(itemID), attemptNo)
}

func buildStudioBatchDesignID(attemptID string, index int) string {
	return fmt.Sprintf("%s:design:%d", strings.TrimSpace(attemptID), index+1)
}

func timePtr(value time.Time) *time.Time {
	return &value
}
