package shein

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

// ReplaceImagesWithAIProductImages replaces package images with generated AI product images.
func ReplaceImagesWithAIProductImages(pkg *Package, imageURLs []string, sourceImages []string) {
	pkg = NormalizePackageSemanticFields(pkg)
	images := ImageSetFromAIProductImages(imageURLs, sourceImages)
	if images == nil {
		return
	}
	pkg.Images = images
	if pkg.DraftPayload != nil {
		pkg.DraftPayload.ImageInfo = BuildImageDraft(images)
		for skcIndex := range pkg.DraftPayload.SKCList {
			pkg.DraftPayload.SKCList[skcIndex].ImageInfo = BuildImageDraft(images)
			for skuIndex := range pkg.DraftPayload.SKCList[skcIndex].SKUList {
				pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex].MainImage = images.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		pkg.SkcList[skcIndex].MainImageURL = images.MainImage
	}
	preview := BuildPreviewProduct(pkg)
	SetPreviewPayload(pkg, preview)
}

// AppendAIProductImages appends generated AI product images to existing package images.
func AppendAIProductImages(pkg *Package, imageURLs []string, sourceImages []string) {
	pkg = NormalizePackageSemanticFields(pkg)
	if len(imageURLs) == 0 {
		return
	}
	if pkg.Images == nil || strings.TrimSpace(pkg.Images.MainImage) == "" {
		ReplaceImagesWithAIProductImages(pkg, imageURLs, sourceImages)
		return
	}
	pkg.Images.SourceImages = uniqueNonEmptySDSImageStrings(append(pkg.Images.SourceImages, sourceImages...))
	pkg.Images.Gallery = appendUniqueSDSImageURLs(pkg.Images.Gallery, imageURLs...)
	if pkg.DraftPayload != nil {
		pkg.DraftPayload.ImageInfo = BuildImageDraft(pkg.Images)
		for skcIndex := range pkg.DraftPayload.SKCList {
			skcImages := ImageDraftToSet(pkg.DraftPayload.SKCList[skcIndex].ImageInfo)
			if skcImages == nil || strings.TrimSpace(skcImages.MainImage) == "" {
				skcImages = pkg.Images
			} else {
				skcImages.SourceImages = uniqueNonEmptySDSImageStrings(append(skcImages.SourceImages, sourceImages...))
				skcImages.Gallery = appendUniqueSDSImageURLs(skcImages.Gallery, imageURLs...)
			}
			pkg.DraftPayload.SKCList[skcIndex].ImageInfo = BuildImageDraft(skcImages)
		}
	}
	preview := BuildPreviewProduct(pkg)
	SetPreviewPayload(pkg, preview)
}

// ImageSetFromAIProductImages builds a publishing image set from generated AI product images.
func ImageSetFromAIProductImages(imageURLs []string, sourceImages []string) *common.ImageSet {
	imageURLs = uniqueNonEmptySDSImageStrings(imageURLs)
	if len(imageURLs) == 0 {
		return nil
	}
	images := &common.ImageSet{
		MainImage:    imageURLs[0],
		SourceImages: uniqueNonEmptySDSImageStrings(sourceImages),
	}
	if len(imageURLs) > 1 {
		images.Gallery = append([]string(nil), imageURLs[1:]...)
	}
	return images
}

// ImageDraftToSet converts a SHEIN image draft into a publishing image set.
func ImageDraftToSet(draft *ImageDraft) *common.ImageSet {
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
