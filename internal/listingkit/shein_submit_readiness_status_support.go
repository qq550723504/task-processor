package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func sheinHasAnySKU(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
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

func sheinFinalImagesReady(pkg *SheinPackage) (bool, string) {
	return sheinFinalImagesReadyForAction(pkg, "publish")
}

func sheinFinalImagesReadyForAction(pkg *SheinPackage, action string) (bool, string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return true, "旧任务未启用最终图片确认，按兼容路径处理"
	}
	action = strings.ToLower(strings.TrimSpace(action))
	main := strings.TrimSpace(pkg.FinalSubmissionDraft.MainImageURL)
	if main == "" && pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil {
		main = strings.TrimSpace(pkg.DraftPayload.ImageInfo.MainImage)
	}
	if main == "" {
		return false, "最终确认页还没有设置主图"
	}
	if !sheinHasFinalGalleryImage(pkg) {
		return false, "最终图库为空，提交前至少需要一张图库图片"
	}
	if action == "save_draft" {
		return true, "草稿保存图片已具备主图和图库；色块图、SKC 图和尺寸图会在正式发布前严格校验"
	}
	if !sheinHasSKCImage(pkg) {
		return false, "缺少 SKC/色块图，提交前需要为每个颜色规格准备可提交图片"
	}
	if !sheinHasSwatchRole(pkg) {
		return false, "缺少色块图标记，请在 SHEIN data images 中标记一张色块图"
	}
	return true, "最终图片已具备主图、图库和可用的色块/SKC 图；尺寸图未选择时不阻断提交"
}

func sheinHasFinalGalleryImage(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || pkg.DraftPayload.ImageInfo == nil {
		return false
	}
	return len(uniqueNonEmptyStrings(append([]string{pkg.DraftPayload.ImageInfo.MainImage}, pkg.DraftPayload.ImageInfo.Gallery...))) > 0
}

func sheinHasSKCImage(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if sheinImageDraftHasImage(skc.ImageInfo) {
				return true
			}
		}
	}
	if pkg.PreviewPayload != nil {
		for _, skc := range pkg.PreviewPayload.SKCList {
			if sheinProductImageInfoHasImage(&skc.ImageInfo) {
				return true
			}
		}
	}
	if sheinHasSingleSKC(pkg) && sheinHasFinalMainImage(pkg) {
		return true
	}
	return false
}

func sheinHasSwatchRole(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return true
	}
	for _, role := range pkg.FinalSubmissionDraft.ImageRoleOverrides {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "swatch", "skc":
			return true
		}
	}
	return sheinHasSKCImage(pkg)
}

func sheinHasSingleSKC(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
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

func sheinHasFinalMainImage(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
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

func sheinPricingReady(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	hasSKU := false
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			hasSKU = true
			if parseMoney(sku.BasePrice) <= 0 {
				return false
			}
			if len(sku.SitePriceList) == 0 {
				return false
			}
			for _, sitePrice := range sku.SitePriceList {
				if parseMoney(sitePrice.BasePrice) <= 0 {
					return false
				}
			}
		}
	}
	return hasSKU
}

func sheinHasSubmitImage(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if pkg.Images != nil && firstNonEmpty(pkg.Images.MainImage, pkg.Images.WhiteBgImage) != "" {
		return true
	}
	if pkg.DraftPayload != nil {
		if sheinImageDraftHasImage(pkg.DraftPayload.ImageInfo) {
			return true
		}
		for _, skc := range pkg.DraftPayload.SKCList {
			if sheinImageDraftHasImage(skc.ImageInfo) {
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
		if sheinProductImageInfoHasImage(pkg.PreviewPayload.ImageInfo) {
			return true
		}
		for _, skc := range pkg.PreviewPayload.SKCList {
			if sheinProductImageInfoHasImage(&skc.ImageInfo) {
				return true
			}
			for _, sku := range skc.SKUS {
				if sheinProductImageInfoHasImage(sku.ImageInfo) {
					return true
				}
			}
		}
	}
	return false
}

func sheinImageDraftHasImage(info *SheinImageDraft) bool {
	if info == nil {
		return false
	}
	if firstNonEmpty(info.MainImage, info.WhiteBg) != "" {
		return true
	}
	for _, image := range append(append([]string(nil), info.Gallery...), info.Source...) {
		if strings.TrimSpace(image) != "" {
			return true
		}
	}
	return false
}

func sheinProductImageInfoHasImage(info *sheinproduct.ImageInfo) bool {
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
