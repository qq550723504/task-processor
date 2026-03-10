// Package models 提供TEMU平台映射数据结构定义
package scheduler

import (
	"task-processor/internal/domain/model"
	managementapi "task-processor/internal/pkg/management/api"
)

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

// MonitorResult 监控结果
type MonitorResult struct {
	TotalProducts     int // 总产品数
	ProcessedProducts int // 已处理产品数
	SkippedProducts   int // 跳过的产品数
	PriceChanges      int // 价格变化数
	StockChanges      int // 库存变化数
	AmazonFetched     int // 成功获取Amazon数据数
	AmazonFailed      int // 获取Amazon数据失败数
}

// InventoryUpdateBatch 批量库存更新数据结构
type InventoryUpdateBatch struct {
	Product *managementapi.ProductDataDTO // 产品信息
	Updates []SkuInventoryUpdate          // SKU更新列表
	StoreID int64                         // 店铺ID
}

// SkuInventoryUpdate SKU库存更新信息
type SkuInventoryUpdate struct {
	PlatformSKU   string         // 平台SKU编号
	NewInventory  int64          // 新库存数量
	AmazonProduct *model.Product // Amazon产品数据
	SkuInfo       *TemuSkuInfo   // TEMU SKU信息
	StoreID       int64          // 店铺ID（用于获取价格类型）
}
