// Package model 提供Amazon产品数据模型
package model

import (
	"time"
)

// AmazonProduct Amazon产品信息
type AmazonProduct struct {
	ASIN            string      `json:"asin"`
	SKU             string      `json:"sku"`
	Title           string      `json:"title"`
	Brand           string      `json:"brand"`
	Price           float64     `json:"price"`
	Currency        string      `json:"currency"`
	Quantity        int         `json:"quantity"`
	Condition       string      `json:"condition"`        // New, Used, Refurbished
	Status          string      `json:"status"`           // Active, Inactive, Incomplete
	FulfillmentType string      `json:"fulfillment_type"` // FBA, FBM
	CategoryID      string      `json:"category_id"`
	CategoryName    string      `json:"category_name"`
	ImageURL        string      `json:"image_url"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	ListingID       string      `json:"listing_id"`
	ProductType     string      `json:"product_type"`
	Variations      []Variation `json:"variations,omitempty"`
}

// Variation 变体信息
type Variation struct {
	SKU        string            `json:"sku"`
	ASIN       string            `json:"asin,omitempty"`
	Attributes map[string]string `json:"attributes"` // 变体属性 (color, size等)
	Price      float64           `json:"price"`
	Quantity   int               `json:"quantity"`
	ImageURL   string            `json:"image_url"`
	Status     string            `json:"status"`
}

// ProductCondition 产品状态常量
const (
	ConditionNew         = "New"
	ConditionUsed        = "Used"
	ConditionRefurbished = "Refurbished"
	ConditionCollectible = "Collectible"
)

// ProductStatus 产品状态常量
const (
	StatusActive     = "Active"
	StatusInactive   = "Inactive"
	StatusIncomplete = "Incomplete"
	StatusSuppressed = "Suppressed"
)

// FulfillmentType 配送类型常量
const (
	FulfillmentFBA = "FBA" // Fulfillment by Amazon
	FulfillmentFBM = "FBM" // Fulfillment by Merchant
)

// ProductListRequest 产品列表请求
type ProductListRequest struct {
	MarketplaceID string   `json:"marketplace_id"`
	SKUs          []string `json:"skus,omitempty"`
	ASINs         []string `json:"asins,omitempty"`
	Status        string   `json:"status,omitempty"`
	PageSize      int      `json:"page_size,omitempty"`
	NextToken     string   `json:"next_token,omitempty"`
}

// ProductListResponse 产品列表响应
type ProductListResponse struct {
	Products  []AmazonProduct `json:"products"`
	NextToken string          `json:"next_token,omitempty"`
	Total     int             `json:"total"`
}

// ProductCreateRequest 产品创建请求
type ProductCreateRequest struct {
	SKU          string                 `json:"sku"`
	ProductType  string                 `json:"product_type"`
	Attributes   map[string]interface{} `json:"attributes"`
	Requirements string                 `json:"requirements,omitempty"`
}

// ProductUpdateRequest 产品更新请求
type ProductUpdateRequest struct {
	SKU        string                 `json:"sku"`
	Attributes map[string]interface{} `json:"attributes"`
	PatchOp    string                 `json:"patch_op,omitempty"` // REPLACE, DELETE
}

// ProductSearchRequest 产品搜索请求
type ProductSearchRequest struct {
	Keywords      []string `json:"keywords"`
	MarketplaceID string   `json:"marketplace_id"`
	ProductType   string   `json:"product_type,omitempty"`
	Brand         string   `json:"brand,omitempty"`
	CategoryID    string   `json:"category_id,omitempty"`
	MinPrice      float64  `json:"min_price,omitempty"`
	MaxPrice      float64  `json:"max_price,omitempty"`
	PageSize      int      `json:"page_size,omitempty"`
	PageToken     string   `json:"page_token,omitempty"`
}

// ProductSearchResponse 产品搜索响应
type ProductSearchResponse struct {
	Products      []AmazonProduct `json:"products"`
	NextPageToken string          `json:"next_page_token,omitempty"`
	TotalCount    int             `json:"total_count"`
}

// IsValidCondition 检查产品状态是否有效
func IsValidCondition(condition string) bool {
	validConditions := []string{
		ConditionNew,
		ConditionUsed,
		ConditionRefurbished,
		ConditionCollectible,
	}

	for _, valid := range validConditions {
		if condition == valid {
			return true
		}
	}
	return false
}

// IsValidStatus 检查产品状态是否有效
func IsValidStatus(status string) bool {
	validStatuses := []string{
		StatusActive,
		StatusInactive,
		StatusIncomplete,
		StatusSuppressed,
	}

	for _, valid := range validStatuses {
		if status == valid {
			return true
		}
	}
	return false
}

// IsValidFulfillmentType 检查配送类型是否有效
func IsValidFulfillmentType(fulfillmentType string) bool {
	return fulfillmentType == FulfillmentFBA || fulfillmentType == FulfillmentFBM
}
