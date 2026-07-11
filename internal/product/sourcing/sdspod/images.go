package sdspod

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

func applyImages(product *canonical.Product, metadata CanonicalMetadata) bool {
	trace := canonicalTrace("SDS rendered mockup images", 0.98)
	byKey := renderedImagesByKey(metadata.Variants, trace)
	allVariantImages := imagesFromVariants(metadata.Variants, trace)
	defaultImages := imagesFromMockups(metadata.MockupURLs, trace)
	if len(defaultImages) == 0 {
		defaultImages = firstVariantImages(metadata.Variants, trace)
	}
	if len(defaultImages) == 0 && len(allVariantImages) == 0 {
		return false
	}

	changed := false
	productImages := defaultImages
	if len(allVariantImages) > 0 {
		productImages = allVariantImages
	}
	if len(productImages) > 0 && !imagesEqual(product.Images, productImages) {
		product.Images = copyImages(productImages)
		changed = true
	}

	variantImages := make([][]canonical.Image, len(product.Variants))
	for index := range variantImages {
		variantImages[index] = defaultImages
	}
	for _, lookup := range metadata.VariantLookup {
		index := lookup.CanonicalVariantIndex
		if index < 0 || index >= len(variantImages) {
			continue
		}
		if images := resolveVariantImages(lookup.Keys, byKey); len(images) > 0 {
			variantImages[index] = images
		}
	}
	for index, images := range variantImages {
		if len(images) == 0 || imagesEqual(product.Variants[index].Images, images) {
			continue
		}
		product.Variants[index].Images = copyImages(images)
		changed = true
	}

	if changed {
		if product.FieldTraces == nil {
			product.FieldTraces = map[string]canonical.FieldTrace{}
		}
		product.FieldTraces["images"] = trace
	}
	return changed
}

func renderedImagesByKey(variants []VariantMetadata, trace canonical.FieldTrace) map[string][]canonical.Image {
	result := map[string][]canonical.Image{}
	for _, variant := range variants {
		if variant.Status == "failed" || len(variant.MockupURLs) == 0 {
			continue
		}
		images := imagesFromMockups(variant.MockupURLs, trace)
		if len(images) == 0 {
			continue
		}
		for _, value := range []string{variant.SKU, variant.Color} {
			if key := normalizeKey(value); key != "" {
				result[key] = images
			}
		}
	}
	return result
}

func imagesFromVariants(variants []VariantMetadata, trace canonical.FieldTrace) []canonical.Image {
	seen := map[string]bool{}
	var result []canonical.Image
	for _, variant := range variants {
		if variant.Status == "failed" || len(variant.MockupURLs) == 0 {
			continue
		}
		for _, image := range imagesFromMockups(variant.MockupURLs, trace) {
			url := strings.TrimSpace(image.URL)
			if url == "" || seen[url] {
				continue
			}
			seen[url] = true
			result = append(result, image)
		}
	}
	return result
}

func firstVariantImages(variants []VariantMetadata, trace canonical.FieldTrace) []canonical.Image {
	for _, variant := range variants {
		if variant.Status == "failed" || len(variant.MockupURLs) == 0 {
			continue
		}
		if images := imagesFromMockups(variant.MockupURLs, trace); len(images) > 0 {
			return images
		}
	}
	return nil
}

func imagesFromMockups(urls []string, trace canonical.FieldTrace) []canonical.Image {
	urls = uniqueNonEmpty(urls)
	images := make([]canonical.Image, 0, len(urls))
	for i, url := range urls {
		role := "gallery"
		if i == 0 {
			role = "primary"
		}
		images = append(images, canonical.Image{
			URL: url, Role: role, Trace: trace,
		})
	}
	return images
}

func resolveVariantImages(keys []string, byKey map[string][]canonical.Image) []canonical.Image {
	for _, value := range keys {
		if key := normalizeKey(value); key != "" {
			if images := byKey[key]; len(images) > 0 {
				return images
			}
		}
	}
	return nil
}

func imagesEqual(left, right []canonical.Image) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if strings.TrimSpace(left[i].URL) != strings.TrimSpace(right[i].URL) ||
			strings.TrimSpace(left[i].Role) != strings.TrimSpace(right[i].Role) {
			return false
		}
	}
	return true
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func copyImages(input []canonical.Image) []canonical.Image {
	return append([]canonical.Image(nil), input...)
}
