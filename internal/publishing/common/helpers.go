package common

import (
	"fmt"
	"strconv"
	"strings"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog/canonical"
)

func BuildVariants(product *canonical.Product) []Variant {
	if product == nil {
		return nil
	}
	if len(product.Variants) == 0 {
		return buildFallbackVariant(product)
	}
	result := make([]Variant, 0, len(product.Variants))
	for _, variant := range product.Variants {
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

func BuildImages(product *canonical.Product) *ImageSet {
	set := &ImageSet{}
	if product == nil {
		return nil
	}
	for _, image := range product.Images {
		url := strings.TrimSpace(image.URL)
		if url == "" {
			continue
		}
		switch productasset.Role(image.Role) {
		case productasset.RoleMain:
			if set.MainImage == "" {
				set.MainImage = url
			}
		case productasset.RoleWhiteBackground:
			if set.WhiteBgImage == "" {
				set.WhiteBgImage = url
			}
		case productasset.RoleDesign, productasset.RoleGallery:
			set.Gallery = append(set.Gallery, url)
		}
	}
	set.Gallery = UniqueStrings(set.Gallery)
	if set.MainImage == "" && set.WhiteBgImage == "" && len(set.Gallery) == 0 {
		return nil
	}
	return set
}

func FlattenAttributes(attributes map[string]canonical.Attribute) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	result := make(map[string]string, len(attributes))
	for key, value := range attributes {
		result[key] = value.Value
	}
	return result
}

func BuildAttributes(attributes map[string]canonical.Attribute) []Attribute {
	if len(attributes) == 0 {
		return nil
	}
	result := make([]Attribute, 0, len(attributes))
	for key, value := range attributes {
		result = append(result, Attribute{Name: key, Value: value.Value})
	}
	return result
}

func CollectReviewNotes(product *canonical.Product, extras ...string) []string {
	notes := make([]string, 0, len(extras)+4)
	if product != nil && product.NeedsReview {
		notes = append(notes, "商品结构化结果存在低置信字段，建议人工复核标题、品牌、属性和变体")
	}
	notes = append(notes, extras...)
	return UniqueStrings(notes)
}

func ResolveBrand(brandHint string, product *canonical.Product) string {
	if strings.TrimSpace(brandHint) != "" {
		return strings.TrimSpace(brandHint)
	}
	if product == nil {
		return ""
	}
	return product.Brand
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
