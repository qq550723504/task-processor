// Package types 提供产品相关的数据类型定义
package types

// FetchRequest 产品获取请求
type FetchRequest struct {
	TenantID   int64  `json:"tenant_id" validate:"required,min=1"`
	Platform   string `json:"platform" validate:"required,min=1"`
	Region     string `json:"region" validate:"required,min=1"`
	ProductID  string `json:"product_id" validate:"required,min=1"`
	StoreID    int64  `json:"store_id" validate:"min=0"`
	CategoryID int64  `json:"category_id" validate:"min=0"`
	Creator    string `json:"creator" validate:"required,min=1"`
}

// CacheRequest 缓存请求
type CacheRequest struct {
	*FetchRequest
	ForceUpdate bool `json:"force_update"`
}

// BatchFetchRequest 批量获取请求
type BatchFetchRequest struct {
	TenantID   int64    `json:"tenant_id" validate:"required,min=1"`
	Platform   string   `json:"platform" validate:"required,min=1"`
	Region     string   `json:"region" validate:"required,min=1"`
	ProductIDs []string `json:"product_ids" validate:"required,min=1,dive,required"`
	StoreID    int64    `json:"store_id" validate:"min=0"`
	CategoryID int64    `json:"category_id" validate:"min=0"`
	Creator    string   `json:"creator" validate:"required,min=1"`
}
