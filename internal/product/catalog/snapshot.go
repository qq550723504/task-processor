package catalog

import (
	"encoding/json"
	"sort"
	"time"
)

type ProductSnapshot struct {
	Title          string          `json:"title,omitempty"`
	Brand          string          `json:"brand,omitempty"`
	CategoryPath   []string        `json:"category_path,omitempty"`
	Description    string          `json:"description,omitempty"`
	SellingPoints  []string        `json:"selling_points,omitempty"`
	SEOKeywords    []string        `json:"seo_keywords,omitempty"`
	Attributes     []Attribute     `json:"attributes,omitempty"`
	Specifications *Specifications `json:"specifications,omitempty"`
	Variants       []Variant       `json:"variants,omitempty"`
	Images         []Image         `json:"images,omitempty"`
	Review         *ReviewState    `json:"review,omitempty"`
	Sources        []SourceRecord  `json:"sources,omitempty"`
	Warnings       []Warning       `json:"warnings,omitempty"`
}

func (p *ProductSnapshot) UnmarshalJSON(data []byte) error {
	type productSnapshotAlias ProductSnapshot
	var raw struct {
		*productSnapshotAlias
		Attributes json.RawMessage `json:"attributes,omitempty"`
	}
	raw.productSnapshotAlias = (*productSnapshotAlias)(p)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	attrs, err := unmarshalAttributeList(raw.Attributes)
	if err != nil {
		return err
	}
	p.Attributes = attrs
	return nil
}

type Attribute struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
	Trace Trace  `json:"trace,omitempty"`
}

type Image struct {
	URL   string `json:"url,omitempty"`
	Role  string `json:"role,omitempty"`
	Trace Trace  `json:"trace,omitempty"`
}

type Variant struct {
	SourceID   string      `json:"source_id,omitempty"`
	Title      string      `json:"title,omitempty"`
	SKU        string      `json:"sku,omitempty"`
	Attributes []Attribute `json:"attributes,omitempty"`
	Price      *Price      `json:"price,omitempty"`
	Stock      int         `json:"stock,omitempty"`
	Images     []Image     `json:"images,omitempty"`
	Barcode    string      `json:"barcode,omitempty"`
	IsDefault  bool        `json:"is_default,omitempty"`
	Trace      Trace       `json:"trace,omitempty"`
}

func (v *Variant) UnmarshalJSON(data []byte) error {
	type variantAlias Variant
	var raw struct {
		*variantAlias
		Attributes json.RawMessage `json:"attributes,omitempty"`
	}
	raw.variantAlias = (*variantAlias)(v)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	attrs, err := unmarshalAttributeList(raw.Attributes)
	if err != nil {
		return err
	}
	v.Attributes = attrs
	return nil
}

type Price struct {
	Currency     string  `json:"currency,omitempty"`
	Amount       float64 `json:"amount,omitempty"`
	CompareAt    float64 `json:"compare_at,omitempty"`
	CostPrice    float64 `json:"cost_price,omitempty"`
	WholesaleMin int     `json:"wholesale_min,omitempty"`
}

type Specifications struct {
	Dimensions *Dimensions       `json:"dimensions,omitempty"`
	Weight     *Weight           `json:"weight,omitempty"`
	Package    *PackageInfo      `json:"package,omitempty"`
	Technical  map[string]string `json:"technical,omitempty"`
}

type Dimensions struct {
	Length float64 `json:"length,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	Unit   string  `json:"unit,omitempty"`
}

type Weight struct {
	Value float64 `json:"value,omitempty"`
	Unit  string  `json:"unit,omitempty"`
}

type PackageInfo struct {
	Dimensions *Dimensions `json:"dimensions,omitempty"`
	Weight     *Weight     `json:"weight,omitempty"`
	Quantity   int         `json:"quantity,omitempty"`
}

type ReviewState struct {
	NeedsReview bool     `json:"needs_review"`
	Reasons     []string `json:"reasons,omitempty"`
}

// Warning preserves structured source validation information in the canonical
// snapshot instead of flattening it into free text alone.
type Warning struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type Trace struct {
	Sources     []SourceRecord `json:"sources,omitempty"`
	Confidence  float64        `json:"confidence,omitempty"`
	IsInferred  bool           `json:"is_inferred,omitempty"`
	NeedsReview bool           `json:"needs_review,omitempty"`
}

type SourceRecord struct {
	Type          string            `json:"type,omitempty"`
	Detail        string            `json:"detail,omitempty"`
	Platform      string            `json:"platform,omitempty"`
	SourceID      string            `json:"source_id,omitempty"`
	SourceVersion string            `json:"source_version,omitempty"`
	ReferenceType string            `json:"reference_type,omitempty"`
	URL           string            `json:"url,omitempty"`
	SnapshotID    string            `json:"snapshot_id,omitempty"`
	Checksum      string            `json:"checksum,omitempty"`
	CapturedAt    time.Time         `json:"captured_at,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	SourceRunID   string            `json:"source_run_id,omitempty"`
	RequestID     string            `json:"request_id,omitempty"`
	Notes         []string          `json:"notes,omitempty"`
}

func unmarshalAttributeList(data json.RawMessage) ([]Attribute, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var attrs []Attribute
	if err := json.Unmarshal(data, &attrs); err == nil {
		return attrs, nil
	}
	var keyed map[string]Attribute
	if err := json.Unmarshal(data, &keyed); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(keyed))
	for key := range keyed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	attrs = make([]Attribute, 0, len(keys))
	for _, key := range keys {
		attr := keyed[key]
		if attr.Name == "" {
			attr.Name = key
		}
		attrs = append(attrs, attr)
	}
	return attrs, nil
}
