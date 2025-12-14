// Package model 提供数据结构定义
package model

// Product Amazon产品结构（完整版）
type Product struct {
	// 基本信息
	Title              string        `json:"title"`
	Brand              string        `json:"brand"`
	Description        string        `json:"description"`
	ProductDescription []Description `json:"product_description,omitempty"`

	// 价格信息
	InitialPrice    float64        `json:"initial_price"`
	FinalPrice      float64        `json:"final_price"`
	FinalPriceHigh  *float64       `json:"final_price_high,omitempty"`
	Currency        string         `json:"currency"` // Currency based on region (USD, EUR, GBP, JPY, etc.)
	PricesBreakdown PriceBreakdown `json:"prices_breakdown"`
	BuyboxPrices    *BuyboxPrices  `json:"buybox_prices,omitempty"`

	// 库存和可用性
	Availability         string `json:"availability"`
	IsAvailable          bool   `json:"is_available"`
	MaxQuantityAvailable int    `json:"max_quantity_available,omitempty"`
	BoughtPastMonth      int    `json:"bought_past_month,omitempty"`

	// 评价信息
	ReviewsCount int           `json:"reviews_count"`
	Rating       float64       `json:"rating"`
	TopReview    string        `json:"top_review,omitempty"`
	CustomersSay *CustomersSay `json:"customers_say,omitempty"`

	// 分类和排名
	Categories      []string      `json:"categories"`
	BsRank          int           `json:"bs_rank,omitempty"`
	BsCategory      string        `json:"bs_category,omitempty"`
	RootBsRank      int           `json:"root_bs_rank,omitempty"`
	RootBsCategory  string        `json:"root_bs_category,omitempty"`
	SubcategoryRank []Subcategory `json:"subcategory_rank,omitempty"`

	// ASIN和标识
	ParentAsin string `json:"parent_asin"`
	Asin       string `json:"asin"`

	// 卖家信息
	SellerName      string `json:"seller_name"`
	SellerID        string `json:"seller_id"`
	SellerURL       string `json:"seller_url,omitempty"`
	BuyboxSeller    string `json:"buybox_seller,omitempty"`
	NumberOfSellers int    `json:"number_of_sellers,omitempty"`

	// URL和图片
	URL         string   `json:"url"`
	ImageURL    string   `json:"image_url"`
	Images      []string `json:"images"`
	ImagesCount int      `json:"images_count"`

	// 视频
	Videos             []string `json:"videos,omitempty"`
	VideoCount         int      `json:"video_count,omitempty"`
	Video              bool     `json:"video,omitempty"`
	DownloadableVideos []string `json:"downloadable_videos,omitempty"`

	// 产品特性
	Features         []string         `json:"features"`
	Variations       []Variation      `json:"variations"`
	VariationsValues []VariationValue `json:"variations_values"`
	ProductDetails   []ProductDetail  `json:"product_details"`

	// 产品详细信息
	ProductDimensions  string `json:"product_dimensions,omitempty"`
	ItemWeight         string `json:"item_weight,omitempty"`
	ModelNumber        string `json:"model_number,omitempty"`
	Department         string `json:"department,omitempty"`
	DateFirstAvailable string `json:"date_first_available,omitempty"`
	Manufacturer       string `json:"manufacturer,omitempty"`
	CountryOfOrigin    string `json:"country_of_origin,omitempty"`

	// 配送信息
	Delivery  []string `json:"delivery,omitempty"`
	ShipsFrom string   `json:"ships_from,omitempty"`

	// 标记和徽章
	Badge                 *string `json:"badge,omitempty"`
	AmazonChoice          bool    `json:"amazon_choice,omitempty"`
	PlusContent           bool    `json:"plus_content,omitempty"`
	ClimatePledgeFriendly bool    `json:"climate_pledge_friendly,omitempty"`
	Sponsored             bool    `json:"sponsored,omitempty"`

	// 其他信息
	Domain            string          `json:"domain,omitempty"`
	Zipcode           string          `json:"zipcode"`
	Timestamp         NullableTime    `json:"timestamp,omitempty"` // 使用NullableTime支持空字符串
	AnsweredQuestions int             `json:"answered_questions,omitempty"`
	StoreURL          string          `json:"store_url,omitempty"`
	ReturnPolicy      *string         `json:"return_policy,omitempty"`
	InactiveBuyBox    *InactiveBuyBox `json:"inactive_buy_box,omitempty"`

	// 额外内容
	FromTheBrand           interface{}   `json:"from_the_brand,omitempty"`
	SustainabilityFeatures []interface{} `json:"sustainability_features,omitempty"`
	OtherSellersPrices     interface{}   `json:"other_sellers_prices,omitempty"`
	EditorialReviews       *string       `json:"editorial_reviews,omitempty"`
	AboutTheAuthor         *string       `json:"about_the_author,omitempty"`
}

// NewProduct 创建新的产品实例
func NewProduct(asin string) *Product {
	return &Product{
		Asin: asin,
	}
}
