package shein

import (
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

// ApplySizeReferenceImages marks size reference images in draft and preview payloads.
func ApplySizeReferenceImages(pkg *Package, imageURLs []string) {
	refs := uniqueNonEmptySDSImageStrings(imageURLs)
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || len(refs) == 0 {
		return
	}
	if pkg.Images != nil {
		pkg.Images.Gallery = appendUniqueSDSImageURLs(pkg.Images.Gallery, refs...)
	}
	if pkg.DraftPayload != nil {
		if pkg.DraftPayload.ImageInfo != nil {
			pkg.DraftPayload.ImageInfo.Gallery = appendUniqueSDSImageURLs(pkg.DraftPayload.ImageInfo.Gallery, refs...)
		}
		for skcIndex := range pkg.DraftPayload.SKCList {
			if pkg.DraftPayload.SKCList[skcIndex].ImageInfo != nil {
				pkg.DraftPayload.SKCList[skcIndex].ImageInfo.Gallery = appendUniqueSDSImageURLs(pkg.DraftPayload.SKCList[skcIndex].ImageInfo.Gallery, refs...)
			}
		}
	}
	if pkg.PreviewPayload == nil {
		SetPreviewPayload(pkg, BuildPreviewProduct(pkg))
	}
	if pkg.PreviewPayload != nil {
		EnsureSizeReferenceDetails(pkg.PreviewPayload.ImageInfo, refs)
		for skcIndex := range pkg.PreviewPayload.SKCList {
			EnsureSizeReferenceDetails(&pkg.PreviewPayload.SKCList[skcIndex].ImageInfo, refs)
		}
	}
}

// EnsureSizeReferenceDetails ensures each reference URL is represented as a size-map image.
func EnsureSizeReferenceDetails(info *sheinproduct.ImageInfo, refs []string) {
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
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
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
