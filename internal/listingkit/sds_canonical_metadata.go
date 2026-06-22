package listingkit

import (
	"strings"

	"task-processor/internal/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySDSSyncMetadataToCanonical(product *canonical.Product, summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	if product == nil {
		return false
	}
	changed := applySDSIdentityAttributesToCanonical(product, summary, options)
	if applyStudioStyleDimension(product, options) {
		changed = true
	}
	if applySDSRenderedImagesToCanonical(product, summary) {
		changed = true
	}
	title := trustedSDSProductName(summary, options)
	if title == "" {
		return changed
	}
	if strings.TrimSpace(product.Title) == title {
		return changed
	}
	product.Title = title
	if product.FieldTraces == nil {
		product.FieldTraces = map[string]canonical.FieldTrace{}
	}
	product.FieldTraces["title"] = canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: "SDS design product detail",
		}},
		Confidence:  0.96,
		IsInferred:  false,
		NeedsReview: false,
	}
	return true
}

func applySDSIdentityAttributesToCanonical(product *canonical.Product, summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	if product == nil {
		return false
	}
	trace := canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: "SDS design product identity",
		}},
		Confidence:  0.96,
		IsInferred:  false,
		NeedsReview: false,
	}
	attrs := map[string]string{}
	if options != nil {
		for key, attr := range studioAttributes(options, trace) {
			if value := strings.TrimSpace(attr.Value); value != "" {
				attrs[key] = value
			}
		}
	}
	attrs["sku"] = firstNonEmptyString(summaryProductSKU(summary), attrs["sku"])
	attrs["product_sku"] = firstNonEmptyString(summaryProductSKU(summary), attrs["product_sku"])
	attrs["variant_sku"] = firstNonEmptyString(summaryVariantSKU(summary), attrs["variant_sku"])
	attrs["variant_size"] = firstNonEmptyString(summaryVariantSize(summary), attrs["variant_size"])
	attrs["variant_color"] = firstNonEmptyString(summaryVariantColor(summary), attrs["variant_color"])

	changed := false
	for key, value := range attrs {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if product.Attributes == nil {
			product.Attributes = map[string]canonical.Attribute{}
		}
		if existing := strings.TrimSpace(product.Attributes[key].Value); existing == value {
			continue
		}
		product.Attributes[key] = canonical.Attribute{Value: value, Trace: trace}
		changed = true
	}
	if changed {
		if product.FieldTraces == nil {
			product.FieldTraces = map[string]canonical.FieldTrace{}
		}
		product.FieldTraces["attributes"] = trace
	}
	return changed
}

func summaryProductSKU(summary *SDSSyncSummary) string {
	if summary == nil {
		return ""
	}
	return summary.ProductSKU
}

func summaryVariantSKU(summary *SDSSyncSummary) string {
	if summary == nil {
		return ""
	}
	if value := strings.TrimSpace(summary.VariantSKU); value != "" {
		return value
	}
	for _, item := range summary.VariantResults {
		if value := strings.TrimSpace(item.VariantSKU); value != "" {
			return value
		}
	}
	return ""
}

func summaryVariantSize(summary *SDSSyncSummary) string {
	if summary == nil {
		return ""
	}
	if value := strings.TrimSpace(summary.VariantSize); value != "" {
		return value
	}
	for _, item := range summary.VariantResults {
		if value := strings.TrimSpace(item.VariantSize); value != "" {
			return value
		}
	}
	return ""
}

func summaryVariantColor(summary *SDSSyncSummary) string {
	if summary == nil {
		return ""
	}
	if value := strings.TrimSpace(summary.VariantColor); value != "" {
		return value
	}
	for _, item := range summary.VariantResults {
		if value := strings.TrimSpace(item.VariantColor); value != "" {
			return value
		}
	}
	return ""
}

func applySDSRenderedImagesToCanonical(product *canonical.Product, summary *SDSSyncSummary) bool {
	if product == nil || summary == nil {
		return false
	}
	trace := canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: "SDS rendered mockup images",
		}},
		Confidence:  0.98,
		IsInferred:  false,
		NeedsReview: false,
	}
	variantImages := sdsRenderedImagesByVariant(summary, trace)
	defaultImages := canonicalImagesFromSDSMockups(summary.MockupImageURLs, trace)
	if len(defaultImages) == 0 {
		defaultImages = firstSDSVariantResultImages(summary, trace)
	}
	if len(defaultImages) == 0 && len(variantImages) == 0 {
		return false
	}

	changed := false
	if len(variantImages) > 0 {
		allImages := canonicalImagesFromSDSVariantResults(summary, trace)
		if len(allImages) > 0 && !canonicalImagesEqual(product.Images, allImages) {
			product.Images = allImages
			changed = true
		}
		for index := range product.Variants {
			images := resolveSDSCanonicalImagesForVariant(&product.Variants[index], variantImages)
			if len(images) == 0 {
				images = defaultImages
			}
			if len(images) == 0 || canonicalImagesEqual(product.Variants[index].Images, images) {
				continue
			}
			product.Variants[index].Images = append([]canonical.Image(nil), images...)
			changed = true
		}
	} else if len(defaultImages) > 0 {
		if !canonicalImagesEqual(product.Images, defaultImages) {
			product.Images = append([]canonical.Image(nil), defaultImages...)
			changed = true
		}
		for index := range product.Variants {
			if canonicalImagesEqual(product.Variants[index].Images, defaultImages) {
				continue
			}
			product.Variants[index].Images = append([]canonical.Image(nil), defaultImages...)
			changed = true
		}
	}
	if changed {
		if product.FieldTraces == nil {
			product.FieldTraces = map[string]canonical.FieldTrace{}
		}
		product.FieldTraces["images"] = trace
	}
	return changed
}

func sdsRenderedImagesByVariant(summary *SDSSyncSummary, trace canonical.FieldTrace) map[string][]canonical.Image {
	result := map[string][]canonical.Image{}
	if summary == nil {
		return result
	}
	for _, item := range summary.VariantResults {
		if item.Status == "failed" || len(item.MockupImageURLs) == 0 {
			continue
		}
		images := canonicalImagesFromSDSMockups(item.MockupImageURLs, trace)
		if len(images) == 0 {
			continue
		}
		for _, key := range []string{item.VariantSKU, item.VariantColor} {
			normalized := sheinpub.NormalizeSDSImageKey(key)
			if normalized == "__default__" {
				continue
			}
			result[normalized] = images
		}
	}
	return result
}

func canonicalImagesFromSDSVariantResults(summary *SDSSyncSummary, trace canonical.FieldTrace) []canonical.Image {
	if summary == nil {
		return nil
	}
	seen := map[string]bool{}
	var result []canonical.Image
	for _, item := range summary.VariantResults {
		if item.Status == "failed" || len(item.MockupImageURLs) == 0 {
			continue
		}
		for _, image := range canonicalImagesFromSDSMockups(item.MockupImageURLs, trace) {
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

func firstSDSVariantResultImages(summary *SDSSyncSummary, trace canonical.FieldTrace) []canonical.Image {
	if summary == nil {
		return nil
	}
	for _, item := range summary.VariantResults {
		if item.Status == "failed" || len(item.MockupImageURLs) == 0 {
			continue
		}
		if images := canonicalImagesFromSDSMockups(item.MockupImageURLs, trace); len(images) > 0 {
			return images
		}
	}
	return nil
}

func canonicalImagesFromSDSMockups(urls []string, trace canonical.FieldTrace) []canonical.Image {
	urls = uniqueNonEmptyStrings(urls)
	images := make([]canonical.Image, 0, len(urls))
	for index, url := range urls {
		role := "gallery"
		if index == 0 {
			role = "primary"
		}
		images = append(images, canonical.Image{
			URL:   url,
			Role:  role,
			Trace: trace,
		})
	}
	return images
}

func resolveSDSCanonicalImagesForVariant(variant *canonical.Variant, byVariant map[string][]canonical.Image) []canonical.Image {
	if variant == nil {
		return nil
	}
	for _, value := range []string{
		variant.Attributes["source_sds_sku"].Value,
		sheinpub.SourceSDSSKUFromSupplierSKU(variant.SKU),
		variant.SKU,
		variant.Attributes["Color"].Value,
		variant.Attributes["color"].Value,
	} {
		if images := byVariant[sheinpub.NormalizeSDSImageKey(value)]; len(images) > 0 {
			return images
		}
	}
	return nil
}

func canonicalImagesEqual(left, right []canonical.Image) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if strings.TrimSpace(left[index].URL) != strings.TrimSpace(right[index].URL) ||
			strings.TrimSpace(left[index].Role) != strings.TrimSpace(right[index].Role) {
			return false
		}
	}
	return true
}

func trustedSDSProductName(summary *SDSSyncSummary, options *SDSSyncOptions) string {
	if summary != nil {
		if name := strings.TrimSpace(summary.ProductName); name != "" {
			return name
		}
	}
	if options != nil {
		if name := strings.TrimSpace(options.ProductName); name != "" {
			return name
		}
	}
	return ""
}
