package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

const (
	sheinVariantImageCoverageStatusKey  = sheinpub.VariantImageCoverageStatusKey
	sheinVariantImageCoverageMessageKey = sheinpub.VariantImageCoverageMessageKey
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
	sdsError := ""
	if sdsSummary != nil && strings.TrimSpace(sdsSummary.Error) != "" {
		sdsError = sdsSummary.Error
	}
	return sheinpub.EnforceVariantImageCoverage(pkg, sheinpub.VariantImageCoverageInput{
		AvailableVariantImageGroups: sheinVariantImageCoverageCount(req, sdsSummary),
		SDSError:                    sdsError,
	})
}

func setSheinVariantImageCoverageMetadata(pkg *sheinpub.Package, warning string, blocked bool) {
	sheinpub.SetVariantImageCoverageMetadata(pkg, warning, blocked)
}

func sheinVariantImageCoverageStatus(pkg *sheinpub.Package) (string, bool) {
	return sheinpub.VariantImageCoverageStatus(pkg)
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
