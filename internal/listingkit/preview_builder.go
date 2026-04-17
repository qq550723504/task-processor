package listingkit

import "strings"

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
	preview.RevisionHistoryMeta = buildRevisionHistoryMeta(task.Result)
	preview.RevisionHistory = buildRevisionHistoryPreviewItems(task.Result.RevisionHistory)

	if selectedPlatform == "" || selectedPlatform == "amazon" {
		if task.Result.Amazon != nil {
			preview.Amazon = buildAmazonPreviewPayload(task.Result.Amazon)
		} else if selectedPlatform == "amazon" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "shein" {
		if task.Result.Shein != nil {
			preview.Shein = buildSheinPreviewPayload(task.Result.Shein)
			preview.NeedsReview = preview.NeedsReview || preview.Shein.NeedsReview
		} else if selectedPlatform == "shein" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "temu" {
		if task.Result.Temu != nil {
			preview.Temu = buildTemuPreviewPayload(task.Result.Temu)
			preview.NeedsReview = preview.NeedsReview || preview.Temu.NeedsReview
		} else if selectedPlatform == "temu" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "walmart" {
		if task.Result.Walmart != nil {
			preview.Walmart = buildWalmartPreviewPayload(task.Result.Walmart)
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

	cards := []ListingKitPlatformCard{}
	if selectedPlatform == "" || selectedPlatform == "amazon" {
		if card, ok := buildAmazonPreviewCard(result.Amazon); ok {
			cards = append(cards, card)
		}
	}
	if selectedPlatform == "" || selectedPlatform == "shein" {
		if card, ok := buildSheinPreviewCard(result.Shein); ok {
			cards = append(cards, card)
		}
	}
	if selectedPlatform == "" || selectedPlatform == "temu" {
		if card, ok := buildTemuPreviewCard(result.Temu); ok {
			cards = append(cards, card)
		}
	}
	if selectedPlatform == "" || selectedPlatform == "walmart" {
		if card, ok := buildWalmartPreviewCard(result.Walmart); ok {
			cards = append(cards, card)
		}
	}
	header.PlatformCards = cards
	return header
}

func buildAmazonPreviewPayload(pkg *AmazonPackage) *AmazonPreviewPayload {
	if pkg == nil || pkg.Draft == nil {
		return nil
	}
	return &AmazonPreviewPayload{
		Title:       pkg.Draft.Title,
		Brand:       pkg.Draft.Brand,
		ProductType: pkg.Draft.ProductType,
		Draft:       pkg.Draft,
	}
}

func buildSheinPreviewPayload(pkg *SheinPackage) *SheinPreviewPayload {
	if pkg == nil {
		return nil
	}
	needsReview := len(pkg.ReviewNotes) > 0
	summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))
	readiness := buildSheinSubmitReadiness(pkg)
	checklist := buildSheinSubmitChecklist(readiness)
	repairCenter := buildSheinRepairCenter(readiness, checklist)
	statusOverview := buildSheinStatusOverview(pkg, readiness)
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
		WorkspaceOverview: buildSheinWorkspaceOverview(statusOverview, readiness, repairCenter),
		EditorContext:     buildSheinEditorContext(pkg),
		RequestDraft:      pkg.RequestDraft,
		PreviewProduct:    pkg.PreviewProduct,
	}
}

func buildTemuPreviewPayload(pkg *TemuPackage) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &TemuPreviewPayload{
		Headline:    pkg.GoodsName,
		NeedsReview: len(pkg.ReviewNotes) > 0,
		ReviewNotes: append([]string(nil), pkg.ReviewNotes...),
		Package:     pkg,
	}
}

func buildWalmartPreviewPayload(pkg *WalmartPackage) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &WalmartPreviewPayload{
		Headline:    pkg.ProductName,
		NeedsReview: len(pkg.ReviewNotes) > 0,
		ReviewNotes: append([]string(nil), pkg.ReviewNotes...),
		Package:     pkg,
	}
}

func buildAmazonPreviewCard(pkg *AmazonPackage) (ListingKitPlatformCard, bool) {
	if pkg == nil || pkg.Draft == nil {
		return ListingKitPlatformCard{}, false
	}
	return ListingKitPlatformCard{
		Platform: "amazon",
		Status:   "ready",
		Summary:  firstNonEmpty(pkg.Draft.Title, "已生成 Amazon 草稿"),
	}, true
}

func buildSheinPreviewCard(pkg *SheinPackage) (ListingKitPlatformCard, bool) {
	if pkg == nil {
		return ListingKitPlatformCard{}, false
	}
	status := "ready"
	summary := firstNonEmpty(pkg.SpuName, pkg.ProductNameEn, "已生成 SHEIN 预览")
	needsReview := len(pkg.ReviewNotes) > 0
	if pkg.Inspection != nil {
		needsReview = needsReview || pkg.Inspection.NeedsReview
		if pkg.Inspection.NeedsReview {
			status = "needs_review"
		}
		summary = firstNonEmpty(joinStrings(pkg.Inspection.Summary, "；"), summary)
	}
	return ListingKitPlatformCard{
		Platform:    "shein",
		Status:      status,
		Summary:     summary,
		NeedsReview: needsReview,
	}, true
}

func buildTemuPreviewCard(pkg *TemuPackage) (ListingKitPlatformCard, bool) {
	if pkg == nil {
		return ListingKitPlatformCard{}, false
	}
	return ListingKitPlatformCard{
		Platform:    "temu",
		Status:      previewStatusFromReviewNotes(pkg.ReviewNotes),
		Summary:     firstNonEmpty(pkg.GoodsName, "已生成 TEMU 资料包"),
		NeedsReview: len(pkg.ReviewNotes) > 0,
	}, true
}

func buildWalmartPreviewCard(pkg *WalmartPackage) (ListingKitPlatformCard, bool) {
	if pkg == nil {
		return ListingKitPlatformCard{}, false
	}
	return ListingKitPlatformCard{
		Platform:    "walmart",
		Status:      previewStatusFromReviewNotes(pkg.ReviewNotes),
		Summary:     firstNonEmpty(pkg.ProductName, "已生成 Walmart 资料包"),
		NeedsReview: len(pkg.ReviewNotes) > 0,
	}, true
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
