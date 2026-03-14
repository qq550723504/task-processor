// Package api 活动产品数据API接口定义
package api

import "time"

// ActivityProductAPI 活动产品数据API接口定义
type ActivityProductAPI interface {
	// BatchSaveActivityProducts 批量保存可报名活动产品数据
	BatchSaveActivityProducts(products []*ActivityProductDTO) error
}

// ActivityProductDTO 活动产品数据传输对象
type ActivityProductDTO struct {
	SKC                 string                 `json:"skc"`
	GoodsName           string                 `json:"goods_name"`
	Image               string                 `json:"image"`
	SupplierNo          string                 `json:"supplier_no"`
	Stock               int                    `json:"stock"`
	SupplyPrice         float64                `json:"supply_price"`
	SupplyPriceCurrency string                 `json:"supply_price_currency"`
	IsConfigured        bool                   `json:"is_configured"`
	SitePriceInfoList   []ActivitySitePriceDTO `json:"site_price_info_list"`
	ActStock            int                    `json:"act_stock,omitempty"`
	DropRate            int                    `json:"drop_rate,omitempty"`
	ReservedActStock    int                    `json:"reserved_act_stock,omitempty"`
	State               int                    `json:"state,omitempty"`
	Platform            string                 `json:"platform"`
	TenantID            int64                  `json:"tenant_id"`
	StoreID             int64                  `json:"store_id"`
	Region              string                 `json:"region"`
	CostPrice           float64                `json:"cost_price,omitempty"`
	ProfitRate          float64                `json:"profit_rate,omitempty"`
}

// ActivitySitePriceDTO 活动站点价格信息
type ActivitySitePriceDTO struct {
	SiteCode    string  `json:"site_code"`
	SalePrice   float64 `json:"sale_price"`
	Currency    string  `json:"currency"`
	IsAvailable bool    `json:"is_available"`
}

// ActivityProductSyncRequest 活动产品同步请求
type ActivityProductSyncRequest struct {
	Platform         string               `json:"platform"`
	TenantID         int64                `json:"tenant_id"`
	Region           string               `json:"region"`
	StoreID          int64                `json:"store_id"`
	ActivityProducts []ActivityProductDTO `json:"activity_products"`
	SyncTime         int64                `json:"sync_time"`
}

// ActivityProductSyncResult 活动产品同步结果
type ActivityProductSyncResult struct {
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	TotalCount   int       `json:"total_count"`
	SuccessCount int       `json:"success_count"`
	FailedCount  int       `json:"failed_count"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
}
