package listingkit

import (
	"fmt"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
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
		enrichCategoryResolutionCacheInfo(summary.Category, pkg.CategoryResolution)
	}
	if pkg.AttributeResolution != nil {
		summary.Attributes = sheinpub.CloneResolutionCacheInfo(pkg.AttributeResolution.Cache)
		enrichAttributeResolutionCacheInfo(summary.Attributes, pkg.AttributeResolution)
	}
	if pkg.SaleAttributeResolution != nil {
		summary.SaleAttributes = sheinpub.CloneResolutionCacheInfo(pkg.SaleAttributeResolution.Cache)
		enrichSaleAttributeResolutionCacheInfo(summary.SaleAttributes, pkg.SaleAttributeResolution)
	}
	if pkg.Pricing != nil {
		summary.Pricing = sheinpub.CloneResolutionCacheInfo(pkg.Pricing.Cache)
		enrichPricingResolutionCacheInfo(summary.Pricing, pkg.Pricing)
	}
	if summary.Category == nil && summary.Attributes == nil && summary.SaleAttributes == nil && summary.Pricing == nil {
		return nil
	}
	return summary
}

func enrichCategoryResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, resolution *sheinpub.CategoryResolution) {
	if info == nil || resolution == nil {
		return
	}
	info.DisplayValue = strings.TrimSpace(strings.Join(resolution.MatchedPath, " > "))
}

func enrichAttributeResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, resolution *sheinpub.AttributeResolution) {
	if info == nil || resolution == nil {
		return
	}
	parts := make([]string, 0, 4)
	if resolution.ResolvedCount > 0 {
		parts = append(parts, fmt.Sprintf("已解析 %d 个", resolution.ResolvedCount))
	}
	for _, item := range resolution.ResolvedAttributes {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", name, value))
		if len(parts) >= 4 {
			break
		}
	}
	if resolution.UnresolvedCount > 0 {
		parts = append(parts, fmt.Sprintf("待补充 %d 个", resolution.UnresolvedCount))
	}
	info.DisplayValue = strings.TrimSpace(strings.Join(parts, "；"))
}

func enrichSaleAttributeResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, resolution *sheinpub.SaleAttributeResolution) {
	if info == nil || resolution == nil {
		return
	}
	if len(resolution.SelectionSummary) > 0 {
		info.DisplayValue = strings.TrimSpace(strings.Join(resolution.SelectionSummary, "；"))
		return
	}
	parts := make([]string, 0, 4)
	for _, item := range resolution.SKCAttributes {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("SKC %s=%s", name, value))
		if len(parts) >= 2 {
			break
		}
	}
	for _, item := range resolution.SKUAttributes {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("SKU %s=%s", name, value))
		if len(parts) >= 4 {
			break
		}
	}
	info.DisplayValue = strings.TrimSpace(strings.Join(parts, "；"))
}

func enrichPricingResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, review *sheinpub.PricingReview) {
	if info == nil || review == nil {
		return
	}
	if info.UpdatedAt == nil && review.UpdatedAt != nil {
		updatedAt := *review.UpdatedAt
		info.UpdatedAt = &updatedAt
	}
	if len(review.SKUPrices) == 0 {
		return
	}
	count := 0
	minPrice := 0.0
	maxPrice := 0.0
	currency := ""
	for _, item := range review.SKUPrices {
		if item.FinalPrice <= 0 {
			continue
		}
		count++
		if currency == "" {
			currency = strings.ToUpper(strings.TrimSpace(item.Currency))
		}
		if minPrice == 0 || item.FinalPrice < minPrice {
			minPrice = item.FinalPrice
		}
		if item.FinalPrice > maxPrice {
			maxPrice = item.FinalPrice
		}
	}
	if count == 0 {
		return
	}
	if currency == "" {
		currency = "PRICE"
	}
	if minPrice == maxPrice {
		info.DisplayValue = fmt.Sprintf("%d SKU；%s %.2f", count, currency, minPrice)
		return
	}
	info.DisplayValue = fmt.Sprintf("%d SKU；%s %.2f - %.2f", count, currency, minPrice, maxPrice)
}
