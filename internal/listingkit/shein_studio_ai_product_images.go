package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

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
