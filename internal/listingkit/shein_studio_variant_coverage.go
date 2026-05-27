package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

const (
	sheinVariantImageCoverageStatusKey  = "variant_image_coverage_status"
	sheinVariantImageCoverageMessageKey = "variant_image_coverage_message"
)

func applySheinVariantImageCoverageGuard(result *ListingKitResult, req *GenerateRequest, pkg *sheinpub.Package) bool {
	result = normalizeListingKitResultSemanticFields(result)
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if result == nil || pkg == nil {
		return false
	}
	warning, blocked := enforceSheinVariantImageCoverage(pkg, req, result.SDSDesignResult)
	setSheinVariantImageCoverageMetadata(pkg, warning, blocked)
	if !blocked || strings.TrimSpace(warning) == "" {
		return false
	}
	if result.Summary == nil {
		result.Summary = &GenerationSummary{}
	}
	result.Summary.NeedsReview = true
	result.Summary.Warnings = uniqueStrings(append(result.Summary.Warnings, warning))
	result.ReviewReasons = uniqueStrings(append(result.ReviewReasons, warning))
	pkg.ReviewNotes = uniqueStrings(append(pkg.ReviewNotes, warning))
	return true
}

func enforceSheinVariantImageCoverage(pkg *sheinpub.Package, req *GenerateRequest, sdsSummary *SDSSyncSummary) (string, bool) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || req == nil || req.Options == nil || req.Options.SheinStudio == nil {
		return "", false
	}
	skcCount := len(pkg.DraftPayload.SKCList)
	if skcCount <= 1 {
		return "", false
	}
	distinctImageCount := sheinDistinctSKCMainImageCount(pkg)
	if distinctImageCount >= skcCount {
		return "", false
	}
	coverageCount := sheinVariantImageCoverageCount(req, sdsSummary)
	if coverageCount >= skcCount {
		return "", false
	}
	warning := "变体图片覆盖不完整：当前颜色规格多于可用变体图，已阻止将同一张图复用到所有 SKC，请补齐每个颜色的商品图后再提交"
	if sdsSummary != nil && strings.TrimSpace(sdsSummary.Error) != "" {
		warning = warning + "；" + strings.TrimSpace(sdsSummary.Error)
	}
	return warning, true
}

func setSheinVariantImageCoverageMetadata(pkg *sheinpub.Package, warning string, blocked bool) {
	if pkg == nil {
		return
	}
	if pkg.Metadata == nil {
		if !blocked {
			return
		}
		pkg.Metadata = map[string]string{}
	}
	if blocked {
		pkg.Metadata[sheinVariantImageCoverageStatusKey] = "blocked"
		pkg.Metadata[sheinVariantImageCoverageMessageKey] = strings.TrimSpace(warning)
		return
	}
	delete(pkg.Metadata, sheinVariantImageCoverageStatusKey)
	delete(pkg.Metadata, sheinVariantImageCoverageMessageKey)
	if len(pkg.Metadata) == 0 {
		pkg.Metadata = nil
	}
}

func sheinVariantImageCoverageStatus(pkg *sheinpub.Package) (string, bool) {
	if pkg == nil || pkg.Metadata == nil {
		return "", false
	}
	if strings.TrimSpace(pkg.Metadata[sheinVariantImageCoverageStatusKey]) != "blocked" {
		return "", false
	}
	message := strings.TrimSpace(pkg.Metadata[sheinVariantImageCoverageMessageKey])
	if message == "" {
		message = "变体图片覆盖不完整，请为每个颜色规格补齐独立商品图后再提交"
	}
	return message, true
}

func sheinDistinctSKCMainImageCount(pkg *sheinpub.Package) int {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return 0
	}
	seen := map[string]struct{}{}
	for _, skc := range pkg.DraftPayload.SKCList {
		url := strings.TrimSpace(skcMainImageURL(skc))
		if url == "" {
			continue
		}
		seen[url] = struct{}{}
	}
	return len(seen)
}

func skcMainImageURL(skc sheinpub.SKCRequestDraft) string {
	if skc.ImageInfo != nil && strings.TrimSpace(skc.ImageInfo.MainImage) != "" {
		return strings.TrimSpace(skc.ImageInfo.MainImage)
	}
	for _, sku := range skc.SKUList {
		if strings.TrimSpace(sku.MainImage) != "" {
			return strings.TrimSpace(sku.MainImage)
		}
	}
	return ""
}

func sheinVariantImageCoverageCount(req *GenerateRequest, sdsSummary *SDSSyncSummary) int {
	counts := []int{
		len(normalizeSheinStudioVariantImageSets(req.Options.SheinStudio.VariantProductImages)),
		len(selectedSDSVariantImageCoverage(req.Options.SheinStudio.SelectedSDSImages)),
		len(completedSDSVariantCoverage(sdsSummary)),
	}
	maxCount := 0
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}

func selectedSDSVariantImageCoverage(items []SheinStudioSelectedSDSImage) map[string]struct{} {
	coverage := map[string]struct{}{}
	for _, item := range normalizeSelectedSDSImages(items) {
		if key := normalizeVariantImageKey(firstNonEmptyString(item.VariantSKU, item.Color)); key != "" {
			coverage[key] = struct{}{}
		}
	}
	return coverage
}

func completedSDSVariantCoverage(summary *SDSSyncSummary) map[string]struct{} {
	coverage := map[string]struct{}{}
	if summary == nil {
		return coverage
	}
	for _, item := range summary.VariantResults {
		if item.Status == "failed" || len(item.MockupImageURLs) == 0 {
			continue
		}
		if key := normalizeVariantImageKey(firstNonEmptyString(item.VariantSKU, item.VariantColor)); key != "" {
			coverage[key] = struct{}{}
		}
	}
	return coverage
}

func clearSharedSheinSKCImages(pkg *sheinpub.Package) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return
	}
	if pkg.DraftPayload != nil {
		for skcIndex := range pkg.DraftPayload.SKCList {
			pkg.DraftPayload.SKCList[skcIndex].ImageInfo = nil
			for skuIndex := range pkg.DraftPayload.SKCList[skcIndex].SKUList {
				pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex].MainImage = ""
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		pkg.SkcList[skcIndex].MainImageURL = ""
	}
	if pkg.PreviewPayload != nil {
		for skcIndex := range pkg.PreviewPayload.SKCList {
			pkg.PreviewPayload.SKCList[skcIndex].ImageInfo = sheinproduct.ImageInfo{}
			for skuIndex := range pkg.PreviewPayload.SKCList[skcIndex].SKUS {
				pkg.PreviewPayload.SKCList[skcIndex].SKUS[skuIndex].ImageInfo = &sheinproduct.ImageInfo{}
			}
		}
	}
}
