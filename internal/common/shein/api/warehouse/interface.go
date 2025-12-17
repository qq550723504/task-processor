package warehouse

type WarehouseAPI interface {
	GetWarehouses() (*WarehouseResponse, error)
}

// Warehouse 仓库信息
type Warehouse struct {
	WarehouseName   string   `json:"warehouse_name"`
	WarehouseCode   string   `json:"warehouse_code"`
	SaleCountryList []string `json:"sale_country_list"`
	WarehouseType   int      `json:"warehouse_type"`
}

// WarehouseResponse 仓库信息响应
type WarehouseResponse struct {
	Data []Warehouse `json:"data"`
	Meta struct {
		Count     int         `json:"count"`
		CustomObj interface{} `json:"customObj"`
	} `json:"meta"`
}
