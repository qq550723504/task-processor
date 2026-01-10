package model

import (
	"encoding/json"
	"fmt"
	"time"
)

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

// Description 产品描述信息
type Description struct {
	Text string `json:"text"`
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

// PriceBreakdown 价格明细
type PriceBreakdown struct {
	TypicalPrice *float64 `json:"typical_price,omitempty"`
	ListPrice    *float64 `json:"list_price,omitempty"`
	DealType     *string  `json:"deal_type,omitempty"`
}

// BuyboxPrices 购买框价格信息
type BuyboxPrices struct {
	FinalPrice float64 `json:"final_price"`
	UnitPrice  *string `json:"unit_price,omitempty"`
}

// Subcategory 子分类排名信息
type Subcategory struct {
	SubcategoryName string `json:"subcategory_name"`
	SubcategoryRank int    `json:"subcategory_rank"`
}

// CustomersSay 客户评价信息
type CustomersSay struct {
	Text     *string           `json:"text,omitempty"`
	Keywords CustomersKeywords `json:"keywords"`
}

// CustomersKeywords 客户评价关键词
type CustomersKeywords struct {
	Positive *[]string `json:"positive,omitempty"`
	Negative *[]string `json:"negative,omitempty"`
	Mixed    *[]string `json:"mixed,omitempty"`
}

// InactiveBuyBox 非活跃购买框信息
type InactiveBuyBox struct {
	Price *float64 `json:"price,omitempty"`
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

// VariationValue 变体值（维度名称及其可选值）
type VariationValue struct {
	VariantName string   `json:"variant_name"` // 维度名称（如 Color, Size）
	Values      []string `json:"values"`       // 该维度的所有可选值
}

// UnmarshalJSON 自定义JSON解析，支持多种字段名格式
func (vv *VariationValue) UnmarshalJSON(data []byte) error {
	// 定义一个临时结构体，包含所有可能的字段名
	type Alias VariationValue
	aux := &struct {
		VariantNameWithSpace string `json:"variant name"` // 支持带空格的字段名
		*Alias
	}{
		Alias: (*Alias)(vv),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 如果 variant_name 为空但 variant name 有值，使用后者
	if vv.VariantName == "" && aux.VariantNameWithSpace != "" {
		vv.VariantName = aux.VariantNameWithSpace
	}

	return nil
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

// ProductNotFoundError 产品不存在错误（不应触发浏览器重建）
type ProductNotFoundError struct {
	ProductID string
	Message   string
	Cause     error
}

func (e *ProductNotFoundError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("产品不存在: %s (ProductID: %s): %v", e.Message, e.ProductID, e.Cause)
	}
	return fmt.Sprintf("产品不存在: %s (ProductID: %s)", e.Message, e.ProductID)
}

func (e *ProductNotFoundError) Unwrap() error {
	return e.Cause
}

// NullableTime 可空时间类型，支持空字符串解析为nil
type NullableTime struct {
	*time.Time
}

// UnmarshalJSON 自定义JSON解析，将空字符串解析为nil
func (nt *NullableTime) UnmarshalJSON(data []byte) error {
	// 去除引号
	str := string(data)
	if str == "null" || str == `""` || str == "" {
		nt.Time = nil
		return nil
	}

	// 解析时间
	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	nt.Time = &t
	return nil
}

// MarshalJSON 自定义JSON序列化
func (nt NullableTime) MarshalJSON() ([]byte, error) {
	if nt.Time == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*nt.Time)
}
