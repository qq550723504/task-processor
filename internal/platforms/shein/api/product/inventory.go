package product

// InventoryQueryRequest 库存详情查询请求
type InventoryQueryRequest struct {
	SpuName string `json:"spu_name"`
}

// InventoryQueryResponse 库存详情查询响应
type InventoryQueryResponse struct {
	Code string        `json:"code"`
	Msg  string        `json:"msg"`
	Info InventoryInfo `json:"info"`
	BBL  any   `json:"bbl"`
}

// InventoryInfo 库存信息
type InventoryInfo struct {
	SpuName            string         `json:"spu_name"`
	ProductNameCh      any    `json:"product_name_ch"`
	MainImageThumbnail any    `json:"main_image_thumbnail"`
	SkcInfo            []SkcInventory `json:"skc_info"`
	IfFbmStore         bool           `json:"if_fbm_store"`
}

// SkcInventory SKC 库存信息
type SkcInventory struct {
	SkcName      string         `json:"skc_name"`
	SortOrder    int            `json:"sort_order"`
	SkcCode      string         `json:"skc_code"`
	SaleName     string         `json:"sale_name"`
	SaleAttrName string         `json:"sale_attr_name"`
	SkuInfo      []SkuInventory `json:"sku_info"`
}

// SkuInventory SKU 库存信息
type SkuInventory struct {
	SkuCode             string               `json:"sku_code"`
	DeliveryMode        int                  `json:"delivery_mode"`
	InventoryInfo       []WarehouseInventory `json:"inventory_info"`
	MallSellStatus      int                  `json:"mall_sell_status"`
	HasSellerPartnerSku bool                 `json:"has_seller_partner_sku"`
	SaleNameInfo        []SkuSaleNameInfo    `json:"sale_name_info"`
}

// WarehouseInventory 仓库库存信息
type WarehouseInventory struct {
	SupplierWarehouseID   int    `json:"supplier_warehouse_id"`
	MerchantWarehouseCode string `json:"merchant_warehouse_code"`
	MerchantWarehouseName string `json:"merchant_warehouse_name"`
	IsSaleable            bool   `json:"is_saleable"`
	Sort                  int    `json:"sort"`
	InventoryNum          int    `json:"inventory_num"`
	SupplierWarehouseName string `json:"supplier_warehouse_name"`
	WarehouseType         string `json:"warehouse_type"`
	InventoryQuantity     int    `json:"inventory_quantity"`
	OrderLockedQuantity   int    `json:"order_locked_quantity"`
	PayLockedQuantity     int    `json:"pay_locked_quantity"`
	UsableInventory       int    `json:"usable_inventory"`
}

// SkuSaleNameInfo SKU 销售名称信息
type SkuSaleNameInfo struct {
	SaleAttributeID      int    `json:"sale_attribute_id"`
	SaleAttributeValueID int    `json:"sale_attribute_value_id"`
	SaleAttrName         string `json:"sale_attr_name"`
	SaleName             string `json:"sale_name"`
}

// InventoryUpdateRequest 库存更新请求
type InventoryUpdateRequest struct {
	SkcInfo []SkcInventoryUpdate `json:"skc_info"`
}

// SkcInventoryUpdate SKC 库存更新信息
type SkcInventoryUpdate struct {
	SkcCode string               `json:"skc_code"`
	SkcName string               `json:"skc_name"`
	SkuInfo []SkuInventoryUpdate `json:"sku_info"`
}

// SkuInventoryUpdate SKU 库存更新信息
type SkuInventoryUpdate struct {
	SkuCode           string                     `json:"sku_code"`
	DeliveryMode      int                        `json:"delivery_mode"`
	WarehouseInfoList []WarehouseInventoryUpdate `json:"warehouse_info_list"`
}

// WarehouseInventoryUpdate 仓库库存更新信息
type WarehouseInventoryUpdate struct {
	MerchantWarehouseCode    string `json:"merchant_warehouse_code"`
	BeforeChangeInventoryNum int    `json:"before_change_inventory_num"`
	AfterChangeInventoryNum  int    `json:"after_change_inventory_num"`
}

// InventoryUpdateResponse 库存更新响应
type InventoryUpdateResponse struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Info any `json:"info"`
	BBL  any `json:"bbl"`
}
