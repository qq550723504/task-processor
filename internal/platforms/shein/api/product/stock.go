package product

// StockQueryRequest 库存查询请求
type StockQueryRequest struct {
	SpuSkcList []SpuSkcItem `json:"spu_skc_list"`
}

// SpuSkcItem SPU-SKC 项
type SpuSkcItem struct {
	SpuName     string        `json:"spu_name"`
	SkcNameList []SkcNameItem `json:"skc_name_list"`
}

// SkcNameItem SKC 名称项
type SkcNameItem struct {
	SkcName     string   `json:"skc_name"`
	SkuCodeList []string `json:"sku_code_list"`
}

// StockQueryResponse 库存查询响应
type StockQueryResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data []StockData `json:"data"`
		Meta struct {
			Count     int         `json:"count"`
			CustomObj any `json:"customObj"`
		} `json:"meta"`
	} `json:"info"`
	BBL any `json:"bbl"`
}

// StockData 库存数据
type StockData struct {
	SpuName             string `json:"spu_name"`
	OrderLockedQuantity int    `json:"order_locked_quantity"`
	PayLockedQuantity   int    `json:"pay_locked_quantity"`
	UsableInventory     int    `json:"usable_inventory"`
	InventoryQuantity   int    `json:"inventory_quantity"`
}
