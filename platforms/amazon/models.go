// Package amazon 提供Amazon平台产品上架相关功能
package amazon

import (
	"time"
)

// AmazonProductResponse Amazon产品列表响应
type AmazonProductResponse struct {
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
	ASIN       string            `json:"asin"`
	SKU        string            `json:"sku"`
	Attributes map[string]string `json:"attributes"` // 如: {"Color": "Red", "Size": "Large"}
	Price      float64           `json:"price"`
	Quantity   int               `json:"quantity"`
	ImageURL   string            `json:"image_url"`
}

// ListingStatus 上架状态
type ListingStatus string

const (
	ListingStatusActive     ListingStatus = "Active"
	ListingStatusInactive   ListingStatus = "Inactive"
	ListingStatusIncomplete ListingStatus = "Incomplete"
	ListingStatusPending    ListingStatus = "Pending"
)

// FulfillmentType 配送类型
type FulfillmentType string

const (
	FulfillmentTypeFBA FulfillmentType = "FBA" // Fulfillment by Amazon
	FulfillmentTypeFBM FulfillmentType = "FBM" // Fulfillment by Merchant
)

// ProductCondition 产品状态
type ProductCondition string

const (
	ProductConditionNew         ProductCondition = "New"
	ProductConditionUsed        ProductCondition = "Used"
	ProductConditionRefurbished ProductCondition = "Refurbished"
)
