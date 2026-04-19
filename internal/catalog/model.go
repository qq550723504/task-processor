package catalog

type Product struct {
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
	SKU        string      `json:"sku,omitempty"`
	Attributes []Attribute `json:"attributes,omitempty"`
	Price      *Price      `json:"price,omitempty"`
	Stock      int         `json:"stock,omitempty"`
	Images     []Image     `json:"images,omitempty"`
	Barcode    string      `json:"barcode,omitempty"`
	IsDefault  bool        `json:"is_default,omitempty"`
	Trace      Trace       `json:"trace,omitempty"`
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

type Trace struct {
	Sources     []SourceRecord `json:"sources,omitempty"`
	Confidence  float64        `json:"confidence,omitempty"`
	IsInferred  bool           `json:"is_inferred,omitempty"`
	NeedsReview bool           `json:"needs_review,omitempty"`
}

type SourceRecord struct {
	Type   string `json:"type,omitempty"`
	Detail string `json:"detail,omitempty"`
}
