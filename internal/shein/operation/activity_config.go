// Package operation 提供SHEIN平台调度器相关配置
package operation

import "time"

// TimeLimitedDiscountConfig 限时折扣活动配置
type TimeLimitedDiscountConfig struct {
	// 活动基础信息
	ActivityName string    // 活动名称（格式：#用户名#限时折扣#日期#序号）
	TimeZone     string    // 时区（如：America/Los_Angeles）
	StartTime    time.Time // 开始时间
	EndTime      time.Time // 结束时间
	RefToolID    int       // 工具ID（默认30）
	SubTypeID    int       // 子类型ID（默认2）

	// 活动规则
	DiscountRate  float64 // 折扣率（0-1之间，如0.4表示打6折，即40%off）
	MinProfitRate float64 // 最低利润率（0-1之间，如0.15表示15%利润率）

	PriceMode     string // 定价模式（DISCOUNT:按折扣率, PROFIT:按最低利润率）
	GoodsLimit    int    // 商品限制（0:不限购, 1:限购）
	GoodsLimitNum int    // 商品限制数量（单用户限购数量）
	StockLimit    bool   // 是否启用活动库存限量
	StockPercent  int    // 活动库存限量百分比（1-100）

	// 商品筛选条件
	EffectiveCenterList []int // 生效中心列表
	IsShelf             int   // 是否上架（0:否, 1:是）
	PageSize            int   // 每页查询数量（默认30）

	// 定价配置
	Currency    string // 币种（如：USD）
	SceneID     int    // 场景ID（默认1）
	PricingType int    // 定价类型（默认2）

	// 商品配置
	DefaultAttendNum int // 默认参与数量
	DefaultStockNum  int // 默认库存数量

	// 价格风险控制
	AllowRiskProducts bool    // 是否允许有风险的商品
	MaxWarningValue   float64 // 最大警告值

	// 价格调整
	FixedPriceAdjustment float64 // 固定价格调整值（在最低售价基础上增加的固定金额）
}

// DefaultTimeLimitedDiscountConfig 返回默认配置
func DefaultTimeLimitedDiscountConfig() TimeLimitedDiscountConfig {
	return TimeLimitedDiscountConfig{
		TimeZone:             "America/Los_Angeles",
		RefToolID:            30,
		SubTypeID:            2,
		DiscountRate:         0.4,        // 默认40%off（打6折）
		MinProfitRate:        0.15,       // 默认15%利润率
		PriceMode:            "DISCOUNT", // 默认按折扣率定价
		GoodsLimit:           0,          // 默认不限购
		GoodsLimitNum:        1,          // 默认限购数量为1
		StockLimit:           false,      // 默认不限量
		StockPercent:         100,        // 默认100%库存
		EffectiveCenterList:  []int{2},
		IsShelf:              1,
		PageSize:             100,
		Currency:             "USD",
		SceneID:              1,
		PricingType:          2,
		DefaultAttendNum:     30,
		DefaultStockNum:      30,
		AllowRiskProducts:    false,
		MaxWarningValue:      0,
		FixedPriceAdjustment: 0, // 默认不添加固定调整值
	}
}

// Validate 验证配置
func (c *TimeLimitedDiscountConfig) Validate() error {
	if c.ActivityName == "" {
		return ErrInvalidActivityName
	}
	if c.StartTime.IsZero() || c.EndTime.IsZero() {
		return ErrInvalidActivityTime
	}
	if c.StartTime.After(c.EndTime) {
		return ErrInvalidActivityTimeRange
	}
	if c.TimeZone == "" {
		return ErrInvalidTimeZone
	}
	return nil
}
