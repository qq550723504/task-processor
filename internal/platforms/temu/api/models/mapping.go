// Package models 提供TEMU平台映射数据结构定义
package models

// TemuMappingData TEMU映射到管理系统的数据结构（产品级别对象，包含多个SKU）
type TemuMappingData struct {
	SkcCode               string        `json:"skc_code"`                 // SKC编码（商品编码）
	SkcName               string        `json:"skc_name"`                 // SKC名称（商品名称）
	SkuInfo               []TemuSkuInfo `json:"sku_info"`                 // SKU信息列表
	SaleName              string        `json:"sale_name"`                // 销售名称
	SupplierID            int64         `json:"supplier_id"`              // 供应商ID
	SupplierCode          string        `json:"supplier_code"`            // 供应商编码
	MallSellStatus        int           `json:"mall_sell_status"`         // 商城销售状态
	MainImageThumbnailURL string        `json:"main_image_thumbnail_url"` // 主图缩略图URL
}

// TemuSkuInfo TEMU SKU信息（对应SHEIN格式中的单个SKU对象）
type TemuSkuInfo struct {
	SkuCode           string                 `json:"sku_code"`        // SKU编码
	MappingInfo       TemuMappingInfo        `json:"mapping_info"`    // 映射信息
	CostPriceInfo     TemuCostPriceInfo      `json:"cost_price_info"` // 成本价格信息
	UsableInventory   int                    `json:"usable_inventory"`
	AmazonMonitorData *TemuAmazonMonitorData `json:"amazon_monitor_data"` // Amazon监控数据
}

type TemuMappingInfo struct {
	ID                      int64    `json:"id"`                      // 映射ID
	ImportTaskId            int64    `json:"importTaskId"`            // 导入任务ID
	StoreId                 int64    `json:"storeId"`                 // 店铺ID
	Platform                string   `json:"platform"`                // 平台类型
	Region                  string   `json:"region"`                  // 区域
	ProductId               string   `json:"productId"`               // 产品ID（ASIN）
	ParentProductId         *string  `json:"parentProductId"`         // 父产品ID
	PlatformProductId       *string  `json:"platformProductId"`       // 平台产品ID（SKU编码）
	PlatformParentProductId *string  `json:"platformParentProductId"` // 平台父产品ID（SPU编码）
	Sku                     *string  `json:"sku"`                     // SKU编号
	CostPrice               *float64 `json:"costPrice"`               // 成本价
	FilterRuleId            *int64   `json:"filterRuleId"`            // 过滤规则ID
	FilterRuleRange         *string  `json:"filterRuleRange"`         // 过滤规则范围
	ProfitRuleId            *int64   `json:"profitRuleId"`            // 利润规则ID
	SalePriceMultiplier     *float64 `json:"salePriceMultiplier"`     // 售价倍数
	DiscountPriceMultiplier *float64 `json:"discountPriceMultiplier"` // 折扣价倍数
	Status                  int16    `json:"status"`                  // 映射状态
	Remark                  *string  `json:"remark"`                  // 备注
	TenantId                int64    `json:"tenantId"`                // 租户ID
}

// TemuInventoryInfo TEMU库存信息
type TemuInventoryInfo struct {
	UsableInventory int `json:"usable_inventory"` // 库存数量
}

// TemuCostPriceInfo TEMU成本价格信息
type TemuCostPriceInfo struct {
	Currency  string `json:"currency"`   // 货币
	CostPrice string `json:"cost_price"` // 成本价格
}

// TemuAmazonMonitorData TEMU Amazon监控数据（对应SHEIN格式中的amazon_monitor_data）
type TemuAmazonMonitorData struct {
	ASIN          string  `json:"asin"`            // Amazon ASIN
	Price         float64 `json:"price"`           // 价格
	Stock         int     `json:"stock"`           // 库存
	LastCheckTime int64   `json:"last_check_time"` // 最后检查时间
}
