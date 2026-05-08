package canonical

import "strings"

type SourceType string

const (
	SourceUserText    SourceType = "user_text"
	SourceUserImage   SourceType = "user_image"
	SourceProductURL  SourceType = "product_url"
	SourceScrapedData SourceType = "scraped_data"
	SourceLLM         SourceType = "llm"
	SourceDerived     SourceType = "derived"
)

type Source struct {
	Type   SourceType `json:"type"`
	Detail string     `json:"detail,omitempty"`
}

type FieldTrace struct {
	Sources     []Source `json:"sources,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
	IsInferred  bool     `json:"is_inferred,omitempty"`
	NeedsReview bool     `json:"needs_review,omitempty"`
}

type Attribute struct {
	Value string     `json:"value"`
	Trace FieldTrace `json:"trace"`
}

type Image struct {
	URL   string     `json:"url"`
	Role  string     `json:"role,omitempty"`
	Trace FieldTrace `json:"trace"`
}

type Variant struct {
	SKU        string               `json:"sku"`
	Attributes map[string]Attribute `json:"attributes,omitempty"`
	Price      *PriceInfo           `json:"price,omitempty"`
	Stock      int                  `json:"stock,omitempty"`
	Images     []Image              `json:"images,omitempty"`
	Dimensions *Dimensions          `json:"dimensions,omitempty"`
	Weight     *Weight              `json:"weight,omitempty"`
	Barcode    string               `json:"barcode,omitempty"`
	IsDefault  bool                 `json:"is_default,omitempty"`
	Trace      FieldTrace           `json:"trace"`
}

type Product struct {
	Title             string                    `json:"title,omitempty"`
	Brand             string                    `json:"brand,omitempty"`
	CategoryPath      []string                  `json:"category_path,omitempty"`
	Description       string                    `json:"description,omitempty"`
	SellingPoints     []string                  `json:"selling_points,omitempty"`
	SEOKeywords       []string                  `json:"seo_keywords,omitempty"`
	Attributes        map[string]Attribute      `json:"attributes,omitempty"`
	Specifications    *ProductSpecs             `json:"specifications,omitempty"`
	VariantDimensions []ScrapedVariantDimension `json:"variant_dimensions,omitempty"`
	Variants          []Variant                 `json:"variants,omitempty"`
	Images            []Image                   `json:"images,omitempty"`
	FieldTraces       map[string]FieldTrace     `json:"field_traces,omitempty"`
	NeedsReview       bool                      `json:"needs_review,omitempty"`
}

type ProductSpecs struct {
	Dimensions *Dimensions       `json:"dimensions,omitempty"`
	Weight     *Weight           `json:"weight,omitempty"`
	Package    *PackageInfo      `json:"package,omitempty"`
	Technical  map[string]string `json:"technical,omitempty"`
}

type Dimensions struct {
	Length float64 `json:"length"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Unit   string  `json:"unit"`
}

type Weight struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type PackageInfo struct {
	Dimensions *Dimensions `json:"dimensions,omitempty"`
	Weight     *Weight     `json:"weight,omitempty"`
	Quantity   int         `json:"quantity"`
}

type PriceInfo struct {
	Currency     string  `json:"currency"`
	Amount       float64 `json:"amount"`
	CompareAt    float64 `json:"compare_at,omitempty"`
	CostPrice    float64 `json:"cost_price,omitempty"`
	WholesaleMin int     `json:"wholesale_min,omitempty"`
}

type ScrapedVariantDimension struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func HasSourceType(sources []Source, want SourceType) bool {
	for _, source := range sources {
		if source.Type == want {
			return true
		}
	}
	return false
}

func BuildFieldTrace(base []Source, inferred bool) FieldTrace {
	sources := append([]Source(nil), base...)
	if inferred {
		sources = append(sources, Source{Type: SourceLLM, Detail: "LLM-generated product normalization"})
	}

	confidence := 0.6
	if HasSourceType(base, SourceProductURL) {
		confidence = 0.9
	} else if HasSourceType(base, SourceUserText) && HasSourceType(base, SourceUserImage) {
		confidence = 0.82
	} else if HasSourceType(base, SourceUserText) || HasSourceType(base, SourceUserImage) {
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

func ProductNeedsReview(product *Product) bool {
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

func ShouldReviewLLMOnlySourceFact(base []Source, inferred bool, fieldEvidence []Source) bool {
	return inferred &&
		HasSourceType(base, SourceProductURL) &&
		HasSourceType(base, SourceScrapedData) &&
		!HasSourceType(fieldEvidence, SourceScrapedData)
}
