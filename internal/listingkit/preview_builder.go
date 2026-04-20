package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	sheinpub "task-processor/internal/publishing/shein"
	sheinworkspace "task-processor/internal/workspace/shein"
)

func buildListingKitPreview(task *Task, selectedPlatform string) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskNotFound
	}

	selectedPlatform = strings.ToLower(strings.TrimSpace(selectedPlatform))
	if selectedPlatform != "" && len(normalizePlatforms([]string{selectedPlatform})) == 0 {
		return nil, ErrUnsupportedPreviewPlatform
	}

	preview := &ListingKitPreview{
		TaskID:           task.ID,
		Status:           task.Status,
		SelectedPlatform: selectedPlatform,
		Platforms:        previewPlatforms(task),
		CreatedAt:        task.CreatedAt,
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
		completedAt := task.UpdatedAt
		preview.CompletedAt = &completedAt
	}

	if task.Result == nil {
		preview.Overview = &ListingKitPreviewHeader{
			StatusMessage: previewStatusMessage(task.Status),
		}
		return preview, nil
	}

	preview.Overview = buildPreviewHeader(task.Result, selectedPlatform)
	preview.NeedsReview = task.Result.Summary != nil && task.Result.Summary.NeedsReview
	preview.Catalog = task.Result.CatalogProduct
	preview.Assets = task.Result.AssetBundle
	preview.AssetInventory = task.Result.AssetInventorySummary
	preview.AssetRenderPreviews = append([]AssetRenderPreview(nil), task.Result.AssetRenderPreviews...)
	preview.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), task.Result.PlatformAssetRenderPreviews...)
	if len(preview.AssetRenderPreviews) == 0 {
		preview.AssetRenderPreviews = buildAssetRenderPreviews(task.Result.AssetBundle)
	}
	if len(preview.PlatformAssetRenderPreviews) == 0 {
		preview.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(task.Result)
	}
	preview.PlatformAssetRenderPreviews = filterPlatformAssetRenderPreviews(preview.PlatformAssetRenderPreviews, selectedPlatform)
	preview.AssetGenerationQueue = task.Result.AssetGenerationQueue
	preview.AssetGenerationOverview = task.Result.AssetGenerationOverview
	preview.RevisionHistoryMeta = buildRevisionHistoryMeta(task.Result)
	preview.RevisionHistory = buildRevisionHistoryPreviewItems(task.Result.RevisionHistory)

	if selectedPlatform == "" || selectedPlatform == "amazon" {
		if task.Result.Amazon != nil {
			preview.Amazon = buildAmazonPreviewPayload(task.Result.Amazon, task.Result.AssetBundle, platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, "amazon"))
		} else if selectedPlatform == "amazon" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "shein" {
		if task.Result.Shein != nil {
			preview.Shein = buildSheinPreviewPayload(task.Result.Shein, task.Result.AssetBundle, platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, "shein"))
			preview.NeedsReview = preview.NeedsReview || preview.Shein.NeedsReview
		} else if selectedPlatform == "shein" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "temu" {
		if task.Result.Temu != nil {
			preview.Temu = buildTemuPreviewPayload(task.Result.Temu, task.Result.AssetBundle, platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, "temu"))
			preview.NeedsReview = preview.NeedsReview || preview.Temu.NeedsReview
		} else if selectedPlatform == "temu" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "walmart" {
		if task.Result.Walmart != nil {
			preview.Walmart = buildWalmartPreviewPayload(task.Result.Walmart, task.Result.AssetBundle, platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, "walmart"))
			preview.NeedsReview = preview.NeedsReview || preview.Walmart.NeedsReview
		} else if selectedPlatform == "walmart" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	return preview, nil
}

func buildRevisionHistoryPreviewItems(records []ListingKitRevisionRecord) []ListingKitRevisionRecord {
	if len(records) == 0 {
		return nil
	}
	items := make([]ListingKitRevisionRecord, 0, len(records))
	for i, record := range records {
		items = append(items, withRevisionHistoryRecordID(record, i))
	}
	return items
}

func previewPlatforms(task *Task) []string {
	if task == nil {
		return nil
	}
	if task.Result != nil && len(task.Result.Platforms) > 0 {
		return append([]string(nil), task.Result.Platforms...)
	}
	if task.Request != nil && len(task.Request.Platforms) > 0 {
		return append([]string(nil), task.Request.Platforms...)
	}
	return nil
}

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	if result == nil {
		return nil
	}

	header := &ListingKitPreviewHeader{
		Country:       result.Country,
		Language:      result.Language,
		StatusMessage: "预览结果已生成",
	}
	if result.Summary != nil {
		header.SourceType = result.Summary.SourceType
		header.ImageCount = result.Summary.ImageCount
		header.VariantCount = result.Summary.VariantCount
		header.Warnings = append([]string(nil), result.Summary.Warnings...)
	}

	header.PlatformCards = buildPlatformPreviewCards(result, selectedPlatform)
	return header
}

func buildAmazonPreviewPayload(pkg *AmazonPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *AmazonPreviewPayload {
	if pkg == nil || pkg.Draft == nil {
		return nil
	}
	return &AmazonPreviewPayload{
		Title:          pkg.Draft.Title,
		Brand:          pkg.Draft.Brand,
		ProductType:    pkg.Draft.ProductType,
		ImageBundle:    pkg.ImageBundle,
		RenderPreviews: renderPreviews,
		ScenePresets:   buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		Draft:          pkg.Draft,
	}
}

func buildSheinPreviewPayload(pkg *sheinpub.Package, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *SheinPreviewPayload {
	if pkg == nil {
		return nil
	}
	needsReview := len(pkg.ReviewNotes) > 0
	summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))
	readiness := buildSheinSubmitReadiness(pkg)
	checklist := buildSheinSubmitChecklist(readiness)
	repairCenter := buildSheinRepairCenter(readiness, checklist)
	statusOverview := sheinworkspace.BuildStatusOverview(pkg.Inspection, toSheinWorkspaceSubmitState(readiness))
	if pkg.Inspection != nil {
		needsReview = needsReview || pkg.Inspection.NeedsReview
		summary = uniqueStrings(append(summary, pkg.Inspection.Summary...))
	}
	return &SheinPreviewPayload{
		Headline:          firstNonEmpty(pkg.SpuName, pkg.ProductNameEn),
		BrandName:         pkg.BrandName,
		CategoryPath:      append([]string(nil), pkg.CategoryPath...),
		CategoryID:        pkg.CategoryID,
		NeedsReview:       needsReview,
		Summary:           summary,
		ReviewNotes:       append([]string(nil), pkg.ReviewNotes...),
		Inspection:        pkg.Inspection,
		SubmitReadiness:   readiness,
		SubmitChecklist:   checklist,
		RepairCenter:      repairCenter,
		StatusOverview:    statusOverview,
		WorkspaceOverview: sheinworkspace.BuildWorkspaceOverview(statusOverview, toSheinWorkspaceSubmitState(readiness), toSheinWorkspaceRepairState(repairCenter)),
		EditorContext:     buildSheinEditorContext(pkg),
		ImageBundle:       pkg.ImageBundle,
		RenderPreviews:    renderPreviews,
		ScenePresets:      buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		RequestDraft:      pkg.RequestDraft,
		PreviewProduct:    pkg.PreviewProduct,
		InspectionData:    pkg.Inspection,
	}
}

func toSheinWorkspaceSubmitState(readiness *SheinSubmitReadiness) *sheinworkspace.SubmitStateInput {
	if readiness == nil {
		return nil
	}
	return &sheinworkspace.SubmitStateInput{
		Status:        readiness.Status,
		Ready:         readiness.Ready,
		Summary:       append([]string(nil), readiness.Summary...),
		BlockingItems: toSheinWorkspaceActionItems(readiness.BlockingItems),
		WarningItems:  toSheinWorkspaceActionItems(readiness.WarningItems),
	}
}

func toSheinWorkspaceActionItems(items []SheinReadinessItem) []sheinworkspace.ActionItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]sheinworkspace.ActionItem, 0, len(items))
	for _, item := range items {
		out = append(out, sheinworkspace.ActionItem{
			Key:             item.Key,
			SuggestedAction: item.SuggestedAction,
		})
	}
	return out
}

func toSheinWorkspaceRepairState(center *SheinRepairCenter) *sheinworkspace.RepairStateInput {
	if center == nil {
		return nil
	}
	out := &sheinworkspace.RepairStateInput{
		Status:             center.Status,
		TotalActions:       safeRepairActionCount(center),
		DirectApplyActions: safeRepairDirectApplyCount(center),
		PrimaryPlanStatus:  safeRepairPlanStatus(center),
		SessionStatus:      safeRepairSessionStatus(center),
		Summary:            append([]string(nil), center.Summary...),
	}
	if center.PrimaryAction != nil {
		out.PrimaryAction = center.PrimaryAction.SuggestedAction
		out.PrimaryActionKey = center.PrimaryAction.Key
	}
	if center.Session != nil {
		out.Session = &sheinworkspace.SessionInput{
			Status:        center.Session.Status,
			CurrentStepID: center.Session.CurrentStepID,
			NextStepID:    center.Session.NextStepID,
			RefreshBlocks: append([]string(nil), center.Session.RefreshBlocks...),
		}
		if center.Session.ResumeState != nil {
			out.Session.ResumeMode = center.Session.ResumeState.ResumeMode
			if out.Session.CurrentStepID == "" {
				out.Session.CurrentStepID = center.Session.ResumeState.ResumeStepID
			}
			if len(out.Session.RefreshBlocks) == 0 {
				out.Session.RefreshBlocks = append([]string(nil), center.Session.ResumeState.RefreshBlocks...)
			}
		}
	}
	return out
}

func buildTemuPreviewPayload(pkg *TemuPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &TemuPreviewPayload{
		Headline:       pkg.GoodsName,
		NeedsReview:    len(pkg.ReviewNotes) > 0,
		ReviewNotes:    append([]string(nil), pkg.ReviewNotes...),
		ImageBundle:    pkg.ImageBundle,
		RenderPreviews: renderPreviews,
		ScenePresets:   buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		Package:        pkg,
	}
}

func buildWalmartPreviewPayload(pkg *WalmartPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &WalmartPreviewPayload{
		Headline:       pkg.ProductName,
		NeedsReview:    len(pkg.ReviewNotes) > 0,
		ReviewNotes:    append([]string(nil), pkg.ReviewNotes...),
		ImageBundle:    pkg.ImageBundle,
		RenderPreviews: renderPreviews,
		ScenePresets:   buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		Package:        pkg,
	}
}

func previewStatusFromReviewNotes(reviewNotes []string) string {
	if len(reviewNotes) > 0 {
		return "needs_review"
	}
	return "ready"
}

func previewStatusMessage(status TaskStatus) string {
	switch status {
	case TaskStatusPending:
		return "任务已创建，预览结果尚未生成"
	case TaskStatusProcessing:
		return "任务处理中，预览结果尚未准备完成"
	case TaskStatusFailed:
		return "任务执行失败，暂无预览结果"
	default:
		return ""
	}
}
