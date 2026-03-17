// package activity 提供SHEIN平台调度器相关错误定义
package activity

import "errors"

var (
	// 配置相关错误
	ErrInvalidActivityName      = errors.New("活动名称不能为空")
	ErrInvalidActivityTime      = errors.New("活动时间不能为空")
	ErrInvalidActivityTimeRange = errors.New("活动开始时间不能晚于结束时间")
	ErrInvalidTimeZone          = errors.New("时区不能为空")

	// 商品相关错误
	ErrNoAvailableProducts = errors.New("没有可用的商品")
	ErrProductPriceRisk    = errors.New("商品价格存在风险")
	ErrInsufficientStock   = errors.New("商品库存不足")

	// 活动创建错误
	ErrActivityCreationFailed = errors.New("活动创建失败")
	ErrActivityAlreadyExists  = errors.New("活动已存在")
)
