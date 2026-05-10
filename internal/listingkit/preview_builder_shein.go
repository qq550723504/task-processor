package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheinworkspace "task-processor/internal/workspace/shein"
)

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
