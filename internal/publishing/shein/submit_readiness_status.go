package shein

import (
	"strconv"
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
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return true, "旧任务未启用最终图片确认，按兼容路径处理"
	}
	main := strings.TrimSpace(pkg.FinalSubmissionDraft.MainImageURL)
	if main == "" && pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil {
		main = strings.TrimSpace(pkg.DraftPayload.ImageInfo.MainImage)
	}
	if main == "" {
		return false, "最终确认页还没有设置主图"
	}
	if !HasFinalGalleryImage(pkg) {
		return false, "最终图库为空，提交前至少需要一张图库图片"
	}
	if !sheinmarketpub.FinalSubmitImagesRequireSKC(action) {
		return true, "草稿保存图片已具备主图和图库；色块图、SKC 图和尺寸图会在正式发布前严格校验"
	}
	if !HasSKCImage(pkg) {
		return false, "缺少 SKC/色块图，提交前需要为每个颜色规格准备可提交图片"
	}
	if !HasSwatchRole(pkg) {
		return false, "缺少色块图标记，请在 SHEIN data images 中标记一张色块图"
	}
	return true, "最终图片已具备主图、图库和可用的色块/SKC 图；尺寸图未选择时不阻断提交"
}

// HasFinalGalleryImage reports whether the final draft has at least one gallery-capable image.
func HasFinalGalleryImage(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || pkg.DraftPayload.ImageInfo == nil {
		return false
	}
	return len(uniqueNonEmptySubmitStrings(append([]string{pkg.DraftPayload.ImageInfo.MainImage}, pkg.DraftPayload.ImageInfo.Gallery...))) > 0
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
	hasSKU := false
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			hasSKU = true
			if parseSubmitMoney(sku.BasePrice) <= 0 {
				return false
			}
			if len(sku.SitePriceList) == 0 {
				return false
			}
			for _, sitePrice := range sku.SitePriceList {
				if parseSubmitMoney(sitePrice.BasePrice) <= 0 {
					return false
				}
			}
		}
	}
	return hasSKU
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
	if pkg.Images != nil && firstNonEmptySubmitString(pkg.Images.MainImage, pkg.Images.WhiteBgImage) != "" {
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
	if firstNonEmptySubmitString(info.MainImage, info.WhiteBg) != "" {
		return true
	}
	for _, image := range append(append([]string(nil), info.Gallery...), info.Source...) {
		if strings.TrimSpace(image) != "" {
			return true
		}
	}
	return false
}

// ProductImageInfoHasImage reports whether a SHEIN product image info contains any image URL.
func ProductImageInfoHasImage(info *sheinproduct.ImageInfo) bool {
	if info == nil {
		return false
	}
	for _, image := range info.ImageInfoList {
		if strings.TrimSpace(image.ImageURL) != "" {
			return true
		}
	}
	return false
}

func firstNonEmptySubmitString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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

func parseSubmitMoney(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
