// Package product 提供产品领域类型定义
package product

// RawJsonReq 获取原始JSON数据请求（domain 层自有类型）
type RawJsonReq struct {
	TenantID   int64
	Platform   string
	ProductID  string
	Region     string
	StoreID    int64
	CategoryID int64
	Creator    string
}

// RawJsonResp 获取原始JSON数据响应（domain 层自有类型）
type RawJsonResp struct {
	ID          int64
	Platform    string
	ProductID   string
	Region      string
	RawJSONData string
	CreateTime  int64
	UpdateTime  int64
}

// RawJsonCreateReq 创建原始JSON数据请求（domain 层自有类型）
type RawJsonCreateReq struct {
	TenantID     int64
	StoreID      int64
	ImportTaskID int64
	Platform     string
	Region       string
	ProductID    string
	CategoryID   int64
	RawJsonData  string
	Creator      string
}

// RawJsonDataClient 原始JSON数据客户端接口（使用 domain 自有类型）
type RawJsonDataClient interface {
	GetRawJsonData(req *RawJsonReq) (*RawJsonResp, error)
	CreateRawJsonData(req *RawJsonCreateReq) (int64, error)
}

// FetchRequest 获取请求
type FetchRequest struct {
	TenantID   int64  `json:"tenant_id" validate:"required,min=1"`
	Platform   string `json:"platform" validate:"required,min=1"`
	Region     string `json:"region" validate:"required,min=1"`
	ProductID  string `json:"product_id" validate:"required,min=1"`
	Zipcode    string `json:"zipcode,omitempty"`
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
