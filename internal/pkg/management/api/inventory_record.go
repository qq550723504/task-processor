package api

import "time"

// InventoryRecordCreateReqDTO 库存记录创建请求DTO
type InventoryRecordCreateReqDTO struct {
	Platform           string   `json:"platform" binding:"required"`    // 平台
	ProductId          string   `json:"productId" binding:"required"`   // 产品ID或ASIN
	Region             string   `json:"region" binding:"required"`      // 区域
	Stock              *int     `json:"stock"`                          // 库存数量
	StockStatus        string   `json:"stockStatus"`                    // 库存状态文本
	IsAvailable        bool     `json:"isAvailable" binding:"required"` // 是否可用
	OriginalPrice      *float64 `json:"originalPrice"`                  // 原价
	CurrentPrice       *float64 `json:"currentPrice"`                   // 特价/当前价格
	Currency           string   `json:"currency"`                       // 货币单位
	PriceChangePercent *float64 `json:"priceChangePercent"`             // 价格变化百分比
	SyncSource         string   `json:"syncSource"`                     // 同步来源
	Remark             string   `json:"remark"`                         // 备注
}

// InventoryRecordRespDTO 库存记录响应DTO
type InventoryRecordRespDTO struct {
	ID                 int64     `json:"id"`                 // 主键ID
	Platform           string    `json:"platform"`           // 平台
	ProductId          string    `json:"productId"`          // 产品ID或ASIN
	Region             string    `json:"region"`             // 区域
	Stock              *int      `json:"stock"`              // 库存数量
	StockStatus        string    `json:"stockStatus"`        // 库存状态文本
	IsAvailable        bool      `json:"isAvailable"`        // 是否可用
	OriginalPrice      *float64  `json:"originalPrice"`      // 原价
	CurrentPrice       *float64  `json:"currentPrice"`       // 特价/当前价格
	Currency           string    `json:"currency"`           // 货币单位
	PriceChangePercent *float64  `json:"priceChangePercent"` // 价格变化百分比
	SyncSource         string    `json:"syncSource"`         // 同步来源
	Remark             string    `json:"remark"`             // 备注
	CreateTime         time.Time `json:"createTime"`         // 创建时间
}

// InventoryRecordAPI 库存记录API接口定义
type InventoryRecordAPI interface {
	// CreateInventoryRecord 创建库存记录
	CreateInventoryRecord(req *InventoryRecordCreateReqDTO) (int64, error)

	// GetLatestInventoryRecord 获取最新的库存记录
	GetLatestInventoryRecord(platform, productId, region string) (*InventoryRecordRespDTO, error)
}
