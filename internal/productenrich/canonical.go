package productenrich

import (
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
)

type CanonicalSourceType = canonical.SourceType

const (
	CanonicalSourceUserText    = canonical.SourceUserText
	CanonicalSourceUserImage   = canonical.SourceUserImage
	CanonicalSourceProductURL  = canonical.SourceProductURL
	CanonicalSourceScrapedData = canonical.SourceScrapedData
	CanonicalSourceLLM         = canonical.SourceLLM
	CanonicalSourceDerived     = canonical.SourceDerived
)

type CanonicalSource = canonical.Source
type FieldTrace = canonical.FieldTrace
type CanonicalAttribute = canonical.Attribute
type CanonicalImage = canonical.Image
type CanonicalVariant = canonical.Variant
type CanonicalProduct = canonical.Product

// BuildCanonicalProduct lifts the current ProductJSON output into a platform-neutral
// product model with basic provenance metadata so downstream packages can start
// consuming a more explicit contract without forcing a big-bang refactor.
func BuildCanonicalProduct(req *GenerateRequest, product *ProductJSON) *CanonicalProduct {
	if product == nil {
		return nil
	}

	baseSources := inferBaseSources(req)
	directTrace := buildFieldTrace(baseSources, false)

	canonical := &CanonicalProduct{
		Title:             product.Title,
		Brand:             strings.TrimSpace(product.Attributes["brand"]),
		CategoryPath:      cloneStrings(product.Category),
		Description:       product.Description,
		SellingPoints:     cloneStrings(product.SellingPoints),
		SEOKeywords:       cloneStrings(product.SEOKeywords),
		Attributes:        make(map[string]CanonicalAttribute, len(product.Attributes)),
		Specifications:    product.Specifications,
		VariantDimensions: append([]ScrapedVariantDimension(nil), product.VariantDimensions...),
		Variants:          make([]CanonicalVariant, 0, len(product.Variants)),
		Images:            make([]CanonicalImage, 0, len(product.Images)),
		FieldTraces:       map[string]FieldTrace{},
	}

	canonical.FieldTraces["title"] = traceWithEvidence(product, "title", baseSources, true)
	canonical.FieldTraces["brand"] = traceForBrand(product, baseSources)
	canonical.FieldTraces["category_path"] = traceWithEvidence(product, "category_path", baseSources, true)
	canonical.FieldTraces["description"] = traceWithEvidence(product, "description", baseSources, true)
	canonical.FieldTraces["selling_points"] = traceWithEvidence(product, "selling_points", baseSources, true)
	canonical.FieldTraces["seo_keywords"] = traceWithEvidence(product, "seo_keywords", baseSources, true)
	if product.Specifications != nil {
		canonical.FieldTraces["specifications"] = traceWithEvidence(product, "specifications", baseSources, true)
	}

	for key, value := range product.Attributes {
		canonical.Attributes[key] = CanonicalAttribute{
			Value: value,
			Trace: traceForAttribute(product, key, baseSources),
		}
	}

	for idx, imageURL := range product.Images {
		image := CanonicalImage{
			URL:   imageURL,
			Role:  inferImageRole(idx),
			Trace: directTrace,
		}
		canonical.Images = append(canonical.Images, image)
	}

	for _, variant := range product.Variants {
		converted := CanonicalVariant{
			SKU:        variant.SKU,
			Attributes: make(map[string]CanonicalAttribute, len(variant.Attributes)),
			Price:      variant.Price,
			Stock:      variant.Stock,
			Barcode:    variant.Barcode,
			IsDefault:  variant.IsDefault,
			Trace:      traceWithEvidence(product, "variants", baseSources, true),
		}
		for key, value := range variant.Attributes {
			converted.Attributes[key] = CanonicalAttribute{
				Value: value,
				Trace: traceWithEvidence(product, "variants.attributes."+key, baseSources, true),
			}
		}
		for _, imageURL := range variant.Images {
			converted.Images = append(converted.Images, CanonicalImage{
				URL:   imageURL,
				Role:  "variant",
				Trace: directTrace,
			})
		}
		canonical.Variants = append(canonical.Variants, converted)
	}

	canonical.NeedsReview = canonicalNeedsReview(canonical)
	return canonical
}

func inferBaseSources(req *GenerateRequest) []CanonicalSource {
	sources := make([]CanonicalSource, 0, 4)
	if req == nil {
		return []CanonicalSource{{Type: CanonicalSourceDerived, Detail: "unknown_request_context"}}
	}
	if strings.TrimSpace(req.Text) != "" {
		sources = append(sources, CanonicalSource{Type: CanonicalSourceUserText, Detail: summarizeUserText(req.Text)})
	}
	if len(req.ImageURLs) > 0 {
		sources = append(sources, CanonicalSource{Type: CanonicalSourceUserImage, Detail: summarizeImageSources(req.ImageURLs)})
	}
	if strings.TrimSpace(req.ProductURL) != "" {
		sources = append(sources,
			CanonicalSource{Type: CanonicalSourceProductURL, Detail: req.ProductURL},
			CanonicalSource{Type: CanonicalSourceScrapedData, Detail: "normalized from product page: " + strings.TrimSpace(req.ProductURL)},
		)
	}
	if len(sources) == 0 {
		sources = append(sources, CanonicalSource{Type: CanonicalSourceDerived, Detail: "empty_request"})
	}
	return sources
}

func buildFieldTrace(base []CanonicalSource, inferred bool) FieldTrace {
	return canonical.BuildFieldTrace(base, inferred)
}

func traceForAttribute(product *ProductJSON, key string, base []CanonicalSource) FieldTrace {
	key = strings.ToLower(strings.TrimSpace(key))
	trace := traceWithEvidence(product, "attributes."+key, base, true)
	if key == "brand" || key == "material" || key == "color" {
		trace.Confidence += 0.05
		if trace.Confidence > 1 {
			trace.Confidence = 1
		}
		trace.NeedsReview = trace.Confidence < 0.8
	}
	return trace
}

func traceForBrand(product *ProductJSON, base []CanonicalSource) FieldTrace {
	if product == nil {
		return buildFieldTrace(base, true)
	}
	if brand, ok := product.Attributes["brand"]; ok && strings.TrimSpace(brand) != "" {
		trace := traceWithEvidence(product, "brand", base, true)
		trace.Confidence += 0.08
		if trace.Confidence > 1 {
			trace.Confidence = 1
		}
		trace.NeedsReview = strings.EqualFold(strings.TrimSpace(brand), "generic")
		return trace
	}
	return traceWithEvidence(product, "brand", base, true)
}

func canonicalNeedsReview(product *CanonicalProduct) bool {
	return canonical.ProductNeedsReview(product)
}

func inferImageRole(idx int) string {
	if idx == 0 {
		return "primary"
	}
	return "gallery"
}

func hasSourceType(sources []CanonicalSource, want CanonicalSourceType) bool {
	return canonical.HasSourceType(sources, want)
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func traceWithEvidence(product *ProductJSON, field string, base []CanonicalSource, inferred bool) FieldTrace {
	trace := buildFieldTrace(base, inferred)
	if product == nil || len(product.Evidence) == 0 {
		if shouldReviewLLMOnlySourceFact(base, inferred, nil) {
			trace.NeedsReview = true
		}
		return trace
	}
	extra := append([]CanonicalSource(nil), product.Evidence[field]...)
	if len(extra) == 0 && strings.HasPrefix(field, "brand") {
		extra = append(extra, product.Evidence["attributes.brand"]...)
	}
	if len(extra) == 0 {
		extra = append(extra, evidenceWithPrefix(product.Evidence, field+".")...)
	}
	if len(extra) == 0 {
		if shouldReviewLLMOnlySourceFact(base, inferred, nil) {
			trace.NeedsReview = true
		}
		return trace
	}
	trace.Sources = append(trace.Sources, extra...)
	if shouldReviewLLMOnlySourceFact(base, inferred, extra) {
		trace.NeedsReview = true
	}
	return trace
}

func evidenceWithPrefix(evidence map[string][]CanonicalSource, prefix string) []CanonicalSource {
	if len(evidence) == 0 || prefix == "" {
		return nil
	}
	var sources []CanonicalSource
	for key, items := range evidence {
		if strings.HasPrefix(key, prefix) {
			sources = append(sources, items...)
		}
	}
	return sources
}

func shouldReviewLLMOnlySourceFact(base []CanonicalSource, inferred bool, fieldEvidence []CanonicalSource) bool {
	return canonical.ShouldReviewLLMOnlySourceFact(base, inferred, fieldEvidence)
}

func summarizeUserText(text string) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if text == "" {
		return ""
	}
	if len(text) <= 96 {
		return `user input: "` + text + `"`
	}
	return `user input: "` + text[:93] + `..."`
}

func summarizeImageSources(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	first := strings.TrimSpace(urls[0])
	if len(urls) == 1 {
		return "user image: " + first
	}
	return "user images: " + first + " +" + strconv.Itoa(len(urls)-1) + " more"
}
