package api

import "task-processor/internal/pkg/types"

// RawJsonDataReqDTO 原始JSON数据请求DTO
type RawJsonDataReqDTO struct {
	TenantID   int64  `json:"tenantId" binding:"required"`
	Platform   string `json:"platform" binding:"required"`
	ProductID  string `json:"productId" binding:"required"`
	Region     string `json:"region" binding:"required"`
	StoreID    int64  `json:"storeId" binding:"required"`
	CategoryID int64  `json:"categoryId" binding:"required"`
	Creator    string `json:"creator" binding:"required"`
}

// RawJsonDataRespDTO 原始JSON数据响应DTO
type RawJsonDataRespDTO struct {
	ID          int64              `json:"id"`
	TaskID      int64              `json:"taskId"`
	Platform    string             `json:"platform"`
	ProductID   string             `json:"productId"`
	Region      string             `json:"region"`
	RawJSONData string             `json:"rawJsonData"`
	CreateTime  types.FlexibleTime `json:"createTime"`
	UpdateTime  types.FlexibleTime `json:"updateTime"`
}

// ProductVariantConfirmationReqDTO 产品变体确认请求DTO
type ProductVariantConfirmationReqDTO struct {
	ProductID  string   `json:"productId" binding:"required"`
	Platform   string   `json:"platform" binding:"required"`
	Region     string   `json:"region" binding:"required"`
	VariantIds []string `json:"variantIds" binding:"required"`
}

// RawJsonDataCreateReqDTO 原始JSON数据创建请求DTO
type RawJsonDataCreateReqDTO struct {
	TenantID     int64  `json:"tenantId"`
	StoreID      int64  `json:"storeId"`
	ImportTaskID int64  `json:"importTaskId"`
	Platform     string `json:"platform"`
	Region       string `json:"region"`
	ProductID    string `json:"productId"`
	CategoryID   int64  `json:"categoryId"`
	RawJsonData  string `json:"rawJsonData"`
	Creator      string `json:"creator"`
}

// RawJsonDataAPI 原始JSON数据API接口定义
type RawJsonDataAPI interface {
	GetRawJsonData(req *RawJsonDataReqDTO) (*RawJsonDataRespDTO, error)
	CreateRawJsonData(req *RawJsonDataCreateReqDTO) (int64, error)
}
