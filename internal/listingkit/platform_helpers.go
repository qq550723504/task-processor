package listingkit

import (
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
)

func buildPlatformVariants(canonical *canonical.Product) []PlatformVariant {
	variants := common.BuildVariants(canonical)
	if len(variants) == 0 {
		return nil
	}
	return append([]PlatformVariant(nil), variants...)
}

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

func flattenAttributes(attributes map[string]canonical.Attribute) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	result := make(map[string]string, len(attributes))
	for key, value := range attributes {
		result[key] = value.Value
	}
	return result
}

func buildPlatformAttributes(attributes map[string]canonical.Attribute) []PlatformAttribute {
	if len(attributes) == 0 {
		return nil
	}
	result := make([]PlatformAttribute, 0, len(attributes))
	for key, value := range attributes {
		result = append(result, PlatformAttribute{
			Name:  key,
			Value: value.Value,
		})
	}
	return result
}

func collectReviewNotes(canonical *canonical.Product, image *productimage.ImageProcessResult, extras ...string) []string {
	notes := make([]string, 0, len(extras)+4)
	if canonical != nil && canonical.NeedsReview {
		notes = append(notes, "商品结构化结果存在低置信字段，建议人工复核标题、品牌、属性和变体")
	}
	if image != nil && image.Review != nil && image.Review.NeedsReview {
		notes = append(notes, image.Review.Reasons...)
	}
	notes = append(notes, extras...)
	return uniqueStrings(notes)
}

func resolveBrand(canonical *canonical.Product, req *GenerateRequest) string {
	if req != nil && strings.TrimSpace(req.BrandHint) != "" {
		return strings.TrimSpace(req.BrandHint)
	}
	if canonical == nil {
		return ""
	}
	return canonical.Brand
}

func withBrandHint(title string, req *GenerateRequest) string {
	title = strings.TrimSpace(title)
	if req == nil || strings.TrimSpace(req.BrandHint) == "" {
		return title
	}
	brand := strings.TrimSpace(req.BrandHint)
	if title == "" {
		return brand
	}
	if strings.Contains(strings.ToLower(title), strings.ToLower(brand)) {
		return title
	}
	return fmt.Sprintf("%s %s", brand, title)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func lastCategory(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return path[len(path)-1]
}

func defaultPlatformSites(req *GenerateRequest) []PlatformSite {
	if req == nil {
		return nil
	}
	country := strings.ToUpper(strings.TrimSpace(req.Country))
	if country == "" {
		country = "US"
	}
	return []PlatformSite{{
		MainSite: country,
		SubSites: []string{country},
	}}
}

func formatFloat(value float64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func cloneMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func parseFloatDefault(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
