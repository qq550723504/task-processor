package sdspod

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

const studioStyleAttributeKey = "ai_style"

func ApplyCanonical(product *canonical.Product, metadata CanonicalMetadata) bool {
	if product == nil {
		return false
	}
	changed := applyIdentity(product, metadata)
	if applyStyle(product, metadata.StyleName) {
		changed = true
	}
	if applyImages(product, metadata) {
		changed = true
	}
	if applyTitle(product, metadata.ProductName) {
		changed = true
	}
	return changed
}

func applyTitle(product *canonical.Product, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.TrimSpace(product.Title) == value {
		return false
	}
	product.Title = value
	if product.FieldTraces == nil {
		product.FieldTraces = map[string]canonical.FieldTrace{}
	}
	product.FieldTraces["title"] = canonicalTrace(
		"SDS design product detail", 0.96)
	return true
}

func applyIdentity(product *canonical.Product, metadata CanonicalMetadata) bool {
	values := copyStringMap(metadata.Attributes)
	setPreferred(values, "sku", metadata.ProductSKU)
	setPreferred(values, "product_sku", metadata.ProductSKU)
	setPreferred(values, "variant_sku", metadata.VariantSKU)
	setPreferred(values, "variant_size", metadata.VariantSize)
	setPreferred(values, "variant_color", metadata.VariantColor)

	trace := canonicalTrace("SDS design product identity", 0.96)
	changed := false
	for key, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if product.Attributes == nil {
			product.Attributes = map[string]canonical.Attribute{}
		}
		if strings.TrimSpace(product.Attributes[key].Value) == value {
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

func applyStyle(product *canonical.Product, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(product.Variants) == 0 {
		return false
	}
	trace := canonicalTrace("SDS studio AI style dimension", 0.94)
	changed := false
	for i := range product.Variants {
		if product.Variants[i].Attributes == nil {
			product.Variants[i].Attributes = map[string]canonical.Attribute{}
		}
		if strings.TrimSpace(
			product.Variants[i].Attributes[studioStyleAttributeKey].Value) == value {
			continue
		}
		product.Variants[i].Attributes[studioStyleAttributeKey] =
			canonical.Attribute{Value: value, Trace: trace}
		changed = true
	}
	return changed
}

func canonicalTrace(detail string, confidence float64) canonical.FieldTrace {
	return canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: detail,
		}},
		Confidence:  confidence,
		IsInferred:  false,
		NeedsReview: false,
	}
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input)+5)
	for key, value := range input {
		out[key] = value
	}
	return out
}

func setPreferred(values map[string]string, key, preferred string) {
	if preferred = strings.TrimSpace(preferred); preferred != "" {
		values[key] = preferred
	}
}
