package common

import (
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func BuildVariants(canonical *productenrich.CanonicalProduct) []Variant {
	if canonical == nil {
		return nil
	}
	if len(canonical.Variants) == 0 {
		return buildFallbackVariant(canonical)
	}
	result := make([]Variant, 0, len(canonical.Variants))
	for _, variant := range canonical.Variants {
		attributes := make(map[string]string, len(variant.Attributes))
		for key, value := range variant.Attributes {
			attributes[key] = value.Value
		}
		item := Variant{
			SKU:        variant.SKU,
			Attributes: attributes,
			Stock:      variant.Stock,
			Barcode:    variant.Barcode,
			IsDefault:  variant.IsDefault,
			Dimensions: variant.Dimensions,
			Weight:     variant.Weight,
		}
		if variant.Price != nil {
			item.Price = &Price{
				Currency:  variant.Price.Currency,
				Amount:    variant.Price.Amount,
				CostPrice: variant.Price.CostPrice,
			}
		}
		if len(variant.Images) > 0 {
			item.Image = variant.Images[0].URL
		}
		result = append(result, item)
	}
	return result
}

func BuildImages(canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *ImageSet {
	return BuildImagesFromBundle(asset.BuildBundle(canonical, image))
}

func BuildImagesFromBundle(bundle *asset.Bundle) *ImageSet {
	set := &ImageSet{}
	if bundle != nil {
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
	}
	if set.MainImage == "" && len(set.Gallery) > 0 {
		set.MainImage = set.Gallery[0]
	}
	if len(set.Gallery) == 0 && len(set.SourceImages) > 1 {
		set.Gallery = append(set.Gallery, set.SourceImages[1:]...)
	}
	if set.MainImage == "" && len(set.SourceImages) == 0 && len(set.Gallery) == 0 && set.WhiteBgImage == "" {
		return nil
	}
	set.Gallery = UniqueStrings(set.Gallery)
	set.SourceImages = UniqueStrings(set.SourceImages)
	return set
}

func FlattenAttributes(attributes map[string]productenrich.CanonicalAttribute) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	result := make(map[string]string, len(attributes))
	for key, value := range attributes {
		result[key] = value.Value
	}
	return result
}

func BuildAttributes(attributes map[string]productenrich.CanonicalAttribute) []Attribute {
	if len(attributes) == 0 {
		return nil
	}
	result := make([]Attribute, 0, len(attributes))
	for key, value := range attributes {
		result = append(result, Attribute{Name: key, Value: value.Value})
	}
	return result
}

func CollectReviewNotes(canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult, extras ...string) []string {
	notes := make([]string, 0, len(extras)+4)
	if canonical != nil && canonical.NeedsReview {
		notes = append(notes, "商品结构化结果存在低置信字段，建议人工复核标题、品牌、属性和变体")
	}
	if image != nil && image.Review != nil && image.Review.NeedsReview {
		notes = append(notes, image.Review.Reasons...)
	}
	notes = append(notes, extras...)
	return UniqueStrings(notes)
}

func ResolveBrand(brandHint string, canonical *productenrich.CanonicalProduct) string {
	if strings.TrimSpace(brandHint) != "" {
		return strings.TrimSpace(brandHint)
	}
	if canonical == nil {
		return ""
	}
	return canonical.Brand
}

func WithBrandHint(title, brandHint string) string {
	title = strings.TrimSpace(title)
	brand := strings.TrimSpace(brandHint)
	if brand == "" {
		return title
	}
	if title == "" {
		return brand
	}
	if strings.Contains(strings.ToLower(title), strings.ToLower(brand)) {
		return title
	}
	return fmt.Sprintf("%s %s", brand, title)
}

func DefaultSites(country string) []Site {
	country = strings.ToUpper(strings.TrimSpace(country))
	if country == "" {
		country = "US"
	}
	return []Site{{MainSite: country, SubSites: []string{country}}}
}

func FormatFloat(value float64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func CloneMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func ParseFloatDefault(value string) float64 {
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

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func LastCategory(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return path[len(path)-1]
}

func UniqueStrings(values []string) []string {
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
