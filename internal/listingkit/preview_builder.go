package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
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
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusFailed {
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
			preview.Shein = buildSheinPreviewPayload(task.Result.Shein, task.Result.CanonicalProduct, task.Result.AssetBundle, platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, "shein"))
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

func buildSheinSourceProductSummary(canonical *canonical.Product) *SheinSourceProductSummary {
	if canonical == nil {
		return nil
	}
	summary := &SheinSourceProductSummary{
		Title:        canonical.Title,
		CategoryPath: append([]string(nil), canonical.CategoryPath...),
		Attributes:   map[string]string{},
	}
	for key, attr := range canonical.Attributes {
		if strings.TrimSpace(attr.Value) != "" {
			summary.Attributes[key] = attr.Value
		}
	}
	if len(summary.Attributes) == 0 {
		summary.Attributes = nil
	}
	if canonical.Specifications != nil {
		if canonical.Specifications.Weight != nil {
			summary.VariantWeight = canonical.Specifications.Weight.Value
		}
		if canonical.Specifications.Technical != nil {
			summary.VariantSize = canonical.Specifications.Technical["size"]
			summary.VariantColor = canonical.Specifications.Technical["color"]
			summary.ProductionCycle = canonical.Specifications.Technical["production_cycle_hours"]
		}
	}
	for _, image := range canonical.Images {
		if strings.TrimSpace(image.URL) != "" {
			summary.ImageURLs = append(summary.ImageURLs, image.URL)
		}
	}
	if len(canonical.Variants) > 0 {
		variant := canonical.Variants[0]
		summary.VariantSKU = variant.SKU
		if variant.Price != nil {
			summary.VariantPrice = variant.Price.Amount
		}
		if value := variant.Attributes["Size"].Value; strings.TrimSpace(value) != "" {
			summary.VariantSize = value
		}
		if value := variant.Attributes["Color"].Value; strings.TrimSpace(value) != "" {
			summary.VariantColor = value
		}
		for _, image := range variant.Images {
			if strings.TrimSpace(image.URL) != "" {
				summary.ImageURLs = append(summary.ImageURLs, image.URL)
			}
		}
	}
	if summary.SKU == "" {
		summary.SKU = summary.Attributes["sku"]
	}
	summary.ImageURLs = uniqueStrings(summary.ImageURLs)
	return summary
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
	header.ReviewReasons = reviewReasonsFromResult(result)

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

func buildSheinPreviewPayload(pkg *sheinpub.Package, canonical *canonical.Product, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *SheinPreviewPayload {
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
		SourceProduct:     buildSheinSourceProductSummary(canonical),
		NeedsReview:       needsReview,
		Summary:           summary,
		ReviewNotes:       append([]string(nil), pkg.ReviewNotes...),
		Inspection:        pkg.Inspection,
		SubmitReadiness:   readiness,
		SubmitChecklist:   checklist,
		ImageUpload:       buildSheinImageUploadPreflight(pkg),
		ResolutionCache:   buildSheinResolutionCacheSummary(pkg),
		RepairCenter:      repairCenter,
		StatusOverview:    statusOverview,
		WorkspaceOverview: sheinworkspace.BuildWorkspaceOverview(statusOverview, toSheinWorkspaceSubmitState(readiness), toSheinWorkspaceRepairState(repairCenter)),
		EditorContext:     buildSheinEditorContext(pkg),
		ImageBundle:       pkg.ImageBundle,
		RenderPreviews:    renderPreviews,
		ScenePresets:      buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		RequestDraft:      pkg.RequestDraft,
		PreviewProduct:    pkg.PreviewProduct,
		Submission:        pkg.Submission,
		Pricing:           pkg.Pricing,
		FinalReview:       buildSheinFinalReviewPayload(pkg, canonical, readiness),
		SubmissionEvents:  append([]sheinpub.SubmissionEvent(nil), pkg.SubmissionEvents...),
		InspectionData:    pkg.Inspection,
	}
}

func buildSheinFinalReviewPayload(pkg *sheinpub.Package, canonical *canonical.Product, readiness *SheinSubmitReadiness) *SheinFinalReview {
	if pkg == nil {
		return nil
	}
	final := &SheinFinalReview{
		SourceProduct: buildSheinSourceProductSummary(canonical),
		Title:         firstNonEmpty(pkg.ProductNameEn, pkg.SpuName),
		Description:   pkg.Description,
		CategoryPath:  append([]string(nil), pkg.CategoryPath...),
		CategoryID:    pkg.CategoryID,
		Attributes:    append([]sheinpub.ResolvedAttribute(nil), pkg.ResolvedAttributes...),
		BlockingItems: cloneSheinReadinessItems(readinessBlockingItems(readiness)),
	}
	if pkg.FinalDraft != nil {
		final.Confirmed = pkg.FinalDraft.Confirmed
		final.SubmitMode = pkg.FinalDraft.SubmitMode
	}
	if pkg.SaleAttributeResolution != nil {
		final.SaleAttributes = append(final.SaleAttributes, pkg.SaleAttributeResolution.SKCAttributes...)
		final.SaleAttributes = append(final.SaleAttributes, pkg.SaleAttributeResolution.SKUAttributes...)
	}
	if pkg.RequestDraft != nil {
		final.SKUs = buildSheinFinalReviewSKUs(pkg.RequestDraft)
		final.Images = buildSheinFinalReviewImages(pkg.RequestDraft, pkg.FinalDraft, pkg.PreviewProduct)
	}
	return final
}

func readinessBlockingItems(readiness *SheinSubmitReadiness) []SheinReadinessItem {
	if readiness == nil {
		return nil
	}
	return readiness.BlockingItems
}

func cloneSheinReadinessItems(items []SheinReadinessItem) []SheinReadinessItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]SheinReadinessItem, len(items))
	copy(out, items)
	return out
}

func buildSheinFinalReviewSKUs(draft *sheinpub.RequestDraft) []SheinFinalReviewSKU {
	if draft == nil {
		return nil
	}
	out := []SheinFinalReviewSKU{}
	for _, skc := range draft.SKCList {
		for _, sku := range skc.SKUList {
			item := SheinFinalReviewSKU{
				SupplierCode: skc.SupplierCode,
				SupplierSKU:  sku.SupplierSKU,
				Price:        parseMoney(sku.BasePrice),
				Currency:     sku.Currency,
				Stock:        sku.StockCount,
				Weight:       sku.Weight,
			}
			for _, attr := range sku.SaleAttributes {
				switch strings.ToLower(strings.TrimSpace(attr.Name)) {
				case "color", "颜色":
					item.Color = attr.Value
				case "size", "尺码", "尺寸":
					item.Size = attr.Value
				}
			}
			out = append(out, item)
		}
	}
	return out
}

func buildSheinFinalReviewImages(draft *sheinpub.RequestDraft, finalDraft *sheinpub.FinalDraft, product *sheinproduct.Product) []SheinFinalReviewImage {
	if draft == nil || draft.ImageInfo == nil {
		return nil
	}
	sizeMapURLs := sheinSizeMapImageURLs(product)
	out := []SheinFinalReviewImage{}
	seen := map[string]int{}
	add := func(url, role string, sort int, main bool) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if finalDraft != nil {
			if override := strings.TrimSpace(finalDraft.ImageRoleOverrides[url]); override != "" {
				role = override
			}
			if strings.TrimSpace(finalDraft.MainImageURL) == url && role != "skc" && role != "swatch" && role != "size_map" {
				main = true
				role = "main"
			}
		}
		if _, ok := sizeMapURLs[url]; ok && role == "gallery" {
			role = "size_map"
		}
		if existingIndex, ok := seen[url]; ok {
			existing := &out[existingIndex]
			if main || role == "main" {
				existing.Role = "main"
				existing.Main = true
				existing.SizeMap = false
				existing.Swatch = false
			} else if role == "size_map" && existing.Role != "main" {
				existing.Role = "size_map"
				existing.SizeMap = true
				existing.Swatch = false
			} else if (role == "skc" || role == "swatch") && existing.Role != "main" && existing.Role != "size_map" {
				existing.Role = role
				existing.Swatch = true
			}
			return
		}
		seen[url] = len(out)
		out = append(out, SheinFinalReviewImage{
			URL:     url,
			Role:    role,
			Sort:    sort,
			Final:   true,
			Main:    main || role == "main",
			Swatch:  role == "swatch" || role == "skc",
			SizeMap: role == "size_map",
		})
	}
	add(draft.ImageInfo.MainImage, "main", 1, true)
	for i, image := range draft.ImageInfo.Gallery {
		add(image, "gallery", i+2, false)
	}
	if draft.ImageInfo.WhiteBg != "" {
		add(draft.ImageInfo.WhiteBg, "white_bg", len(out)+1, false)
	}
	for _, skc := range draft.SKCList {
		if skc.ImageInfo != nil {
			add(skc.ImageInfo.MainImage, "skc", len(out)+1, false)
		}
	}
	return out
}

func sheinSizeMapImageURLs(product *sheinproduct.Product) map[string]struct{} {
	if product == nil {
		return nil
	}
	out := map[string]struct{}{}
	add := func(info *sheinproduct.ImageInfo) {
		if info == nil {
			return
		}
		for _, image := range info.ImageInfoList {
			url := strings.TrimSpace(image.ImageURL)
			if url == "" || !image.SizeImgFlag {
				continue
			}
			out[url] = struct{}{}
		}
	}
	add(product.ImageInfo)
	for i := range product.SKCList {
		add(&product.SKCList[i].ImageInfo)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildSheinResolutionCacheSummary(pkg *sheinpub.Package) *SheinResolutionCacheSummary {
	if pkg == nil {
		return nil
	}
	summary := &SheinResolutionCacheSummary{}
	if pkg.CategoryResolution != nil {
		summary.Category = cloneSheinResolutionCacheInfo(pkg.CategoryResolution.Cache)
	}
	if pkg.AttributeResolution != nil {
		summary.Attributes = cloneSheinResolutionCacheInfo(pkg.AttributeResolution.Cache)
	}
	if pkg.SaleAttributeResolution != nil {
		summary.SaleAttributes = cloneSheinResolutionCacheInfo(pkg.SaleAttributeResolution.Cache)
	}
	if summary.Category == nil && summary.Attributes == nil && summary.SaleAttributes == nil {
		return nil
	}
	return summary
}

func cloneSheinResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo) *sheinpub.ResolutionCacheInfo {
	if info == nil {
		return nil
	}
	clone := *info
	if info.UpdatedAt != nil {
		updatedAt := *info.UpdatedAt
		clone.UpdatedAt = &updatedAt
	}
	return &clone
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
	case TaskStatusNeedsReview:
		return "任务已完成，等待人工审核"
	case TaskStatusFailed:
		return "任务执行失败，暂无预览结果"
	default:
		return ""
	}
}
