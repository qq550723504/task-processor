// Package marketing 提供SHEIN促销活动相关数据类型定义
package marketing

// QueryPromotionGoodsRequest 查询促销活动商品请求参数
type QueryPromotionGoodsRequest struct {
	ActivityBaseInfoRequest ActivityBaseInfoRequest `json:"activity_base_info_request"` // 活动基础信息
	EffectiveCenterList     []int                   `json:"effective_center_list"`      // 生效中心列表
	IsShelf                 int                     `json:"is_shelf"`                   // 是否上架 (0:否, 1:是)
	PageNum                 int                     `json:"page_num"`                   // 页码
	PageSize                int                     `json:"page_size"`                  // 每页数量
}

// ActivityBaseInfoRequest 活动基础信息请求
type ActivityBaseInfoRequest struct {
	ActName       string `json:"act_name"`        // 活动名称
	RefToolID     int    `json:"ref_tool_id"`     // 工具ID
	TimeZone      string `json:"time_zone"`       // 时区
	ZoneEndTime   string `json:"zone_end_time"`   // 结束时间
	ZoneStartTime string `json:"zone_start_time"` // 开始时间
	SubTypeID     int    `json:"sub_type_id"`     // 子类型ID
}

// QueryPromotionGoodsResponse 查询促销活动商品响应
type QueryPromotionGoodsResponse struct {
	Code string              `json:"code"` // 响应码
	Msg  string              `json:"msg"`  // 响应消息
	Info *PromotionGoodsInfo `json:"info"` // 响应数据
	BBL  interface{}         `json:"bbl"`  // BBL字段
}

// PromotionGoodsInfo 促销商品信息
type PromotionGoodsInfo struct {
	Data []PromotionGoodsData `json:"data"` // 商品数据列表
	Meta MetaInfo             `json:"meta"` // 元数据信息
}

// MetaInfo 元数据信息
type MetaInfo struct {
	Count     int         `json:"count"`     // 总数量
	CustomObj interface{} `json:"customObj"` // 自定义对象
}

// PromotionGoodsData 促销商品数据
type PromotionGoodsData struct {
	Skc                         string             `json:"skc"`                            // SKC编码
	IsSaleAttribute             int                `json:"is_sale_attribute"`              // 是否销售属性
	ErrorCode                   string             `json:"error_code"`                     // 错误码
	EffectiveCenterList         []int              `json:"effective_center_list"`          // 生效中心列表
	EffectivePromotionIDList    []string           `json:"effective_promotion_id_list"`    // 生效促销ID列表
	ImageURL                    string             `json:"image_url"`                      // 图片URL
	SkuSupplierNo               string             `json:"sku_supplier_no"`                // SKU供应商编号
	LayerID                     string             `json:"layer_id"`                       // 层级ID
	LayerNm                     string             `json:"layer_nm"`                       // 层级名称
	SupplierLayerNm             string             `json:"supplier_layer_nm"`              // 供应商层级名称
	GoodsNm                     string             `json:"goods_nm"`                       // 商品名称
	IvtNum                      int                `json:"ivt_num"`                        // 库存数量
	InventoryNum                int                `json:"inventory_num"`                  // 可用库存数量
	IsShelf                     int                `json:"is_shelf"`                       // 是否上架
	SupplyPrice                 *float64           `json:"supply_price"`                   // 供货价格
	USSupplyPrice               float64            `json:"us_supply_price"`                // 美元供货价格
	EURSupplyPrice              *float64           `json:"eur_supply_price"`               // 欧元供货价格
	UKSupplyPrice               *float64           `json:"uk_supply_price"`                // 英镑供货价格
	MXNSupplyPrice              *float64           `json:"mxn_supply_price"`               // 墨西哥比索供货价格
	MaxSupplyPrice              *float64           `json:"max_supply_price"`               // 最大供货价格
	MaxUSSupplyPrice            float64            `json:"max_us_supply_price"`            // 最大美元供货价格
	MaxEURSupplyPrice           *float64           `json:"max_eur_supply_price"`           // 最大欧元供货价格
	MaxUKSupplyPrice            *float64           `json:"max_uk_supply_price"`            // 最大英镑供货价格
	MaxMXNSupplyPrice           *float64           `json:"max_mxn_supply_price"`           // 最大墨西哥比索供货价格
	SkuInfoList                 []PromotionSkuInfo `json:"sku_info_list"`                  // SKU信息列表
	SupplyPriceInfo             *SupplyPriceInfo   `json:"supply_price_info"`              // 供货价格信息
	CheckSupplyPrice            *CheckSupplyPrice  `json:"check_supply_price"`             // 检查供货价格
	RateWarning                 float64            `json:"rate_warning"`                   // 警告比率
	RateIntercept               float64            `json:"rate_intercept"`                 // 拦截比率
	CheckStock                  *CheckStock        `json:"check_stock"`                    // 检查库存
	MinActSupplyPrice           *float64           `json:"min_act_supply_price"`           // 最小活动供货价格
	MinActSupplyPriceCurrency   *string            `json:"min_act_supply_price_currency"`  // 最小活动供货价格币种
	CouponDiscountValue         *float64           `json:"coupon_discount_value"`          // 优惠券折扣值
	CouponDiscountValueCurrency *string            `json:"coupon_discount_value_currency"` // 优惠券折扣值币种
}

// PromotionSkuInfo 促销SKU信息
type PromotionSkuInfo struct {
	Sku               string             `json:"sku"`                  // SKU编码
	SupplyPrice       *float64           `json:"supply_price"`         // 供货价格
	USSupplyPrice     *float64           `json:"us_supply_price"`      // 美元供货价格
	EURSupplyPrice    *float64           `json:"eur_supply_price"`     // 欧元供货价格
	UKSupplyPrice     *float64           `json:"uk_supply_price"`      // 英镑供货价格
	MXNSupplyPrice    *float64           `json:"mxn_supply_price"`     // 墨西哥比索供货价格
	MaxSupplyPrice    *float64           `json:"max_supply_price"`     // 最大供货价格
	MaxUSSupplyPrice  *float64           `json:"max_us_supply_price"`  // 最大美元供货价格
	MaxEURSupplyPrice *float64           `json:"max_eur_supply_price"` // 最大欧元供货价格
	MaxUKSupplyPrice  *float64           `json:"max_uk_supply_price"`  // 最大英镑供货价格
	MaxMXNSupplyPrice *float64           `json:"max_mxn_supply_price"` // 最大墨西哥比索供货价格
	SupplyPriceInfo   *SupplyPriceInfo   `json:"supply_price_info"`    // 供货价格信息
	CheckSupplyPrice  *CheckSupplyPrice  `json:"check_supply_price"`   // 检查供货价格
	SkuAttrInfoList   []SkuAttributeInfo `json:"sku_attr_info_list"`   // SKU属性信息列表
}

// SkuAttributeInfo SKU属性信息
type SkuAttributeInfo struct {
	AttributeValueID   int    `json:"attribute_vale_id"`   // 属性值ID
	AttributeValueName string `json:"attribute_vale_name"` // 属性值名称
	IsMain             int    `json:"is_main"`             // 是否主属性
}

// SupplyPriceInfo 供货价格信息
type SupplyPriceInfo struct {
	SupplyPrice          float64  `json:"supply_price"`           // 供货价格
	Currency             string   `json:"currency"`               // 币种
	MaxSupplyPrice       float64  `json:"max_supply_price"`       // 最大供货价格
	WarnSupplyPrice      float64  `json:"warn_supply_price"`      // 警告供货价格
	InterceptSupplyPrice float64  `json:"intercept_supply_price"` // 拦截供货价格
	MinActSupplyPrice    *float64 `json:"min_act_supply_price"`   // 最小活动供货价格
}

// CheckSupplyPrice 检查供货价格
type CheckSupplyPrice struct {
	WarnSupplyPrice         *float64 `json:"warn_supply_price"`          // 警告供货价格
	WarnUSSupplyPrice       float64  `json:"warn_us_supply_price"`       // 警告美元供货价格
	WarnEURSupplyPrice      *float64 `json:"warn_eur_supply_price"`      // 警告欧元供货价格
	WarnUKSupplyPrice       *float64 `json:"warn_uk_supply_price"`       // 警告英镑供货价格
	WarnMXNSupplyPrice      *float64 `json:"warn_mxn_supply_price"`      // 警告墨西哥比索供货价格
	InterceptSupplyPrice    *float64 `json:"intercept_supply_price"`     // 拦截供货价格
	InterceptUSSupplyPrice  float64  `json:"intercept_us_supply_price"`  // 拦截美元供货价格
	InterceptEURSupplyPrice *float64 `json:"intercept_eur_supply_price"` // 拦截欧元供货价格
	InterceptUKSupplyPrice  *float64 `json:"intercept_uk_supply_price"`  // 拦截英镑供货价格
	InterceptMXNSupplyPrice *float64 `json:"intercept_mxn_supply_price"` // 拦截墨西哥比索供货价格
}

// CheckStock 检查库存
type CheckStock struct {
	MaxStock  int `json:"max_stock"`  // 最大库存
	MinStock  int `json:"min_stock"`  // 最小库存
	StockType int `json:"stock_type"` // 库存类型
}
