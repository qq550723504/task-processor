package publishing

import (
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

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

// FinalDraftImageInput is the neutral final image draft shape used to build SHEIN image payloads.
type FinalDraftImageInput struct {
	MainImage string
	WhiteBg   string
	Gallery   []string
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

// UniqueNonEmptyImageURLs returns trimmed unique image URLs preserving order.
func UniqueNonEmptyImageURLs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// OrderFinalDraftImages applies explicit image order and deletion filters to image URLs.
func OrderFinalDraftImages(existing []string, order []string, deleted map[string]struct{}) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(existing)+len(order))
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := deleted[value]; ok {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, image := range order {
		add(image)
	}
	for _, image := range existing {
		add(image)
	}
	return out
}

// FirstNonEmptyImageURL returns the first value with non-empty URL content.
func FirstNonEmptyImageURL(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// ProductImageInfoFromFinalDraft builds SHEIN product image info from a final image draft.
func ProductImageInfoFromFinalDraft(input FinalDraftImageInput, roles map[string]string) *sheinproduct.ImageInfo {
	if !finalDraftImageInputHasImage(input) {
		return nil
	}
	seen := map[string]struct{}{}
	images := make([]sheinproduct.ImageDetail, 0, 1+len(input.Gallery)+1)
	add := func(url string, defaultType int, main bool) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		image := sheinproduct.ImageDetail{
			ImageURL:           url,
			ImageType:          defaultType,
			ImageSort:          len(images) + 1,
			MarketingMainImage: main,
		}
		switch strings.ToLower(strings.TrimSpace(roles[url])) {
		case "main":
			image.ImageType = 1
			image.MarketingMainImage = true
		case "swatch":
			image.ImageType = 6
			image.MarketingMainImage = false
		case "skc":
			image.ImageType = 2
			image.MarketingMainImage = false
		case "size_map":
			image.ImageType = 6
			image.SizeImgFlag = true
			image.MarketingMainImage = false
		}
		images = append(images, image)
	}
	add(input.MainImage, 1, true)
	for _, image := range input.Gallery {
		add(image, 2, false)
	}
	add(input.WhiteBg, 2, false)
	if len(images) == 0 {
		return nil
	}
	return &sheinproduct.ImageInfo{ImageInfoList: images}
}

// ProductImageDetailsCoverFinalDraft reports whether existing product images cover all final draft URLs.
func ProductImageDetailsCoverFinalDraft(existing []sheinproduct.ImageDetail, draft FinalDraftImageInput) bool {
	if len(existing) == 0 || !finalDraftImageInputHasImage(draft) {
		return false
	}
	expected := make(map[string]struct{}, 1+len(draft.Gallery)+1)
	addExpected := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		expected[url] = struct{}{}
	}
	addExpected(draft.MainImage)
	for _, image := range draft.Gallery {
		addExpected(image)
	}
	addExpected(draft.WhiteBg)
	if len(expected) == 0 {
		return false
	}
	for _, image := range existing {
		url := strings.TrimSpace(image.ImageURL)
		if url == "" {
			continue
		}
		delete(expected, url)
	}
	return len(expected) == 0
}

func finalDraftImageInputHasImage(input FinalDraftImageInput) bool {
	if SubmitImageURLsHaveImage(input.MainImage, input.WhiteBg) {
		return true
	}
	return SubmitImageURLSliceHasImage(input.Gallery)
}

// ReorderFinalDraftProductImages applies ordering, deletion, main image, and role overrides to product images.
func ReorderFinalDraftProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {
	if info == nil || len(info.ImageInfoList) == 0 {
		return
	}
	priority := make(map[string]int, len(order))
	for i, image := range order {
		priority[strings.TrimSpace(image)] = i + 1
	}
	filtered := make([]sheinproduct.ImageDetail, 0, len(info.ImageInfoList))
	for _, image := range info.ImageInfoList {
		url := strings.TrimSpace(image.ImageURL)
		if url == "" {
			continue
		}
		if _, ok := deleted[url]; ok {
			continue
		}
		if url == main {
			image.ImageSort = 1
			image.MarketingMainImage = true
			image.ImageType = 1
		} else if sort, ok := priority[url]; ok {
			image.ImageSort = sort + 1
		}
		switch roles[url] {
		case "main":
			image.ImageSort = 1
			image.MarketingMainImage = true
			image.ImageType = 1
		case "swatch":
			image.ImageType = 6
			image.MarketingMainImage = false
			image.SizeImgFlag = false
		case "skc":
			image.ImageType = 2
		case "size_map":
			image.ImageType = 6
			image.SizeImgFlag = true
		}
		filtered = append(filtered, image)
	}
	info.ImageInfoList = filtered
}

// GalleryWithoutMain removes the main image from gallery URLs.
func GalleryWithoutMain(images []string, main string) []string {
	main = strings.TrimSpace(main)
	if len(images) == 0 {
		return nil
	}
	out := make([]string, 0, len(images))
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image == "" || image == main {
			continue
		}
		out = append(out, image)
	}
	return out
}

// NormalizeImageRoleOverrides normalizes accepted final image role overrides.
func NormalizeImageRoleOverrides(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for url, role := range input {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		normalizedRole := NormalizeImageRoleOverride(role)
		if normalizedRole == "" {
			continue
		}
		out[url] = normalizedRole
	}
	return out
}

// NormalizeImageRoleOverride returns the normalized role when it is accepted.
func NormalizeImageRoleOverride(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "main", "gallery", "swatch", "size_map", "skc":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return ""
	}
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
