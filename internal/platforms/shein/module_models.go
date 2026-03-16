// Package shein 提供SHEIN平台模块的数据模型定义
package shein

import (
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/platforms/shein/api/product"
)

// ImageUploadResult 图片上传结果结构体
// 用于图片处理器中的并行上传结果收集
type ImageUploadResult struct {
	Index     int    `json:"index"`
	URL       string `json:"url"`
	Err       error  `json:"error,omitempty"`
	IsMain    bool   `json:"is_main"`
	IsColor   bool   `json:"is_color"`
	ColorData []byte `json:"color_data,omitempty"`
}

// AttributeImportance 属性重要性结构体
// 用于SKC属性策略处理器中的销售属性重要性分析
type AttributeImportance struct {
	AttrID     int `json:"attr_id"`
	Importance int `json:"importance"` // 重要性评分，数值越高越重要
}

// EnrichedSkuInfo 增强的SKU数据结构
// 用于同步服务中的SKU信息增强
type EnrichedSkuInfo struct {
	product.SkuInfo
	MappingInfo       *api.ProductImportMappingRespDTO `json:"mapping_info,omitempty"`
	SaleNameInfo      []product.SaleNameInfo           `json:"sale_name_info,omitempty"`      // 自营店铺：销售属性
	PriceInfoList     []product.SkuPriceDetail         `json:"price_info_list,omitempty"`     // 自营店铺：价格列表
	SaleAttributeList []product.SaleAttributeItem      `json:"sale_attribute_list,omitempty"` // 半托店铺：销售属性
	CostPriceInfo     *product.CostPrice               `json:"cost_price_info,omitempty"`     // 半托店铺：成本价
	InventoryInfo     []product.WarehouseInventory     `json:"inventory_info,omitempty"`      // SKU 库存信息
	UsableInventory   *int                             `json:"usable_inventory,omitempty"`    // 可用库存汇总
	InventoryQuantity *int                             `json:"inventory_quantity,omitempty"`  // 总库存汇总
}

// EnrichedSkcInfo 增强的SKC数据结构
// 用于同步服务中的SKC信息增强
type EnrichedSkcInfo struct {
	SkcName               string            `json:"skc_name"`
	SkcCode               string            `json:"skc_code"`
	SaleName              string            `json:"sale_name"`
	MainImageThumbnailURL string            `json:"main_image_thumbnail_url"`
	SupplierCode          string            `json:"supplier_code"`
	BusinessModel         int               `json:"business_model"`
	IsSaleAttribute       int               `json:"is_sale_attribute"`
	SupplierID            int64             `json:"supplier_id"`
	SkuInfo               []EnrichedSkuInfo `json:"sku_info"`
	MallSellStatus        int               `json:"mall_sell_status"`
	Abandoned             bool              `json:"abandoned"`
	TagInfoList           []any     `json:"tag_info_list"`
	ShelfFailReason       *string           `json:"shelf_fail_reason"`
	HasOriginalImage      bool              `json:"has_original_image"`
}

// InventoryInfo 库存信息结构（从monitor_helper.go移动过来）
type InventoryInfo struct {
	InventoryNum    int `json:"inventory_num"`
	UsableInventory int `json:"usable_inventory"`
}

// AmazonMonitorData Amazon监控数据结构（从monitor_helper.go移动过来）
type AmazonMonitorData struct {
	ASIN     string  `json:"asin"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	InStock  bool    `json:"in_stock"`
}

// SkuInfo SKU信息结构（从monitor_helper.go移动过来）
type SkuInfo struct {
	SKUCode           string             `json:"sku_code"`
	MappingInfo       *MappingInfo       `json:"mapping_info"`
	InventoryInfo     *InventoryInfo     `json:"inventory_info"`
	AmazonMonitorData *AmazonMonitorData `json:"amazon_monitor_data"`
	CostPriceInfo     *CostPriceInfo     `json:"cost_price_info"`
}

// MappingInfo 映射信息结构（从monitor_helper.go移动过来）
type MappingInfo struct {
	ID       int64   `json:"id"`
	SKU      string  `json:"sku"`
	ASIN     string  `json:"asin"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

// CostPriceInfo 成本价格信息（从monitor_helper.go移动过来）
type CostPriceInfo struct {
	CostPrice string `json:"cost_price"`
	Currency  string `json:"currency"`
}

