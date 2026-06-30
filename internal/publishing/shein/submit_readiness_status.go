package shein

import (
	"strings"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinproduct "task-processor/internal/shein/api/product"
)

// HasAnySubmitSKU reports whether the package has at least one SKU in package or draft data.
func HasAnySubmitSKU(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	for _, skc := range pkg.SkcList {
		if len(skc.SKUs) > 0 {
			return true
		}
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if len(skc.SKUList) > 0 {
				return true
			}
		}
	}
	return false
}

// FinalSubmitImagesReady reports whether final submit images are ready for an action.
func FinalSubmitImagesReady(pkg *Package, action string) (bool, string) {
	pkg = NormalizePackageSemanticFields(pkg)
	input := sheinmarketpub.FinalSubmitImageReadinessInput{
		HasFinalDraft: pkg != nil && pkg.FinalSubmissionDraft != nil,
		RequiresSKC:   sheinmarketpub.FinalSubmitImagesRequireSKC(action),
	}
	if input.HasFinalDraft {
		main := strings.TrimSpace(pkg.FinalSubmissionDraft.MainImageURL)
		if main == "" && pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil {
			main = strings.TrimSpace(pkg.DraftPayload.ImageInfo.MainImage)
		}
		input.HasMainImage = main != ""
		input.HasGallery = HasFinalGalleryImage(pkg)
		input.HasSKCImage = HasSKCImage(pkg)
		input.HasSwatchRole = HasSwatchRole(pkg)
	}
	return sheinmarketpub.FinalSubmitImagesReady(action, input)
}

// HasFinalGalleryImage reports whether the final draft has at least one gallery-capable image.
func HasFinalGalleryImage(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || pkg.DraftPayload.ImageInfo == nil {
		return false
	}
	return sheinmarketpub.SubmitImageURLSliceHasImage(append([]string{pkg.DraftPayload.ImageInfo.MainImage}, pkg.DraftPayload.ImageInfo.Gallery...))
}

// HasSKCImage reports whether the package has an SKC or fallback single-SKC image.
func HasSKCImage(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if ImageDraftHasImage(skc.ImageInfo) {
				return true
			}
		}
	}
	if pkg.PreviewPayload != nil {
		for _, skc := range pkg.PreviewPayload.SKCList {
			if ProductImageInfoHasImage(&skc.ImageInfo) {
				return true
			}
		}
	}
	if HasSingleSKC(pkg) && HasFinalMainImage(pkg) {
		return true
	}
	return false
}

// HasSwatchRole reports whether final image roles include an explicit swatch/SKC role or an SKC image fallback exists.
func HasSwatchRole(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return true
	}
	for _, role := range pkg.FinalSubmissionDraft.ImageRoleOverrides {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "swatch", "skc":
			return true
		}
	}
	return HasSKCImage(pkg)
}

// HasSingleSKC reports whether the package has exactly one SKC across draft/package/preview data.
func HasSingleSKC(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	count := 0
	if pkg.DraftPayload != nil && len(pkg.DraftPayload.SKCList) > 0 {
		count = len(pkg.DraftPayload.SKCList)
	} else if len(pkg.SkcList) > 0 {
		count = len(pkg.SkcList)
	} else if pkg.PreviewPayload != nil && len(pkg.PreviewPayload.SKCList) > 0 {
		count = len(pkg.PreviewPayload.SKCList)
	}
	return count == 1
}

// HasFinalMainImage reports whether any final-submit main image source is present.
func HasFinalMainImage(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if pkg.FinalSubmissionDraft != nil && strings.TrimSpace(pkg.FinalSubmissionDraft.MainImageURL) != "" {
		return true
	}
	if pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil && strings.TrimSpace(pkg.DraftPayload.ImageInfo.MainImage) != "" {
		return true
	}
	if pkg.Images != nil && strings.TrimSpace(pkg.Images.MainImage) != "" {
		return true
	}
	return false
}

// SubmitPricingReady reports whether draft SKU prices are complete and positive.
func SubmitPricingReady(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	skus := make([]sheinmarketpub.SubmitPricingSKUInput, 0)
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			item := sheinmarketpub.SubmitPricingSKUInput{
				BasePrice:      sku.BasePrice,
				SiteBasePrices: make([]string, 0, len(sku.SitePriceList)),
			}
			for _, sitePrice := range sku.SitePriceList {
				item.SiteBasePrices = append(item.SiteBasePrices, sitePrice.BasePrice)
			}
			skus = append(skus, item)
		}
	}
	return sheinmarketpub.SubmitPricingReady(skus)
}

// FinalReviewReady reports whether final review confirmation allows the submit action to continue.
func FinalReviewReady(pkg *Package, action string) bool {
	if !sheinmarketpub.FinalReviewRequired(action) {
		return true
	}
	pkg = NormalizePackageSemanticFields(pkg)
	return pkg == nil || pkg.FinalSubmissionDraft == nil || pkg.FinalSubmissionDraft.Confirmed
}

// HasSubmitImage reports whether package, draft, or preview payload data contains any submit image.
func HasSubmitImage(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if pkg.Images != nil && sheinmarketpub.SubmitImageURLsHaveImage(pkg.Images.MainImage, pkg.Images.WhiteBgImage) {
		return true
	}
	if pkg.DraftPayload != nil {
		if ImageDraftHasImage(pkg.DraftPayload.ImageInfo) {
			return true
		}
		for _, skc := range pkg.DraftPayload.SKCList {
			if ImageDraftHasImage(skc.ImageInfo) {
				return true
			}
			for _, sku := range skc.SKUList {
				if strings.TrimSpace(sku.MainImage) != "" {
					return true
				}
			}
		}
	}
	if pkg.PreviewPayload != nil {
		if ProductImageInfoHasImage(pkg.PreviewPayload.ImageInfo) {
			return true
		}
		for _, skc := range pkg.PreviewPayload.SKCList {
			if ProductImageInfoHasImage(&skc.ImageInfo) {
				return true
			}
			for _, sku := range skc.SKUS {
				if ProductImageInfoHasImage(sku.ImageInfo) {
					return true
				}
			}
		}
	}
	return false
}

// ImageDraftHasImage reports whether a request image draft contains any image URL.
func ImageDraftHasImage(info *ImageDraft) bool {
	if info == nil {
		return false
	}
	return sheinmarketpub.SubmitImageDraftHasImage(sheinmarketpub.SubmitImageDraftInput{
		MainImage: info.MainImage,
		WhiteBg:   info.WhiteBg,
		Gallery:   append([]string(nil), info.Gallery...),
		Source:    append([]string(nil), info.Source...),
	})
}

// ProductImageInfoHasImage reports whether a SHEIN product image info contains any image URL.
func ProductImageInfoHasImage(info *sheinproduct.ImageInfo) bool {
	if info == nil {
		return false
	}
	urls := make([]string, 0, len(info.ImageInfoList))
	for _, image := range info.ImageInfoList {
		urls = append(urls, image.ImageURL)
	}
	return sheinmarketpub.SubmitImageURLSliceHasImage(urls)
}

func uniqueNonEmptySubmitStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
