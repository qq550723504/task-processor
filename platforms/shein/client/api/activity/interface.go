// Package activity 提供SHEIN活动产品相关API接口定义
package activity

// ActivityProductAPI 活动产品相关API接口
type ActivityProductAPI interface {
	// GetAvailableActivityProducts 获取可报名活动的产品列表
	GetAvailableActivityProducts(req *GetAvailableActivityProductsRequest) (*GetAvailableActivityProductsResponse, error)
}

// GetAvailableActivityProductsRequest 获取可报名活动产品列表请求参数
type GetAvailableActivityProductsRequest struct {
	PageNum  int `json:"page_num"`  // 页码
	PageSize int `json:"page_size"` // 每页数量
}

// GetAvailableActivityProductsResponse 获取可报名活动产品列表响应
type GetAvailableActivityProductsResponse struct {
	Code string                        `json:"code"` // 响应码
	Msg  string                        `json:"msg"`  // 响应消息
	Info *AvailableActivityProductInfo `json:"info"` // 响应数据
	BBL  any                           `json:"bbl"`  // BBL字段
}

// AvailableActivityProductInfo 可报名活动产品列表信息
type AvailableActivityProductInfo struct {
	Total       int                   `json:"total"`         // 总数量
	SkcInfoList []ActivityProductInfo `json:"skc_info_list"` // SKC信息列表
}

// ActivityProductInfo 活动产品信息
type ActivityProductInfo struct {
	Skc                 string                  `json:"skc"`                   // SKC编码
	GoodsName           string                  `json:"goods_name"`            // 商品名称
	Image               string                  `json:"image"`                 // 商品图片URL
	SupplierNo          string                  `json:"supplier_no"`           // 供应商编号
	Stock               int                     `json:"stock"`                 // 库存数量
	SupplyPrice         float64                 `json:"supply_price"`          // 供货价格
	SupplyPriceCurrency string                  `json:"supply_price_currency"` // 供货价格币种
	IsConfigured        bool                    `json:"is_configured"`         // 是否已配置
	SitePriceInfoList   []ActivitySitePriceInfo `json:"site_price_info_list"`  // 站点价格信息列表
}

// ActivitySitePriceInfo 活动站点价格信息
type ActivitySitePriceInfo struct {
	SiteCode    string  `json:"site_code"`    // 站点代码
	SalePrice   float64 `json:"sale_price"`   // 销售价格
	Currency    string  `json:"currency"`     // 币种
	IsAvailable bool    `json:"is_available"` // 是否可用
}
