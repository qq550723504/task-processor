package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func applySheinStudioAIImagesToShein(pkg *sheinpub.Package, req *GenerateRequest, sdsSummary *SDSSyncSummary) {
	if pkg == nil || req == nil || req.Options == nil || req.Options.SheinStudio == nil {
		return
	}
	productImages := uniqueNonEmptyStrings(req.Options.SheinStudio.ProductImageURLs)
	variantImages := normalizeSheinStudioVariantImageSets(req.Options.SheinStudio.VariantProductImages)
	sizeReferenceImages := resolveSheinSizeReferenceImages(req, sdsSummary)
	sourceImages := uniqueNonEmptyStrings(append(
		append([]string(nil), req.Options.SheinStudio.SourceDesignURLs...),
		req.ImageURLs...,
	))
	if len(productImages) == 0 {
		if sdsSummary != nil && len(sdsSummary.MockupImageURLs) > 0 {
			applySDSTemplateImagesToShein(pkg, sdsSummary, sourceImages)
			applySheinSizeReferenceImages(pkg, sizeReferenceImages)
		}
		return
	}
	productImages = appendUniqueImageURLs(productImages, sizeReferenceImages...)
	if resolveSheinImageStrategy(req) == sheinImageStrategyHybrid {
		appendAIProductImagesToShein(pkg, productImages, sourceImages)
		applyVariantProductImagesToShein(pkg, variantImages, sourceImages)
		applySheinSizeReferenceImages(pkg, sizeReferenceImages)
		return
	}
	replaceSheinImagesWithAIProductImages(pkg, productImages, sourceImages)
	applyVariantProductImagesToShein(pkg, variantImages, sourceImages)
	applySheinSizeReferenceImages(pkg, sizeReferenceImages)
}

func replaceSheinImagesWithAIProductImages(pkg *sheinpub.Package, imageURLs []string, sourceImages []string) {
	images := imageSetFromAIProductImages(imageURLs, sourceImages)
	if images == nil {
		return
	}
	pkg.Images = images
	if pkg.RequestDraft != nil {
		pkg.RequestDraft.ImageInfo = sheinpub.BuildImageDraft(images)
		for skcIndex := range pkg.RequestDraft.SKCList {
			pkg.RequestDraft.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(images)
			for skuIndex := range pkg.RequestDraft.SKCList[skcIndex].SKUList {
				pkg.RequestDraft.SKCList[skcIndex].SKUList[skuIndex].MainImage = images.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		pkg.SkcList[skcIndex].MainImageURL = images.MainImage
	}
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
}

func appendAIProductImagesToShein(pkg *sheinpub.Package, imageURLs []string, sourceImages []string) {
	if len(imageURLs) == 0 {
		return
	}
	if pkg.Images == nil || strings.TrimSpace(pkg.Images.MainImage) == "" {
		replaceSheinImagesWithAIProductImages(pkg, imageURLs, sourceImages)
		return
	}
	pkg.Images.SourceImages = uniqueNonEmptyStrings(append(pkg.Images.SourceImages, sourceImages...))
	pkg.Images.Gallery = appendUniqueImageURLs(pkg.Images.Gallery, imageURLs...)
	if pkg.RequestDraft != nil {
		pkg.RequestDraft.ImageInfo = sheinpub.BuildImageDraft(pkg.Images)
		for skcIndex := range pkg.RequestDraft.SKCList {
			skcImages := imageDraftToSet(pkg.RequestDraft.SKCList[skcIndex].ImageInfo)
			if skcImages == nil || strings.TrimSpace(skcImages.MainImage) == "" {
				skcImages = pkg.Images
			} else {
				skcImages.SourceImages = uniqueNonEmptyStrings(append(skcImages.SourceImages, sourceImages...))
				skcImages.Gallery = appendUniqueImageURLs(skcImages.Gallery, imageURLs...)
			}
			pkg.RequestDraft.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(skcImages)
		}
	}
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
}

func imageSetFromAIProductImages(imageURLs []string, sourceImages []string) *common.ImageSet {
	imageURLs = uniqueNonEmptyStrings(imageURLs)
	if len(imageURLs) == 0 {
		return nil
	}
	images := &common.ImageSet{
		MainImage:    imageURLs[0],
		SourceImages: uniqueNonEmptyStrings(sourceImages),
	}
	if len(imageURLs) > 1 {
		images.Gallery = append([]string(nil), imageURLs[1:]...)
	}
	return images
}

func imageDraftToSet(draft *sheinpub.ImageDraft) *common.ImageSet {
	if draft == nil {
		return nil
	}
	return &common.ImageSet{
		MainImage:    draft.MainImage,
		Gallery:      append([]string(nil), draft.Gallery...),
		WhiteBgImage: draft.WhiteBg,
		SourceImages: append([]string(nil), draft.Source...),
	}
}

func appendUniqueImageURLs(existing []string, additions ...string) []string {
	result := append([]string(nil), existing...)
	seen := map[string]struct{}{}
	for _, imageURL := range result {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL != "" {
			seen[imageURL] = struct{}{}
		}
	}
	for _, imageURL := range additions {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		if _, ok := seen[imageURL]; ok {
			continue
		}
		seen[imageURL] = struct{}{}
		result = append(result, imageURL)
	}
	return result
}

func applySheinSizeReferenceImages(pkg *sheinpub.Package, imageURLs []string) {
	refs := uniqueNonEmptyStrings(imageURLs)
	if pkg == nil || len(refs) == 0 {
		return
	}
	if pkg.Images != nil {
		pkg.Images.Gallery = appendUniqueImageURLs(pkg.Images.Gallery, refs...)
	}
	if pkg.RequestDraft != nil {
		if pkg.RequestDraft.ImageInfo != nil {
			pkg.RequestDraft.ImageInfo.Gallery = appendUniqueImageURLs(pkg.RequestDraft.ImageInfo.Gallery, refs...)
		}
		for skcIndex := range pkg.RequestDraft.SKCList {
			if pkg.RequestDraft.SKCList[skcIndex].ImageInfo != nil {
				pkg.RequestDraft.SKCList[skcIndex].ImageInfo.Gallery = appendUniqueImageURLs(pkg.RequestDraft.SKCList[skcIndex].ImageInfo.Gallery, refs...)
			}
		}
	}
	if pkg.PreviewProduct == nil {
		pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
	}
	if pkg.PreviewProduct != nil {
		ensureSheinSizeReferenceDetails(pkg.PreviewProduct.ImageInfo, refs)
		for skcIndex := range pkg.PreviewProduct.SKCList {
			ensureSheinSizeReferenceDetails(&pkg.PreviewProduct.SKCList[skcIndex].ImageInfo, refs)
		}
	}
}

func ensureSheinSizeReferenceDetails(info *sheinproduct.ImageInfo, refs []string) {
	if info == nil || len(refs) == 0 {
		return
	}
	maxSort := 0
	for _, image := range info.ImageInfoList {
		if image.ImageSort > maxSort {
			maxSort = image.ImageSort
		}
	}
	for _, ref := range refs {
		found := false
		for i := range info.ImageInfoList {
			if strings.TrimSpace(info.ImageInfoList[i].ImageURL) != ref {
				continue
			}
			info.ImageInfoList[i].SizeImgFlag = true
			info.ImageInfoList[i].ImageType = 6
			found = true
		}
		if found {
			continue
		}
		maxSort++
		info.ImageInfoList = append(info.ImageInfoList, sheinproduct.ImageDetail{
			ImageType:   6,
			ImageSort:   maxSort,
			ImageURL:    ref,
			SizeImgFlag: true,
			AISStatus:   1,
			PSTypes:     []string{},
			ImageItemID: nil,
		})
	}
}

func normalizeSheinStudioVariantImageSets(input []SheinStudioVariantImageSet) []SheinStudioVariantImageSet {
	result := make([]SheinStudioVariantImageSet, 0, len(input))
	for _, item := range input {
		images := uniqueNonEmptyStrings(item.ImageURLs)
		if len(images) == 0 {
			continue
		}
		result = append(result, SheinStudioVariantImageSet{
			VariantSKU: strings.TrimSpace(item.VariantSKU),
			Color:      strings.TrimSpace(item.Color),
			ImageURLs:  images,
		})
	}
	return result
}

func applyVariantProductImagesToShein(pkg *sheinpub.Package, variantImages []SheinStudioVariantImageSet, sourceImages []string) {
	if pkg == nil || len(variantImages) == 0 {
		return
	}
	byColor := make(map[string]SheinStudioVariantImageSet, len(variantImages))
	bySKU := make(map[string]SheinStudioVariantImageSet, len(variantImages))
	for _, item := range variantImages {
		if key := normalizeVariantImageKey(item.Color); key != "" {
			byColor[key] = item
		}
		if key := normalizeVariantImageKey(item.VariantSKU); key != "" {
			bySKU[key] = item
		}
	}
	if pkg.RequestDraft != nil {
		for skcIndex := range pkg.RequestDraft.SKCList {
			skc := &pkg.RequestDraft.SKCList[skcIndex]
			if item, ok := findVariantImageSetForSKC(*skc, byColor, bySKU); ok {
				images := imageSetFromAIProductImages(item.ImageURLs, sourceImages)
				if images == nil {
					continue
				}
				skc.ImageInfo = sheinpub.BuildImageDraft(images)
				for skuIndex := range skc.SKUList {
					skc.SKUList[skuIndex].MainImage = images.MainImage
				}
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		skc := &pkg.SkcList[skcIndex]
		if item, ok := findVariantImageSetForSKCPackage(*skc, byColor, bySKU); ok && len(item.ImageURLs) > 0 {
			skc.MainImageURL = item.ImageURLs[0]
		}
	}
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
}

func findVariantImageSetForSKC(skc sheinpub.SKCRequestDraft, byColor map[string]SheinStudioVariantImageSet, bySKU map[string]SheinStudioVariantImageSet) (SheinStudioVariantImageSet, bool) {
	for _, candidate := range []string{
		skc.SaleName,
		skc.SkcName,
		saleAttributeValue(skc.SaleAttribute),
	} {
		if item, ok := byColor[normalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	for _, sku := range skc.SKUList {
		if item, ok := bySKU[normalizeVariantImageKey(sku.Attributes["source_sds_sku"])]; ok {
			return item, true
		}
		if item, ok := byColor[normalizeVariantImageKey(sku.Attributes["Color"])]; ok {
			return item, true
		}
	}
	if item, ok := bySKU[normalizeVariantImageKey(strings.Split(skc.SupplierCode, "-")[0])]; ok {
		return item, true
	}
	return SheinStudioVariantImageSet{}, false
}

func findVariantImageSetForSKCPackage(skc sheinpub.SKCPackage, byColor map[string]SheinStudioVariantImageSet, bySKU map[string]SheinStudioVariantImageSet) (SheinStudioVariantImageSet, bool) {
	for _, candidate := range []string{skc.SaleName, skc.SkcName, skc.Attributes["Color"]} {
		if item, ok := byColor[normalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	if item, ok := bySKU[normalizeVariantImageKey(strings.Split(skc.SupplierCode, "-")[0])]; ok {
		return item, true
	}
	return SheinStudioVariantImageSet{}, false
}

func saleAttributeValue(attr *sheinpub.ResolvedSaleAttribute) string {
	if attr == nil {
		return ""
	}
	return attr.Value
}

func normalizeVariantImageKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func applySheinVariantImageCoverageGuard(task *Task, pkg *sheinpub.Package) bool {
	if task == nil || task.Result == nil || pkg == nil {
		return false
	}
	warning, blocked := enforceSheinVariantImageCoverage(pkg, task.Request, task.Result.SDSSync)
	if !blocked || strings.TrimSpace(warning) == "" {
		return false
	}
	if task.Result.Summary == nil {
		task.Result.Summary = &GenerationSummary{}
	}
	task.Result.Summary.NeedsReview = true
	task.Result.Summary.Warnings = uniqueStrings(append(task.Result.Summary.Warnings, warning))
	task.Result.ReviewReasons = uniqueStrings(append(task.Result.ReviewReasons, warning))
	pkg.ReviewNotes = uniqueStrings(append(pkg.ReviewNotes, warning))
	return true
}

func enforceSheinVariantImageCoverage(pkg *sheinpub.Package, req *GenerateRequest, sdsSummary *SDSSyncSummary) (string, bool) {
	if pkg == nil || req == nil || req.Options == nil || req.Options.SheinStudio == nil {
		return "", false
	}
	skcCount := len(pkg.RequestDraft.SKCList)
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
	clearSharedSheinSKCImages(pkg)
	warning := "变体图片覆盖不完整：当前颜色规格多于可用变体图，已阻止将同一张图复用到所有 SKC，请补齐每个颜色的商品图后再提交"
	if sdsSummary != nil && strings.TrimSpace(sdsSummary.Error) != "" {
		warning = warning + "；" + strings.TrimSpace(sdsSummary.Error)
	}
	return warning, true
}

func sheinDistinctSKCMainImageCount(pkg *sheinpub.Package) int {
	if pkg == nil || pkg.RequestDraft == nil {
		return 0
	}
	seen := map[string]struct{}{}
	for _, skc := range pkg.RequestDraft.SKCList {
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
	if pkg == nil {
		return
	}
	if pkg.RequestDraft != nil {
		for skcIndex := range pkg.RequestDraft.SKCList {
			pkg.RequestDraft.SKCList[skcIndex].ImageInfo = nil
			for skuIndex := range pkg.RequestDraft.SKCList[skcIndex].SKUList {
				pkg.RequestDraft.SKCList[skcIndex].SKUList[skuIndex].MainImage = ""
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		pkg.SkcList[skcIndex].MainImageURL = ""
	}
	if pkg.PreviewProduct != nil {
		for skcIndex := range pkg.PreviewProduct.SKCList {
			pkg.PreviewProduct.SKCList[skcIndex].ImageInfo = sheinproduct.ImageInfo{}
			for skuIndex := range pkg.PreviewProduct.SKCList[skcIndex].SKUS {
				pkg.PreviewProduct.SKCList[skcIndex].SKUS[skuIndex].ImageInfo = &sheinproduct.ImageInfo{}
			}
		}
	}
}
