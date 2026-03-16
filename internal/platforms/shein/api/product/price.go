package product

// PriceQueryRequest 价格查询请求
type PriceQueryRequest struct {
	SpuName string `json:"spu_name"`
}

// PriceQueryResponse 价格查询响应
type PriceQueryResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data []SkcPriceData `json:"data"`
		Meta struct {
			Count     int         `json:"count"`
			CustomObj any `json:"customObj"`
		} `json:"meta"`
	} `json:"info"`
	BBL any `json:"bbl"`
}

// SkcPriceData SKC 价格数据
type SkcPriceData struct {
	SkcName      string         `json:"skc_name"`
	SortOrder    int            `json:"sort_order"`
	SaleAttrName string         `json:"sale_attr_name"`
	SaleName     string         `json:"sale_name"`
	SkuInfoList  []SkuPriceInfo `json:"sku_info_list"`
	PreCheckInfo PreCheckInfo   `json:"pre_check_info"`
}

// SkuPriceInfo SKU 价格信息
type SkuPriceInfo struct {
	SkuCode       string           `json:"sku_code"`
	SaleNameInfo  []SaleNameInfo   `json:"sale_name_info"`
	PriceInfoList []SkuPriceDetail `json:"price_info_list"`
}

// SaleNameInfo 销售名称信息
type SaleNameInfo struct {
	SkuCode          string `json:"sku_code"`
	AttributeID      int    `json:"attribute_id"`
	AttributeValueID int    `json:"attribute_value_id"`
	SaleAttrName     string `json:"sale_attr_name"`
	SaleName         string `json:"sale_name"`
}

// SkuPriceDetail SKU 价格详情
type SkuPriceDetail struct {
	SubSite             string  `json:"sub_site"`
	SiteName            string  `json:"site_name"`
	Currency            string  `json:"currency"`
	ShopPrice           float64 `json:"shop_price"`
	SpecialPrice        float64 `json:"special_price"`
	FirstShelfTimeLimit bool    `json:"first_shelf_time_limit"`
}

// PreCheckInfo 预检查信息
type PreCheckInfo struct {
	CanChangePrice bool        `json:"can_change_price"`
	Prompt         any `json:"prompt"`
	LinkURL        any `json:"link_url"`
}
