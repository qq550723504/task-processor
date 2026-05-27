package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func applySheinSizeReferenceImages(pkg *sheinpub.Package, imageURLs []string) {
	refs := uniqueNonEmptyStrings(imageURLs)
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || len(refs) == 0 {
		return
	}
	if pkg.Images != nil {
		pkg.Images.Gallery = appendUniqueImageURLs(pkg.Images.Gallery, refs...)
	}
	if pkg.DraftPayload != nil {
		if pkg.DraftPayload.ImageInfo != nil {
			pkg.DraftPayload.ImageInfo.Gallery = appendUniqueImageURLs(pkg.DraftPayload.ImageInfo.Gallery, refs...)
		}
		for skcIndex := range pkg.DraftPayload.SKCList {
			if pkg.DraftPayload.SKCList[skcIndex].ImageInfo != nil {
				pkg.DraftPayload.SKCList[skcIndex].ImageInfo.Gallery = appendUniqueImageURLs(pkg.DraftPayload.SKCList[skcIndex].ImageInfo.Gallery, refs...)
			}
		}
	}
	if pkg.PreviewPayload == nil {
		sheinpub.SetPreviewPayload(pkg, sheinpub.BuildPreviewProduct(pkg))
	}
	if pkg.PreviewPayload != nil {
		ensureSheinSizeReferenceDetails(pkg.PreviewPayload.ImageInfo, refs)
		for skcIndex := range pkg.PreviewPayload.SKCList {
			ensureSheinSizeReferenceDetails(&pkg.PreviewPayload.SKCList[skcIndex].ImageInfo, refs)
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
