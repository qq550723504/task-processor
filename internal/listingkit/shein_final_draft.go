package listingkit

import (
	"context"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg := task.Result.Shein
	if pkg.FinalDraft == nil {
		pkg.FinalDraft = &sheinpub.FinalDraft{}
	}
	if req != nil {
		if req.SubmitMode != "" {
			mode := strings.ToLower(strings.TrimSpace(req.SubmitMode))
			if mode == "publish" || mode == "save_draft" {
				pkg.FinalDraft.SubmitMode = mode
			}
		}
		if len(req.ManualPriceOverrides) > 0 {
			pkg.FinalDraft.ManualPriceOverrides = clonePriceOverrides(req.ManualPriceOverrides)
		}
		if req.FinalImageOrder != nil {
			pkg.FinalDraft.FinalImageOrder = uniqueNonEmptyStrings(*req.FinalImageOrder)
		}
		if value := strings.TrimSpace(req.MainImageURL); value != "" {
			pkg.FinalDraft.MainImageURL = value
		}
		if req.DeletedImageURLs != nil {
			pkg.FinalDraft.DeletedImageURLs = uniqueNonEmptyStrings(*req.DeletedImageURLs)
		}
		if len(req.ImageRoleOverrides) > 0 {
			pkg.FinalDraft.ImageRoleOverrides = normalizeImageRoleOverrides(req.ImageRoleOverrides)
		}
		if req.Confirmed != nil {
			pkg.FinalDraft.Confirmed = *req.Confirmed
			if *req.Confirmed {
				now := time.Now()
				pkg.FinalDraft.ConfirmedAt = &now
			} else {
				pkg.FinalDraft.ConfirmedAt = nil
			}
		}
	}
	now := time.Now()
	pkg.FinalDraft.UpdatedAt = &now
	rule := s.currentSheinPricingRule()
	if pkg.Pricing != nil && pkg.Pricing.RuleSnapshot != nil {
		rule = *pkg.Pricing.RuleSnapshot
	}
	review := buildSheinDraftBackedPricingReview(pkg, rule, pkg.FinalDraft.ManualPriceOverrides)
	applySheinPricingReview(pkg, review)
	applySheinFinalImageDraft(pkg)
	applySheinVariantImageCoverageGuard(task, pkg)
	task.Result.UpdatedAt = now
	if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
		return nil, err
	}
	return buildListingKitPreview(task, "shein")
}

func applySheinFinalImageDraft(pkg *sheinpub.Package) {
	if pkg == nil || pkg.FinalDraft == nil {
		return
	}
	order := pkg.FinalDraft.FinalImageOrder
	main := strings.TrimSpace(pkg.FinalDraft.MainImageURL)
	deleted := make(map[string]struct{}, len(pkg.FinalDraft.DeletedImageURLs))
	for _, image := range pkg.FinalDraft.DeletedImageURLs {
		deleted[strings.TrimSpace(image)] = struct{}{}
	}
	if pkg.RequestDraft != nil && pkg.RequestDraft.ImageInfo != nil {
		images := orderSheinImages(pkg.RequestDraft.ImageInfo.Gallery, order, deleted)
		if main == "" && len(images) > 0 {
			main = images[0]
		}
		if main != "" {
			pkg.RequestDraft.ImageInfo.MainImage = main
		}
		pkg.RequestDraft.ImageInfo.Gallery = images
	}
	ensureSheinFinalDraftSKCImages(pkg, main, order, deleted)
	if pkg.RequestDraft != nil {
		for i := range pkg.RequestDraft.SKCList {
			if pkg.RequestDraft.SKCList[i].ImageInfo == nil {
				continue
			}
			pkg.RequestDraft.SKCList[i].ImageInfo.Gallery = orderSheinImages(pkg.RequestDraft.SKCList[i].ImageInfo.Gallery, order, deleted)
			if _, removed := deleted[pkg.RequestDraft.SKCList[i].ImageInfo.MainImage]; removed {
				pkg.RequestDraft.SKCList[i].ImageInfo.MainImage = firstNonEmpty(pkg.RequestDraft.SKCList[i].ImageInfo.Gallery...)
			}
		}
	}
	if pkg.PreviewProduct != nil && pkg.PreviewProduct.ImageInfo != nil {
		reorderSheinProductImages(pkg.PreviewProduct.ImageInfo, order, main, deleted, pkg.FinalDraft.ImageRoleOverrides)
	}
	ensureSheinFinalPreviewSKCImages(pkg)
	if pkg.PreviewProduct != nil {
		for i := range pkg.PreviewProduct.SKCList {
			reorderSheinProductImages(&pkg.PreviewProduct.SKCList[i].ImageInfo, order, main, deleted, pkg.FinalDraft.ImageRoleOverrides)
		}
	}
}

func ensureSheinFinalDraftSKCImages(pkg *sheinpub.Package, main string, order []string, deleted map[string]struct{}) {
	if pkg == nil || pkg.RequestDraft == nil || len(pkg.RequestDraft.SKCList) == 0 {
		return
	}
	fallback := sheinFinalDraftFallbackImages(pkg, main, deleted)
	for index := range pkg.RequestDraft.SKCList {
		skcDraft := &pkg.RequestDraft.SKCList[index]
		if sheinImageDraftHasImages(skcDraft.ImageInfo) {
			continue
		}
		mainImage := firstNonEmpty(
			sheinPackageSKCMainImage(pkg, index, skcDraft.SupplierCode),
			sheinRequestSKCMainImage(skcDraft),
			main,
			firstNonEmpty(fallback...),
		)
		if strings.TrimSpace(mainImage) == "" {
			continue
		}
		if skcDraft.ImageInfo == nil {
			skcDraft.ImageInfo = &sheinpub.ImageDraft{}
		}
		skcDraft.ImageInfo.MainImage = mainImage
		skcDraft.ImageInfo.Gallery = sheinGalleryWithoutMain(orderSheinImages(nil, fallback, deleted), mainImage)
		if pkg.RequestDraft.ImageInfo != nil && strings.TrimSpace(skcDraft.ImageInfo.WhiteBg) == "" {
			skcDraft.ImageInfo.WhiteBg = strings.TrimSpace(pkg.RequestDraft.ImageInfo.WhiteBg)
		}
		for skuIndex := range skcDraft.SKUList {
			if strings.TrimSpace(skcDraft.SKUList[skuIndex].MainImage) == "" {
				skcDraft.SKUList[skuIndex].MainImage = mainImage
			}
		}
	}
}

func ensureSheinFinalPreviewSKCImages(pkg *sheinpub.Package) {
	if pkg == nil || pkg.PreviewProduct == nil || len(pkg.PreviewProduct.SKCList) == 0 {
		return
	}
	roleOverrides := map[string]string(nil)
	if pkg.FinalDraft != nil {
		roleOverrides = pkg.FinalDraft.ImageRoleOverrides
	}
	for index := range pkg.PreviewProduct.SKCList {
		skc := &pkg.PreviewProduct.SKCList[index]
		draft := sheinRequestDraftSKCByIndexOrCode(pkg.RequestDraft, index, sheinPreviewSKCSupplierCode(skc))
		if draft == nil || !sheinImageDraftHasImages(draft.ImageInfo) {
			continue
		}
		info := sheinProductImageInfoFromDraft(draft.ImageInfo, roleOverrides)
		if info == nil {
			continue
		}
		if len(skc.ImageInfo.ImageInfoList) > 0 && sheinPreviewSKCImagesCoverDraft(skc.ImageInfo.ImageInfoList, draft.ImageInfo) {
			continue
		}
		skc.ImageInfo = *info
	}
}

func sheinPreviewSKCImagesCoverDraft(existing []sheinproduct.ImageDetail, draft *sheinpub.ImageDraft) bool {
	if len(existing) == 0 || !sheinImageDraftHasImages(draft) {
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

func orderSheinImages(existing []string, order []string, deleted map[string]struct{}) []string {
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

func sheinFinalDraftFallbackImages(pkg *sheinpub.Package, main string, deleted map[string]struct{}) []string {
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
		return uniqueNonEmptyStrings(images)
	}
	if pkg.RequestDraft != nil && pkg.RequestDraft.ImageInfo != nil {
		add(pkg.RequestDraft.ImageInfo.MainImage)
		for _, image := range pkg.RequestDraft.ImageInfo.Gallery {
			add(image)
		}
		add(pkg.RequestDraft.ImageInfo.WhiteBg)
	}
	if pkg.PreviewProduct != nil && pkg.PreviewProduct.ImageInfo != nil {
		for _, image := range pkg.PreviewProduct.ImageInfo.ImageInfoList {
			add(image.ImageURL)
		}
	}
	for _, skc := range pkg.SkcList {
		add(skc.MainImageURL)
	}
	return uniqueNonEmptyStrings(images)
}

func sheinPackageSKCMainImage(pkg *sheinpub.Package, index int, supplierCode string) string {
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

func sheinRequestSKCMainImage(skc *sheinpub.SKCRequestDraft) string {
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

func sheinImageDraftHasImages(info *sheinpub.ImageDraft) bool {
	if info == nil {
		return false
	}
	if strings.TrimSpace(info.MainImage) != "" || strings.TrimSpace(info.WhiteBg) != "" {
		return true
	}
	for _, image := range info.Gallery {
		if strings.TrimSpace(image) != "" {
			return true
		}
	}
	return false
}

func sheinGalleryWithoutMain(images []string, main string) []string {
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

func sheinRequestDraftSKCByIndexOrCode(draft *sheinpub.RequestDraft, index int, supplierCode string) *sheinpub.SKCRequestDraft {
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

func sheinPreviewSKCSupplierCode(skc *sheinproduct.SKC) string {
	if skc == nil || skc.SupplierCode == nil {
		return ""
	}
	return strings.TrimSpace(*skc.SupplierCode)
}

func sheinProductImageInfoFromDraft(info *sheinpub.ImageDraft, roles map[string]string) *sheinproduct.ImageInfo {
	if !sheinImageDraftHasImages(info) {
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

func reorderSheinProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {
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

func normalizeImageRoleOverrides(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for url, role := range input {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "main", "gallery", "swatch", "size_map", "skc":
			out[url] = strings.ToLower(strings.TrimSpace(role))
		}
	}
	return out
}
