package publishing

import "strings"

// FinalSubmitImagesRequireSKC reports whether final submit image readiness must
// include SKC/swatch evidence for the action.
func FinalSubmitImagesRequireSKC(action string) bool {
	return FinalReviewRequired(action)
}

// FinalSubmitImageReadinessInput is the distilled image state for final submit readiness.
type FinalSubmitImageReadinessInput struct {
	HasFinalDraft bool
	HasMainImage  bool
	HasGallery    bool
	HasSKCImage   bool
	HasSwatchRole bool
	RequiresSKC   bool
}

// SubmitImageDraftInput is the neutral image draft shape used by submit image policies.
type SubmitImageDraftInput struct {
	MainImage string
	WhiteBg   string
	Gallery   []string
	Source    []string
}

// FinalSubmitImagesReady reports whether final submit images satisfy action-specific readiness.
func FinalSubmitImagesReady(action string, input FinalSubmitImageReadinessInput) (bool, string) {
	if !input.HasFinalDraft {
		return true, "旧任务未启用最终图片确认，按兼容路径处理"
	}
	if !input.HasMainImage {
		return false, "最终确认页还没有设置主图"
	}
	if !input.HasGallery {
		return false, "最终图库为空，提交前至少需要一张图库图片"
	}
	if !input.RequiresSKC && !FinalSubmitImagesRequireSKC(action) {
		return true, "草稿保存图片已具备主图和图库；色块图、SKC 图和尺寸图会在正式发布前严格校验"
	}
	if !input.HasSKCImage {
		return false, "缺少 SKC/色块图，提交前需要为每个颜色规格准备可提交图片"
	}
	if !input.HasSwatchRole {
		return false, "缺少色块图标记，请在 SHEIN data images 中标记一张色块图"
	}
	return true, "最终图片已具备主图、图库和可用的色块/SKC 图；尺寸图未选择时不阻断提交"
}

// SubmitImageDraftHasImage reports whether an image draft contains any image URL.
func SubmitImageDraftHasImage(input SubmitImageDraftInput) bool {
	if SubmitImageURLsHaveImage(input.MainImage, input.WhiteBg) {
		return true
	}
	return SubmitImageURLSliceHasImage(input.Gallery) || SubmitImageURLSliceHasImage(input.Source)
}

// SubmitImageURLSliceHasImage reports whether any URL in a slice is non-empty.
func SubmitImageURLSliceHasImage(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// SubmitImageURLsHaveImage reports whether any URL is non-empty.
func SubmitImageURLsHaveImage(values ...string) bool {
	return SubmitImageURLSliceHasImage(values)
}

// IsUploadedImageURL reports whether url already points at a SHEIN-hosted image.
func IsUploadedImageURL(url string) bool {
	value := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(value, "shein.com") ||
		strings.Contains(value, "sheinimg.com") ||
		strings.Contains(value, "ltwebstatic.com")
}

// IsSDSImageURL reports whether url points at an SDS-generated/source image host.
func IsSDSImageURL(url string) bool {
	value := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(value, "sdspod.com") || strings.Contains(value, "sdsdiy.com")
}

// CloneImageUploadCache normalizes and filters source-to-uploaded image URL cache entries.
func CloneImageUploadCache(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for sourceURL, uploadedURL := range input {
		sourceURL = strings.TrimSpace(sourceURL)
		uploadedURL = strings.TrimSpace(uploadedURL)
		if sourceURL == "" || uploadedURL == "" || !IsUploadedImageURL(uploadedURL) {
			continue
		}
		out[sourceURL] = uploadedURL
	}
	return out
}
