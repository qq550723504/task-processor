// Package shein 提供SHEIN平台库存相关的类型定义
package shein

// LocalInventoryInfo 本地库存信息（避免与其他包冲突）
type LocalInventoryInfo struct {
	SpuName            string              `json:"spu_name"`
	ProductNameCh      interface{}         `json:"product_name_ch"`
	MainImageThumbnail interface{}         `json:"main_image_thumbnail"`
	SkcInfo            []LocalSkcInventory `json:"skc_info"`
	IfFbmStore         bool                `json:"if_fbm_store"`
}

// LocalSkcInventory 本地SKC库存信息
type LocalSkcInventory struct {
	SkcName   string              `json:"skc_name"`
	SortOrder int                 `json:"sort_order"`
	SkcCode   string              `json:"skc_code"`
	SaleName  string              `json:"sale_name"`
	SkuInfo   []LocalSkuInventory `json:"sku_info"`
}

// LocalSkuInventory 本地SKU库存信息
type LocalSkuInventory struct {
	SkuCode       string                    `json:"sku_code"`
	SkuName       string                    `json:"sku_name"`
	InventoryInfo []LocalWarehouseInventory `json:"inventory_info"`
}

// LocalWarehouseInventory 本地仓库库存信息
type LocalWarehouseInventory struct {
	WarehouseCode     string `json:"warehouse_code"`
	UsableInventory   int    `json:"usable_inventory"`
	InventoryQuantity int    `json:"inventory_quantity"`
}
