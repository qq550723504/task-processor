package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestApplySizeReferenceImagesMarksPreviewProductAndSKC(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		Images: &common.ImageSet{MainImage: "main.jpg"},
		RequestDraft: &RequestDraft{
			ImageInfo: BuildImageDraft(&common.ImageSet{MainImage: "main.jpg"}),
			SKCList: []SKCRequestDraft{{
				ImageInfo: BuildImageDraft(&common.ImageSet{MainImage: "main.jpg"}),
			}},
		},
	}

	ApplySizeReferenceImages(pkg, []string{"size.jpg"})

	if got := pkg.Images.Gallery; len(got) != 1 || got[0] != "size.jpg" {
		t.Fatalf("package gallery = %#v, want size reference", got)
	}
	if pkg.PreviewProduct == nil || !hasSizeReferenceDetail(pkg.PreviewProduct.ImageInfo, "size.jpg") {
		t.Fatalf("preview product image info = %+v, want size reference", pkg.PreviewProduct)
	}
	if !hasSizeReferenceDetail(&pkg.PreviewProduct.SKCList[0].ImageInfo, "size.jpg") {
		t.Fatalf("preview skc image info = %+v, want size reference", pkg.PreviewProduct.SKCList[0].ImageInfo)
	}
}

func TestEnsureSizeReferenceDetailsUpdatesExistingImage(t *testing.T) {
	t.Parallel()

	info := &sheinproduct.ImageInfo{
		ImageInfoList: []sheinproduct.ImageDetail{{
			ImageType: 2,
			ImageURL:  "size.jpg",
		}},
	}

	EnsureSizeReferenceDetails(info, []string{"size.jpg"})

	if len(info.ImageInfoList) != 1 {
		t.Fatalf("image count = %d, want existing image updated", len(info.ImageInfoList))
	}
	image := info.ImageInfoList[0]
	if image.ImageType != 6 || !image.SizeImgFlag {
		t.Fatalf("image = %+v, want size-map role", image)
	}
}

func hasSizeReferenceDetail(info *sheinproduct.ImageInfo, url string) bool {
	if info == nil {
		return false
	}
	for _, image := range info.ImageInfoList {
		if image.ImageURL == url && image.ImageType == 6 && image.SizeImgFlag {
			return true
		}
	}
	return false
}
