package api

import "task-processor/internal/pkg/types"

// InventoryRecordCreateReqDTO 库存记录创建请求DTO
type InventoryRecordCreateReqDTO struct {
	Platform           string   `json:"platform" binding:"required"`
	ProductId          string   `json:"productId" binding:"required"`
	Region             string   `json:"region" binding:"required"`
	Stock              *int     `json:"stock"`
	StockStatus        string   `json:"stockStatus"`
	IsAvailable        bool     `json:"isAvailable" binding:"required"`
	OriginalPrice      *float64 `json:"originalPrice"`
	CurrentPrice       *float64 `json:"currentPrice"`
	Currency           string   `json:"currency"`
	PriceChangePercent *float64 `json:"priceChangePercent"`
	SyncSource         string   `json:"syncSource"`
	Remark             string   `json:"remark"`
}

// InventoryRecordRespDTO 库存记录响应DTO
type InventoryRecordRespDTO struct {
	ID                 int64              `json:"id"`
	Platform           string             `json:"platform"`
	ProductId          string             `json:"productId"`
	Region             string             `json:"region"`
	Stock              *int               `json:"stock"`
	StockStatus        string             `json:"stockStatus"`
	IsAvailable        bool               `json:"isAvailable"`
	OriginalPrice      *float64           `json:"originalPrice"`
	CurrentPrice       *float64           `json:"currentPrice"`
	Currency           string             `json:"currency"`
	PriceChangePercent *float64           `json:"priceChangePercent"`
	SyncSource         string             `json:"syncSource"`
	Remark             string             `json:"remark"`
	CreateTime         types.FlexibleTime `json:"createTime"`
}

// InventoryRecordAPI 库存记录API接口定义
type InventoryRecordAPI interface {
	CreateInventoryRecord(req *InventoryRecordCreateReqDTO) (int64, error)
	GetLatestInventoryRecord(platform, productId, region string) (*InventoryRecordRespDTO, error)
}
