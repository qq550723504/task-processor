package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
)

func buildPlatformImages(canonical *canonical.Product, image *productimage.ImageProcessResult) *PlatformImageSet {
	return buildPlatformImagesFromAssetBundle(asset.BuildBundle(canonical, image))
}

func buildPlatformImagesFromAssetBundle(bundle *asset.Bundle) *PlatformImageSet {
	return common.BuildImagesFromBundleWithSelection(bundle)
}
