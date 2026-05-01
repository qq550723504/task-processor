package productenrich

import (
	"strconv"
	"strings"
)

type CanonicalSourceType string

const (
	CanonicalSourceUserText    CanonicalSourceType = "user_text"
	CanonicalSourceUserImage   CanonicalSourceType = "user_image"
	CanonicalSourceProductURL  CanonicalSourceType = "product_url"
	CanonicalSourceScrapedData CanonicalSourceType = "scraped_data"
	CanonicalSourceLLM         CanonicalSourceType = "llm"
	CanonicalSourceDerived     CanonicalSourceType = "derived"
)

type CanonicalSource struct {
	Type   CanonicalSourceType `json:"type"`
	Detail string              `json:"detail,omitempty"`
}

type FieldTrace struct {
	Sources     []CanonicalSource `json:"sources,omitempty"`
	Confidence  float64           `json:"confidence,omitempty"`
	IsInferred  bool              `json:"is_inferred,omitempty"`
	NeedsReview bool              `json:"needs_review,omitempty"`
}

type CanonicalAttribute struct {
	Value string     `json:"value"`
	Trace FieldTrace `json:"trace"`
}

type CanonicalImage struct {
	URL   string     `json:"url"`
	Role  string     `json:"role,omitempty"`
	Trace FieldTrace `json:"trace"`
}

type CanonicalVariant struct {
	SKU        string                        `json:"sku"`
	Attributes map[string]CanonicalAttribute `json:"attributes,omitempty"`
	Price      *PriceInfo                    `json:"price,omitempty"`
	Stock      int                           `json:"stock,omitempty"`
	Images     []CanonicalImage              `json:"images,omitempty"`
	Dimensions *Dimensions                   `json:"dimensions,omitempty"`
	Weight     *Weight                       `json:"weight,omitempty"`
	Barcode    string                        `json:"barcode,omitempty"`
	IsDefault  bool                          `json:"is_default,omitempty"`
	Trace      FieldTrace                    `json:"trace"`
}

type CanonicalProduct struct {
	Title             string                        `json:"title,omitempty"`
	Brand             string                        `json:"brand,omitempty"`
	CategoryPath      []string                      `json:"category_path,omitempty"`
	Description       string                        `json:"description,omitempty"`
	SellingPoints     []string                      `json:"selling_points,omitempty"`
	SEOKeywords       []string                      `json:"seo_keywords,omitempty"`
	Attributes        map[string]CanonicalAttribute `json:"attributes,omitempty"`
	Specifications    *ProductSpecs                 `json:"specifications,omitempty"`
	VariantDimensions []ScrapedVariantDimension     `json:"variant_dimensions,omitempty"`
	Variants          []CanonicalVariant            `json:"variants,omitempty"`
	Images            []CanonicalImage              `json:"images,omitempty"`
	FieldTraces       map[string]FieldTrace         `json:"field_traces,omitempty"`
	NeedsReview       bool                          `json:"needs_review,omitempty"`
}

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
	sources := append([]CanonicalSource(nil), base...)
	if inferred {
		sources = append(sources, CanonicalSource{Type: CanonicalSourceLLM, Detail: "LLM-generated product normalization"})
	}

	confidence := 0.6
	if hasSourceType(base, CanonicalSourceProductURL) {
		confidence = 0.9
	} else if hasSourceType(base, CanonicalSourceUserText) && hasSourceType(base, CanonicalSourceUserImage) {
		confidence = 0.82
	} else if hasSourceType(base, CanonicalSourceUserText) || hasSourceType(base, CanonicalSourceUserImage) {
		confidence = 0.72
	}
	if inferred {
		confidence -= 0.08
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	return FieldTrace{
		Sources:     sources,
		Confidence:  confidence,
		IsInferred:  inferred,
		NeedsReview: confidence < 0.75,
	}
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
	if product == nil {
		return true
	}
	if strings.TrimSpace(product.Title) == "" || strings.TrimSpace(product.Description) == "" {
		return true
	}
	for _, trace := range product.FieldTraces {
		if trace.NeedsReview {
			return true
		}
	}
	return false
}

func inferImageRole(idx int) string {
	if idx == 0 {
		return "primary"
	}
	return "gallery"
}

func hasSourceType(sources []CanonicalSource, want CanonicalSourceType) bool {
	for _, source := range sources {
		if source.Type == want {
			return true
		}
	}
	return false
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
		return trace
	}
	extra := append([]CanonicalSource(nil), product.Evidence[field]...)
	if len(extra) == 0 && strings.HasPrefix(field, "brand") {
		extra = append(extra, product.Evidence["attributes.brand"]...)
	}
	if len(extra) == 0 {
		return trace
	}
	trace.Sources = append(trace.Sources, extra...)
	return trace
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
