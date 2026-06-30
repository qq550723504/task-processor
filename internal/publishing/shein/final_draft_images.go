package shein

import (
	"strings"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinproduct "task-processor/internal/shein/api/product"
)

// ApplyFinalImageDraft applies final image ordering, deletion, and role overrides to draft and preview payloads.
func ApplyFinalImageDraft(pkg *Package) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return
	}
	order := pkg.FinalSubmissionDraft.FinalImageOrder
	main := strings.TrimSpace(pkg.FinalSubmissionDraft.MainImageURL)
	deleted := make(map[string]struct{}, len(pkg.FinalSubmissionDraft.DeletedImageURLs))
	for _, image := range pkg.FinalSubmissionDraft.DeletedImageURLs {
		deleted[strings.TrimSpace(image)] = struct{}{}
	}
	if pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil {
		images := OrderFinalDraftImages(pkg.DraftPayload.ImageInfo.Gallery, order, deleted)
		if main == "" && len(images) > 0 {
			main = images[0]
		}
		if main != "" {
			pkg.DraftPayload.ImageInfo.MainImage = main
		}
		pkg.DraftPayload.ImageInfo.Gallery = images
	}
	EnsureFinalDraftSKCImages(pkg, main, order, deleted)
	if pkg.DraftPayload != nil {
		for i := range pkg.DraftPayload.SKCList {
			if pkg.DraftPayload.SKCList[i].ImageInfo == nil {
				continue
			}
			pkg.DraftPayload.SKCList[i].ImageInfo.Gallery = OrderFinalDraftImages(pkg.DraftPayload.SKCList[i].ImageInfo.Gallery, order, deleted)
			if _, removed := deleted[pkg.DraftPayload.SKCList[i].ImageInfo.MainImage]; removed {
				pkg.DraftPayload.SKCList[i].ImageInfo.MainImage = firstNonEmptyFinalDraftString(pkg.DraftPayload.SKCList[i].ImageInfo.Gallery...)
			}
		}
	}
	if pkg.PreviewPayload != nil && pkg.PreviewPayload.ImageInfo != nil {
		ReorderFinalDraftProductImages(pkg.PreviewPayload.ImageInfo, order, main, deleted, pkg.FinalSubmissionDraft.ImageRoleOverrides)
	}
	EnsureFinalPreviewSKCImages(pkg)
	if pkg.PreviewPayload != nil {
		for i := range pkg.PreviewPayload.SKCList {
			ReorderFinalDraftProductImages(&pkg.PreviewPayload.SKCList[i].ImageInfo, order, main, deleted, pkg.FinalSubmissionDraft.ImageRoleOverrides)
		}
	}
}

// EnsureFinalDraftSKCImages fills draft SKC/SKU images from final image selections and package fallbacks.
func EnsureFinalDraftSKCImages(pkg *Package, main string, order []string, deleted map[string]struct{}) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(pkg.DraftPayload.SKCList) == 0 {
		return
	}
	fallback := FinalDraftFallbackImages(pkg, main, deleted)
	for index := range pkg.DraftPayload.SKCList {
		skcDraft := &pkg.DraftPayload.SKCList[index]
		mainImage := firstNonEmptyFinalDraftString(
			PackageSKCMainImage(pkg, index, skcDraft.SupplierCode),
			RequestSKCMainImage(skcDraft),
			main,
			firstNonEmptyFinalDraftString(fallback...),
		)
		if strings.TrimSpace(mainImage) == "" {
			continue
		}
		if skcDraft.ImageInfo == nil {
			skcDraft.ImageInfo = &ImageDraft{}
		}
		if strings.TrimSpace(skcDraft.ImageInfo.MainImage) == "" {
			skcDraft.ImageInfo.MainImage = mainImage
		}
		galleryFallback := fallback
		if topMain := strings.TrimSpace(main); topMain != "" {
			filtered := make([]string, 0, len(fallback))
			for _, image := range fallback {
				if strings.TrimSpace(image) == topMain {
					continue
				}
				filtered = append(filtered, image)
			}
			galleryFallback = filtered
		}
		mergedGallery := GalleryWithoutMain(
			OrderFinalDraftImages(skcDraft.ImageInfo.Gallery, galleryFallback, deleted),
			firstNonEmptyFinalDraftString(skcDraft.ImageInfo.MainImage, mainImage),
		)
		if len(skcDraft.ImageInfo.Gallery) == 0 || len(mergedGallery) > len(skcDraft.ImageInfo.Gallery) {
			skcDraft.ImageInfo.Gallery = mergedGallery
		}
		if pkg.DraftPayload.ImageInfo != nil && strings.TrimSpace(skcDraft.ImageInfo.WhiteBg) == "" {
			skcDraft.ImageInfo.WhiteBg = strings.TrimSpace(pkg.DraftPayload.ImageInfo.WhiteBg)
		}
		for skuIndex := range skcDraft.SKUList {
			if strings.TrimSpace(skcDraft.SKUList[skuIndex].MainImage) == "" {
				skcDraft.SKUList[skuIndex].MainImage = firstNonEmptyFinalDraftString(skcDraft.ImageInfo.MainImage, mainImage)
			}
		}
	}
}

// EnsureFinalPreviewSKCImages copies draft SKC image selections into preview payload SKCs when preview images are incomplete.
func EnsureFinalPreviewSKCImages(pkg *Package) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.PreviewPayload == nil || len(pkg.PreviewPayload.SKCList) == 0 {
		return
	}
	roleOverrides := map[string]string(nil)
	if pkg.FinalSubmissionDraft != nil {
		roleOverrides = pkg.FinalSubmissionDraft.ImageRoleOverrides
	}
	for index := range pkg.PreviewPayload.SKCList {
		skc := &pkg.PreviewPayload.SKCList[index]
		draft := RequestDraftSKCByIndexOrCode(pkg.DraftPayload, index, PreviewSKCSupplierCode(skc))
		if draft == nil || !ImageDraftHasImages(draft.ImageInfo) {
			continue
		}
		info := ProductImageInfoFromDraft(draft.ImageInfo, roleOverrides)
		if info == nil {
			continue
		}
		if len(skc.ImageInfo.ImageInfoList) > 0 && PreviewSKCImagesCoverDraft(skc.ImageInfo.ImageInfoList, draft.ImageInfo) {
			continue
		}
		skc.ImageInfo = *info
	}
}

// PreviewSKCImagesCoverDraft reports whether preview SKC images already cover all draft image URLs.
func PreviewSKCImagesCoverDraft(existing []sheinproduct.ImageDetail, draft *ImageDraft) bool {
	if len(existing) == 0 || !ImageDraftHasImages(draft) {
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

// FinalDraftFallbackImages collects image fallbacks for final draft SKC images.
func FinalDraftFallbackImages(pkg *Package, main string, deleted map[string]struct{}) []string {
	images := make([]string, 0, 16)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, removed := deleted[value]; removed {
			return
		}
		images = append(images, value)
	}
	add(main)
	if pkg == nil {
		return uniqueNonEmptyFinalDraftStrings(images)
	}
	if pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil {
		add(pkg.DraftPayload.ImageInfo.MainImage)
		for _, image := range pkg.DraftPayload.ImageInfo.Gallery {
			add(image)
		}
		add(pkg.DraftPayload.ImageInfo.WhiteBg)
	}
	if pkg.PreviewPayload != nil && pkg.PreviewPayload.ImageInfo != nil {
		for _, image := range pkg.PreviewPayload.ImageInfo.ImageInfoList {
			add(image.ImageURL)
		}
	}
	for _, skc := range pkg.SkcList {
		add(skc.MainImageURL)
	}
	return uniqueNonEmptyFinalDraftStrings(images)
}

// PackageSKCMainImage returns a package SKC image by supplier code or index.
func PackageSKCMainImage(pkg *Package, index int, supplierCode string) string {
	if pkg == nil {
		return ""
	}
	if strings.TrimSpace(supplierCode) != "" {
		for _, skc := range pkg.SkcList {
			if strings.EqualFold(strings.TrimSpace(skc.SupplierCode), strings.TrimSpace(supplierCode)) {
				return strings.TrimSpace(skc.MainImageURL)
			}
		}
	}
	if index >= 0 && index < len(pkg.SkcList) {
		return strings.TrimSpace(pkg.SkcList[index].MainImageURL)
	}
	return ""
}

// RequestSKCMainImage returns the main image from a request SKC draft.
func RequestSKCMainImage(skc *SKCRequestDraft) string {
	if skc == nil {
		return ""
	}
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

// ImageDraftHasImages reports whether an image draft contains any image URL.
func ImageDraftHasImages(info *ImageDraft) bool {
	if info == nil {
		return false
	}
	return sheinmarketpub.SubmitImageDraftHasImage(sheinmarketpub.SubmitImageDraftInput{
		MainImage: info.MainImage,
		WhiteBg:   info.WhiteBg,
		Gallery:   append([]string(nil), info.Gallery...),
	})
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

// RequestDraftSKCByIndexOrCode resolves a request SKC draft by supplier code or index fallback.
func RequestDraftSKCByIndexOrCode(draft *RequestDraft, index int, supplierCode string) *SKCRequestDraft {
	if draft == nil {
		return nil
	}
	if strings.TrimSpace(supplierCode) != "" {
		for i := range draft.SKCList {
			if strings.EqualFold(strings.TrimSpace(draft.SKCList[i].SupplierCode), strings.TrimSpace(supplierCode)) {
				return &draft.SKCList[i]
			}
		}
	}
	if index >= 0 && index < len(draft.SKCList) {
		return &draft.SKCList[index]
	}
	return nil
}

// PreviewSKCSupplierCode returns a preview SKC supplier code.
func PreviewSKCSupplierCode(skc *sheinproduct.SKC) string {
	if skc == nil || skc.SupplierCode == nil {
		return ""
	}
	return strings.TrimSpace(*skc.SupplierCode)
}

// ProductImageInfoFromDraft builds SHEIN product image info from a final image draft.
func ProductImageInfoFromDraft(info *ImageDraft, roles map[string]string) *sheinproduct.ImageInfo {
	if !ImageDraftHasImages(info) {
		return nil
	}
	seen := map[string]struct{}{}
	images := make([]sheinproduct.ImageDetail, 0, 1+len(info.Gallery)+1)
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
	add(info.MainImage, 1, true)
	for _, image := range info.Gallery {
		add(image, 2, false)
	}
	add(info.WhiteBg, 2, false)
	if len(images) == 0 {
		return nil
	}
	return &sheinproduct.ImageInfo{ImageInfoList: images}
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

// NormalizeImageRoleOverrides normalizes accepted final image role overrides.
func NormalizeImageRoleOverrides(input map[string]string) map[string]string {
	return sheinmarketpub.NormalizeImageRoleOverrides(input)
}

func firstNonEmptyFinalDraftString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueNonEmptyFinalDraftStrings(values []string) []string {
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
