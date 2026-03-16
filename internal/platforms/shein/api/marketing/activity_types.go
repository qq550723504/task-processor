// Package marketing 提供SHEIN活动创建相关数据类型定义
package marketing

// CreateActivityRequest 创建活动请求参数
type CreateActivityRequest struct {
	ActivityBaseInfoRequest ActivityBaseInfo   `json:"activity_base_info_request"`   // 活动基础信息
	AddCostAndStockInfoList []CostAndStockInfo `json:"add_cost_and_stock_info_list"` // 成本和库存信息列表
	PricingType             int                `json:"pricing_type"`                 // 定价类型
}

// ActivityBaseInfo 活动基础信息
type ActivityBaseInfo struct {
	ActName       string       `json:"act_name"`        // 活动名称
	TimeZone      string       `json:"time_zone"`       // 时区
	ActivityRule  ActivityRule `json:"activity_rule"`   // 活动规则
	ZoneStartTime string       `json:"zone_start_time"` // 开始时间
	ZoneEndTime   string       `json:"zone_end_time"`   // 结束时间
	RefToolID     int          `json:"ref_tool_id"`     // 工具ID
	NotifyFlag    int          `json:"notify_flag"`     // 通知标志（1:通知）
	SubTypeID     int          `json:"sub_type_id"`     // 子类型ID
}

// ActivityRule 活动规则
type ActivityRule struct {
	GoodsLimit    int `json:"goods_limit"`     // 商品限制
	GoodsLimitNum int `json:"goods_limit_num"` // 商品限制数量
}

// CostAndStockInfo 成本和库存信息
type CostAndStockInfo struct {
	AttendNum          int           `json:"attend_num"`            // 参与数量
	CenterList         []int         `json:"center_list"`           // 中心列表
	IsSaleAttribute    int           `json:"is_sale_attribute"`     // 是否销售属性
	PromotionIDList    []string      `json:"promotion_id_list"`     // 促销ID列表
	Skc                string        `json:"skc"`                   // SKC编码
	StockNum           int           `json:"stock_num"`             // 库存数量
	CostPrice          float64       `json:"cost_price"`            // 成本价格
	MaxProductActPrice float64       `json:"max_product_act_price"` // 最大商品活动价格
	ProductActPrice    float64       `json:"product_act_price"`     // 商品活动价格
	AddSkuList         []SkuCostInfo `json:"add_sku_list"`          // SKU列表
}

// SkuCostInfo SKU成本信息
type SkuCostInfo struct {
	CostPrice          float64 `json:"cost_price"`            // 成本价格
	Sku                string  `json:"sku"`                   // SKU编码
	MaxProductActPrice float64 `json:"max_product_act_price"` // 最大商品活动价格
	ProductActPrice    float64 `json:"product_act_price"`     // 商品活动价格
}

// CreateActivityResponse 创建活动响应
type CreateActivityResponse struct {
	Code string              `json:"code"` // 响应码
	Msg  string              `json:"msg"`  // 响应消息
	Info *ActivityCreateInfo `json:"info"` // 活动创建信息
	BBL  any         `json:"bbl"`  // BBL字段
}

// ActivityCreateInfo 活动创建信息
type ActivityCreateInfo struct {
	ActivityID   int64       `json:"activity_id"`    // 活动ID
	ErrorInfo    any `json:"error_info"`     // 错误信息
	SkcErrorInfo any `json:"skc_error_info"` // SKC错误信息
	SkuErrorInfo any `json:"sku_error_info"` // SKU错误信息
}
