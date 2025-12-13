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
	// SKC编码
	SKC string `json:"skc"`
	// 商品名称
	GoodsName string `json:"goods_name"`
	// 商品图片URL
	Image string `json:"image"`
	// 供应商编号
	SupplierNo string `json:"supplier_no"`
	// 库存数量
	Stock int `json:"stock"`
	// 供货价格
	SupplyPrice float64 `json:"supply_price"`
	// 供货价格币种
	SupplyPriceCurrency string `json:"supply_price_currency"`
	// 是否已配置
	IsConfigured bool `json:"is_configured"`
	// 站点价格信息列表
	SitePriceInfoList []ActivitySitePriceDTO `json:"site_price_info_list"`
	// 活动库存
	ActStock int `json:"act_stock,omitempty"`
	// 降价幅度（百分比）
	DropRate int `json:"drop_rate,omitempty"`
	// 预留活动库存
	ReservedActStock int `json:"reserved_act_stock,omitempty"`
	// 状态（1:正常）
	State int `json:"state,omitempty"`

	// 扩展字段
	Platform string `json:"platform"`
	TenantID int64  `json:"tenant_id"`
	StoreID  int64  `json:"store_id"`
	Region   string `json:"region"`

	// 成本和利润相关字段
	CostPrice  float64 `json:"cost_price,omitempty"`
	ProfitRate float64 `json:"profit_rate,omitempty"`
}

// ActivitySitePriceDTO 活动站点价格信息
type ActivitySitePriceDTO struct {
	// 站点代码
	SiteCode string `json:"site_code"`
	// 销售价格
	SalePrice float64 `json:"sale_price"`
	// 币种
	Currency string `json:"currency"`
	// 是否可用
	IsAvailable bool `json:"is_available"`
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
