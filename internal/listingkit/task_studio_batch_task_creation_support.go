package listingkit

import (
	"context"
	"strconv"
	"strings"
)

func shouldResumeStudioBatchTaskCreation(ctx context.Context, repo StudioBatchRepository, batchID string) bool {
	if repo == nil {
		return false
	}
	batch, err := repo.GetStudioBatch(ctx, batchID)
	if err != nil || batch == nil {
		return false
	}
	return batch.Status == StudioBatchStatusTasksCreating
}

func (s *taskStudioBatchService) findExistingStudioBatchTask(
	ctx context.Context,
	recorded SheinStudioCreatedTaskList,
	design StudioMaterializedDesignRecord,
	grouped SheinStudioGroupedSelection,
	fallbackTitle string,
) (SheinStudioCreatedTask, bool) {
	if s == nil || s.getTask == nil || len(recorded) == 0 {
		return SheinStudioCreatedTask{}, false
	}
	designID := strings.TrimSpace(design.ID)
	for _, created := range recorded {
		if strings.TrimSpace(created.DesignID) != designID || strings.TrimSpace(created.ID) == "" {
			continue
		}
		task, err := s.getTask(ctx, created.ID)
		if err != nil || task == nil || task.Status == TaskStatusFailed {
			continue
		}
		if !studioBatchTaskMatchesSelection(task, design, grouped.Selection) {
			continue
		}
		if strings.TrimSpace(created.Title) == "" {
			created.Title = fallbackTitle
		}
		return created, true
	}
	return SheinStudioCreatedTask{}, false
}

func studioBatchTaskMatchesSelection(
	task *Task,
	design StudioMaterializedDesignRecord,
	selection SheinStudioSelection,
) bool {
	if task == nil || task.Request == nil || task.Request.Options == nil {
		return false
	}
	studio := task.Request.Options.SheinStudio
	sds := task.Request.Options.SDS
	if studio == nil || sds == nil {
		return false
	}
	if strings.TrimSpace(studio.StyleID) != buildStudioBatchTaskStyleID(design.ID) {
		return false
	}
	if len(task.Request.ImageURLs) == 0 || strings.TrimSpace(task.Request.ImageURLs[0]) != strings.TrimSpace(design.ImageURL) {
		return false
	}
	return sds.VariantID == selection.VariantID &&
		sds.ParentProductID == selection.ParentProductID &&
		sds.PrototypeGroupID == selection.PrototypeGroupID &&
		strings.TrimSpace(sds.LayerID) == strings.TrimSpace(selection.LayerID)
}

func mergeStudioCreatedTasks(
	existing SheinStudioCreatedTaskList,
	created []SheinStudioCreatedTask,
) SheinStudioCreatedTaskList {
	if len(existing) == 0 && len(created) == 0 {
		return nil
	}
	merged := make(SheinStudioCreatedTaskList, 0, len(existing)+len(created))
	seen := make(map[string]struct{}, len(existing)+len(created))
	appendIfMissing := func(task SheinStudioCreatedTask) {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		merged = append(merged, task)
	}
	for _, task := range existing {
		appendIfMissing(task)
	}
	for _, task := range created {
		appendIfMissing(task)
	}
	return merged
}

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
