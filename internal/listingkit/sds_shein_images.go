package listingkit

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySDSTemplateImagesToShein(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string) {
	if pkg == nil || summary == nil || len(summary.MockupImageURLs) == 0 {
		return
	}

	images := &common.ImageSet{
		MainImage:    summary.MockupImageURLs[0],
		SourceImages: uniqueNonEmptyStrings(sourceImages),
	}
	if len(summary.MockupImageURLs) > 1 {
		images.Gallery = append([]string(nil), summary.MockupImageURLs[1:]...)
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
