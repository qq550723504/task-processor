package amazon

import "time"

// Product Amazon产品结构
type Product struct {
	Title              string           `json:"title"`
	Brand              string           `json:"brand"`
	Description        string           `json:"description"`
	ProductDescription []Description    `json:"product_description,omitempty"`
	InitialPrice       float64          `json:"initial_price"`
	FinalPrice         float64          `json:"final_price"`
	Currency           string           `json:"currency"` // Currency based on region (USD, EUR, GBP, JPY, etc.)
	Availability       string           `json:"availability"`
	IsAvailable        bool             `json:"is_available"`
	ReviewsCount       int              `json:"reviews_count"`
	Rating             float64          `json:"rating"`
	Categories         []string         `json:"categories"`
	ParentAsin         string           `json:"parent_asin"`
	Asin               string           `json:"asin"`
	SellerName         string           `json:"seller_name"`
	SellerID           string           `json:"seller_id"`
	URL                string           `json:"url"`
	ImageURL           string           `json:"image_url"`
	Images             []string         `json:"images"`
	ImagesCount        int              `json:"images_count"`
	Features           []string         `json:"features"`
	Variations         []Variation      `json:"variations"`
	VariationsValues   []VariationValue `json:"variations_values"`
	ProductDetails     []ProductDetail  `json:"product_details"`
	PricesBreakdown    PriceBreakdown   `json:"prices_breakdown"`
	Zipcode            string           `json:"zipcode"`
	Timestamp          time.Time        `json:"timestamp"`
	// 畅销排名相关
	BsRank         int    `json:"bs_rank,omitempty"`
	BsCategory     string `json:"bs_category,omitempty"`
	RootBsRank     int    `json:"root_bs_rank,omitempty"`
	RootBsCategory string `json:"root_bs_category,omitempty"`
	// 产品详细信息
	ProductDimensions  string `json:"product_dimensions,omitempty"`
	ItemWeight         string `json:"item_weight,omitempty"`
	ModelNumber        string `json:"model_number,omitempty"`
	Department         string `json:"department,omitempty"`
	DateFirstAvailable string `json:"date_first_available,omitempty"`
	Manufacturer       string `json:"manufacturer,omitempty"`
	Domain             string `json:"domain,omitempty"`
}

// Description 产品描述信息
type Description struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

// PriceBreakdown 价格明细
type PriceBreakdown struct {
	TypicalPrice *float64 `json:"typical_price,omitempty"`
	ListPrice    *float64 `json:"list_price,omitempty"`
	DealType     *string  `json:"deal_type,omitempty"`
}

// Variation 变体信息
type Variation struct {
	Name       string                 `json:"name"`
	Asin       string                 `json:"asin"`
	Price      float64                `json:"price"`
	Currency   string                 `json:"currency"`
	Image      string                 `json:"image,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// VariationValue 变体值
type VariationValue struct {
	VariantName string   `json:"variant_name"`
	Values      []string `json:"values"`
}

// ProductDetail 产品详情
type ProductDetail struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ProductRequest 产品请求
type ProductRequest struct {
	URL     string
	Zipcode string
}

// ProductResult 产品结果
type ProductResult struct {
	Product *Product
	Error   error
}
