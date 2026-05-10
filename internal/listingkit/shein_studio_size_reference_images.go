package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

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
