// Package productsync 提供SHEIN平台产品同步相关服务的类型定义
package productsync

import (
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein"
	"task-processor/internal/shein/api/product"
)

// EnrichedSkuInfo 增强的SKU数据结构（用于序列化到Attributes）
type EnrichedSkuInfo struct {
	product.SkuInfo
	MappingInfo       *api.ProductImportMappingRespDTO `json:"mapping_info,omitempty"`        //管理系统映射
	SaleNameInfo      []product.SaleNameInfo           `json:"sale_name_info,omitempty"`      // 自营店铺：销售属性
	PriceInfoList     []product.SkuPriceDetail         `json:"price_info_list,omitempty"`     // 自营店铺：价格列表
	SaleAttributeList []product.SaleAttributeItem      `json:"sale_attribute_list,omitempty"` // 半托店铺：销售属性
	CostPriceInfo     *product.CostPrice               `json:"cost_price_info,omitempty"`     // 半托店铺：成本价
	InventoryInfo     []product.WarehouseInventory     `json:"inventory_info,omitempty"`      // SKU 库存信息
	UsableInventory   *int                             `json:"usable_inventory,omitempty"`    // 可用库存汇总
	InventoryQuantity *int                             `json:"inventory_quantity,omitempty"`  // 总库存汇总
	AmazonMonitorData *shein.AmazonMonitorData         `json:"amazon_monitor_data,omitempty"` // Amazon监控数据
}

// EnrichedSkcInfo 增强的SKC数据结构（用于序列化到Attributes）
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
	TagInfoList           []any             `json:"tag_info_list"`
	ShelfFailReason       *string           `json:"shelf_fail_reason"`
	HasOriginalImage      bool              `json:"has_original_image"`
}
