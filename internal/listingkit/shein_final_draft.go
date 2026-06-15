package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func applySheinFinalImageDraft(pkg *sheinpub.Package) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
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
		images := orderSheinImages(pkg.DraftPayload.ImageInfo.Gallery, order, deleted)
		if main == "" && len(images) > 0 {
			main = images[0]
		}
		if main != "" {
			pkg.DraftPayload.ImageInfo.MainImage = main
		}
		pkg.DraftPayload.ImageInfo.Gallery = images
	}
	ensureSheinFinalDraftSKCImages(pkg, main, order, deleted)
	if pkg.DraftPayload != nil {
		for i := range pkg.DraftPayload.SKCList {
			if pkg.DraftPayload.SKCList[i].ImageInfo == nil {
				continue
			}
			pkg.DraftPayload.SKCList[i].ImageInfo.Gallery = orderSheinImages(pkg.DraftPayload.SKCList[i].ImageInfo.Gallery, order, deleted)
			if _, removed := deleted[pkg.DraftPayload.SKCList[i].ImageInfo.MainImage]; removed {
				pkg.DraftPayload.SKCList[i].ImageInfo.MainImage = firstNonEmpty(pkg.DraftPayload.SKCList[i].ImageInfo.Gallery...)
			}
		}
	}
	if pkg.PreviewPayload != nil && pkg.PreviewPayload.ImageInfo != nil {
		reorderSheinProductImages(pkg.PreviewPayload.ImageInfo, order, main, deleted, pkg.FinalSubmissionDraft.ImageRoleOverrides)
	}
	ensureSheinFinalPreviewSKCImages(pkg)
	if pkg.PreviewPayload != nil {
		for i := range pkg.PreviewPayload.SKCList {
			reorderSheinProductImages(&pkg.PreviewPayload.SKCList[i].ImageInfo, order, main, deleted, pkg.FinalSubmissionDraft.ImageRoleOverrides)
		}
	}
}

func ensureSheinFinalDraftSKCImages(pkg *sheinpub.Package, main string, order []string, deleted map[string]struct{}) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(pkg.DraftPayload.SKCList) == 0 {
		return
	}
	fallback := sheinFinalDraftFallbackImages(pkg, main, deleted)
	for index := range pkg.DraftPayload.SKCList {
		skcDraft := &pkg.DraftPayload.SKCList[index]
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
		mergedGallery := sheinGalleryWithoutMain(
			orderSheinImages(skcDraft.ImageInfo.Gallery, galleryFallback, deleted),
			firstNonEmpty(skcDraft.ImageInfo.MainImage, mainImage),
		)
		if len(skcDraft.ImageInfo.Gallery) == 0 || len(mergedGallery) > len(skcDraft.ImageInfo.Gallery) {
			skcDraft.ImageInfo.Gallery = mergedGallery
		}
		if pkg.DraftPayload.ImageInfo != nil && strings.TrimSpace(skcDraft.ImageInfo.WhiteBg) == "" {
			skcDraft.ImageInfo.WhiteBg = strings.TrimSpace(pkg.DraftPayload.ImageInfo.WhiteBg)
		}
		for skuIndex := range skcDraft.SKUList {
			if strings.TrimSpace(skcDraft.SKUList[skuIndex].MainImage) == "" {
				skcDraft.SKUList[skuIndex].MainImage = firstNonEmpty(skcDraft.ImageInfo.MainImage, mainImage)
			}
		}
	}
}

func ensureSheinFinalPreviewSKCImages(pkg *sheinpub.Package) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.PreviewPayload == nil || len(pkg.PreviewPayload.SKCList) == 0 {
		return
	}
	roleOverrides := map[string]string(nil)
	if pkg.FinalSubmissionDraft != nil {
		roleOverrides = pkg.FinalSubmissionDraft.ImageRoleOverrides
	}
	for index := range pkg.PreviewPayload.SKCList {
		skc := &pkg.PreviewPayload.SKCList[index]
		draft := sheinRequestDraftSKCByIndexOrCode(pkg.DraftPayload, index, sheinPreviewSKCSupplierCode(skc))
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
