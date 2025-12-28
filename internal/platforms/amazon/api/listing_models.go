// Package api 提供Amazon API的数据结构定义
package api

// ListingRequest 创建listing请求
type ListingRequest struct {
	SKU           string         `json:"-"` // SKU在URL路径中，不在请求体中
	ProductType   string         `json:"productType"`
	Requirements  string         `json:"requirements"`
	Attributes    map[string]any `json:"attributes"`
	MarketplaceID string         `json:"-"` // MarketplaceID在查询参数中，不在请求体中
}

// ListingResponse listing响应
type ListingResponse struct {
	SKU    string `json:"sku"`
	Status string `json:"status"`
	Issues []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Severity string `json:"severity"`
	} `json:"issues,omitempty"`
}

// PostmanListingRequest 基于Amazon官方Postman集合的listing请求格式
type PostmanListingRequest struct {
	SKU           string         `json:"-"` // SKU在URL路径中
	MarketplaceID string         `json:"-"` // MarketplaceID在查询参数中
	ProductType   string         `json:"productType"`
	Requirements  string         `json:"requirements,omitempty"`
	Attributes    map[string]any `json:"attributes"`
}
