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
	if bundle == nil {
		return nil
	}
	set := &PlatformImageSet{}
	for _, item := range bundle.Assets {
		switch item.Kind {
		case asset.KindSourceImage:
			set.SourceImages = append(set.SourceImages, item.URL)
		case asset.KindWhiteBgImage:
			if set.WhiteBgImage == "" {
				set.WhiteBgImage = item.URL
			}
		case asset.KindGalleryImage, asset.KindSceneImage, asset.KindSellingPointImage, asset.KindSizeSceneImage, asset.KindDetailCrop:
			set.Gallery = append(set.Gallery, item.URL)
		case asset.KindMainImage, asset.KindModelImage, asset.KindCleanImage:
			if set.MainImage == "" {
				set.MainImage = item.URL
			}
		}
	}
	if bundle.Selection != nil {
		if set.MainImage == "" {
			if item := findAssetURL(bundle.Assets, bundle.Selection.MainAssetID); item != "" {
				set.MainImage = item
			}
		}
		if set.WhiteBgImage == "" {
			if item := findAssetURL(bundle.Assets, bundle.Selection.WhiteBgAssetID); item != "" {
				set.WhiteBgImage = item
			}
		}
		if len(set.Gallery) == 0 {
			for _, id := range bundle.Selection.GalleryAssetIDs {
				if url := findAssetURL(bundle.Assets, id); url != "" {
					set.Gallery = append(set.Gallery, url)
				}
			}
		}
	}
	if set.MainImage == "" && len(set.SourceImages) > 0 {
		set.MainImage = set.SourceImages[0]
	}
	if len(set.Gallery) == 0 && len(set.SourceImages) > 1 {
		set.Gallery = append(set.Gallery, set.SourceImages[1:]...)
	}
	if set.MainImage == "" && len(set.Gallery) > 0 {
		set.MainImage = set.Gallery[0]
	}
	if set.MainImage == "" && len(set.SourceImages) == 0 && len(set.Gallery) == 0 && set.WhiteBgImage == "" {
		return nil
	}
	set.Gallery = uniqueStrings(set.Gallery)
	set.SourceImages = uniqueStrings(set.SourceImages)
	return set
}

func findAssetURL(items []asset.Asset, id string) string {
	for _, item := range items {
		if item.ID == id {
			return item.URL
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	return common.FirstNonEmpty(values...)
}

func uniqueStrings(values []string) []string {
	return common.UniqueStrings(values)
}
