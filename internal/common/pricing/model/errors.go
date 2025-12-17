// Package model 提供核价相关的错误定义。
package model

import "errors"

// 核价相关错误定义
var (
	// 参数验证错误
	ErrInvalidTenantID  = errors.New("无效的租户ID")
	ErrInvalidStoreID   = errors.New("无效的店铺ID")
	ErrInvalidProductID = errors.New("无效的商品ID")
	ErrInvalidPrice     = errors.New("无效的价格")
	ErrInvalidRule      = errors.New("无效的核价规则")

	// 业务逻辑错误
	ErrNoPricingRule       = errors.New("未找到适用的核价规则")
	ErrNoCostPrice         = errors.New("未找到成本价信息")
	ErrNoImportMapping     = errors.New("未找到商品导入映射")
	ErrAutoPricingDisabled = errors.New("自动核价功能未启用")

	// 计算错误
	ErrCalculationFailed = errors.New("价格计算失败")
	ErrZeroCostPrice     = errors.New("成本价不能为零")
	ErrNegativePrice     = errors.New("价格不能为负数")

	// 系统错误
	ErrClientNotFound     = errors.New("客户端未找到")
	ErrServiceUnavailable = errors.New("服务不可用")
)
