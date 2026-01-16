// Package marketing 提供SHEIN价格计算相关数据类型定义
package marketing

// CalculateSupplyPriceRequest 计算供货价格请求参数
type CalculateSupplyPriceRequest struct {
	Currency      string         `json:"currency"`        // 币种（如：USD）
	RefToolID     int            `json:"ref_tool_id"`     // 工具ID
	SceneID       int            `json:"scene_id"`        // 场景ID
	SkcInfoList   []SkcPriceInfo `json:"skc_info_list"`   // SKC信息列表
	TimeZone      string         `json:"time_zone"`       // 时区
	ZoneEndTime   string         `json:"zone_end_time"`   // 结束时间
	ZoneStartTime string         `json:"zone_start_time"` // 开始时间
}

// SkcPriceInfo SKC价格信息
type SkcPriceInfo struct {
	SkcName     string         `json:"skc_name"`      // SKC名称
	SkuInfoList []SkuPriceInfo `json:"sku_info_list"` // SKU信息列表
}

// SkuPriceInfo SKU价格信息
type SkuPriceInfo struct {
	DiscountValue float64 `json:"discount_value"` // 折扣价格
	ProductPrice  float64 `json:"product_price"`  // 商品原价
	SkuCode       string  `json:"sku_code"`       // SKU编码
}

// CalculateSupplyPriceResponse 计算供货价格响应
type CalculateSupplyPriceResponse struct {
	Code string                 `json:"code"` // 响应码
	Msg  string                 `json:"msg"`  // 响应消息
	Info []SkcCalculationResult `json:"info"` // 计算结果列表
	BBL  interface{}            `json:"bbl"`  // BBL字段
}

// SkcCalculationResult SKC计算结果
type SkcCalculationResult struct {
	SkcName     string               `json:"skc_name"`      // SKC名称
	SkuInfoList []SkuCalculationInfo `json:"sku_info_list"` // SKU计算信息列表
}

// SkuCalculationInfo SKU计算信息
type SkuCalculationInfo struct {
	SkuCode      string    `json:"sku_code"`      // SKU编码
	PriceInfo    PriceInfo `json:"price_info"`    // 价格信息
	RiskTag      int       `json:"risk_tag"`      // 风险标签（0:无风险）
	WarningValue float64   `json:"warning_value"` // 警告值
}

// PriceInfo 价格详细信息
type PriceInfo struct {
	ProductAmount      float64 `json:"product_amount"`       // 商品金额
	SettlementAmount   float64 `json:"settlement_amount"`    // 结算金额
	PromotionAmount    float64 `json:"promotion_amount"`     // 促销金额
	CouponAmount       float64 `json:"coupon_amount"`        // 优惠券金额
	DiscountAmount     float64 `json:"discount_amount"`      // 折扣金额
	StockExpenseAmount float64 `json:"stock_expense_amount"` // 库存费用金额
	PerformanceAmount  float64 `json:"performance_amount"`   // 绩效金额
}
