package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func buildSheinPreviewPayload(pkg *sheinpub.Package, pod *PodExecutionSummary, canonical *canonical.Product, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *SheinPreviewPayload {
	if pkg == nil {
		return nil
	}
	sheinpub.NormalizePackageSemanticFields(pkg)
	needsReview := len(pkg.ReviewNotes) > 0
	summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	readiness := projection.Readiness
	checklist := projection.Checklist
	repairCenter := buildSheinRepairCenter(readiness, checklist)
	submitState := projection.SubmitState
	repairState := sheinworkspace.BuildRepairStateInput(repairCenter)
	statusOverview := projection.StatusOverview
	if pkg.Inspection != nil {
		needsReview = needsReview || pkg.Inspection.NeedsReview
		summary = uniqueStrings(append(summary, pkg.Inspection.Summary...))
	}
	payload := &SheinPreviewPayload{
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
		WorkspaceOverview: sheinworkspace.BuildWorkspaceOverview(statusOverview, submitState, repairState),
		EditorContext:     buildSheinEditorContext(pkg),
		ImageBundle:       pkg.ImageBundle,
		RenderPreviews:    renderPreviews,
		ScenePresets:      buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		DraftPayload:      pkg.DraftPayload,
		PreviewPayload:    pkg.PreviewPayload,
		SubmissionState:   pkg.SubmissionState,
		Pricing:           pkg.Pricing,
		FinalReview:       buildSheinFinalReviewPayload(pkg, canonical, readiness),
		SubmissionEvents:  append([]sheinpub.SubmissionEvent(nil), pkg.SubmissionEvents...),
		InspectionData:    pkg.Inspection,
	}
	return normalizeSheinPreviewPayloadSemanticFields(payload)
}

func buildSheinResolutionCacheSummary(pkg *sheinpub.Package) *SheinResolutionCacheSummary {
	if pkg == nil {
		return nil
	}
	summary := &SheinResolutionCacheSummary{}
	if pkg.CategoryResolution != nil {
		summary.Category = sheinpub.CloneResolutionCacheInfo(pkg.CategoryResolution.Cache)
	}
	if pkg.AttributeResolution != nil {
		summary.Attributes = sheinpub.CloneResolutionCacheInfo(pkg.AttributeResolution.Cache)
	}
	if pkg.SaleAttributeResolution != nil {
		summary.SaleAttributes = sheinpub.CloneResolutionCacheInfo(pkg.SaleAttributeResolution.Cache)
	}
	if pkg.Pricing != nil {
		summary.Pricing = sheinpub.CloneResolutionCacheInfo(pkg.Pricing.Cache)
	}
	if summary.Category == nil && summary.Attributes == nil && summary.SaleAttributes == nil && summary.Pricing == nil {
		return nil
	}
	return summary
}

func buildSheinFinalReviewPayload(pkg *sheinpub.Package, canonical *canonical.Product, readiness *SheinSubmitReadiness) *SheinFinalReview {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
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
		BlockingItems: sheinworkspace.CloneReadinessItems(readiness.BlockingItems),
	}
	if pkg.FinalSubmissionDraft != nil {
		final.Confirmed = pkg.FinalSubmissionDraft.Confirmed
		final.SubmitMode = pkg.FinalSubmissionDraft.SubmitMode
	}
	if pkg.SaleAttributeResolution != nil {
		final.SaleAttributes = append(final.SaleAttributes, pkg.SaleAttributeResolution.SKCAttributes...)
		final.SaleAttributes = append(final.SaleAttributes, pkg.SaleAttributeResolution.SKUAttributes...)
	}
	if pkg.DraftPayload != nil {
		final.SKUs = buildSheinFinalReviewSKUs(pkg.DraftPayload)
		final.Images = buildSheinFinalReviewImages(pkg.DraftPayload, pkg.FinalSubmissionDraft, pkg.PreviewPayload)
	}
	return final
}

func buildSheinFinalReviewSKUs(draft *sheinpub.RequestDraft) []SheinFinalReviewSKU {
	if draft == nil {
		return nil
	}
	out := []SheinFinalReviewSKU{}
	for _, skc := range draft.SKCList {
		for _, sku := range skc.SKUList {
			out = append(out, buildSheinFinalReviewSKU(skc.SupplierCode, sku))
		}
	}
	return out
}

func buildSheinFinalReviewImages(draft *sheinpub.RequestDraft, finalDraft *sheinpub.FinalDraft, product *sheinproduct.Product) []SheinFinalReviewImage {
	if draft == nil || draft.ImageInfo == nil {
		return nil
	}
	sizeMapURLs := sheinproduct.CollectSizeMapImageURLs(product)
	out := []SheinFinalReviewImage{}
	seen := map[string]int{}
	add := func(url, role string, sort int, main bool) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		role, main = resolveSheinFinalReviewImageRole(url, role, main, finalDraft, sizeMapURLs)
		if existingIndex, ok := seen[url]; ok {
			mergeSheinFinalReviewImage(&out[existingIndex], role, main)
			return
		}
		seen[url] = len(out)
		out = append(out, SheinFinalReviewImage{
			URL:     url,
			Role:    role,
			Sort:    sort,
			Final:   true,
			Main:    main || role == "main",
			Swatch:  isSheinFinalReviewSwatchRole(role),
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

func buildSheinFinalReviewSKU(supplierCode string, sku SheinSKUDraft) SheinFinalReviewSKU {
	item := SheinFinalReviewSKU{
		SupplierCode: supplierCode,
		SupplierSKU:  sku.SupplierSKU,
		Price:        parseMoney(sku.BasePrice),
		Currency:     sku.Currency,
		Stock:        sku.StockCount,
		Weight:       sku.Weight,
	}
	for _, attr := range sku.SaleAttributes {
		switch normalizeSheinFinalReviewAttributeName(attr.Name) {
		case "color":
			item.Color = attr.Value
		case "size":
			item.Size = attr.Value
		}
	}
	return item
}

func normalizeSheinFinalReviewAttributeName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "color", "颜色":
		return "color"
	case "size", "尺码", "尺寸":
		return "size"
	default:
		return ""
	}
}

func resolveSheinFinalReviewImageRole(url, role string, main bool, finalDraft *sheinpub.FinalDraft, sizeMapURLs map[string]struct{}) (string, bool) {
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
	return role, main
}

func isSheinFinalReviewSwatchRole(role string) bool {
	return role == "swatch" || role == "skc"
}

func mergeSheinFinalReviewImage(existing *SheinFinalReviewImage, role string, main bool) {
	if existing == nil {
		return
	}
	switch {
	case main || role == "main":
		existing.Role = "main"
		existing.Main = true
		existing.SizeMap = false
		existing.Swatch = false
	case role == "size_map" && existing.Role != "main":
		existing.Role = "size_map"
		existing.SizeMap = true
		existing.Swatch = false
	case isSheinFinalReviewSwatchRole(role) && existing.Role != "main" && existing.Role != "size_map":
		existing.Role = role
		existing.Swatch = true
	}
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
