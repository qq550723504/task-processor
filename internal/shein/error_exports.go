// Package shein 提供SHEIN平台的错误定义
// 错误类型实现在 sherr 子包，此处重新导出以保持向后兼容
package shein

import "task-processor/internal/shein/sherr"

// RetryableError 可重试错误接口
type RetryableError = sherr.RetryableError

// CookieLoadError Cookie加载失败错误
type CookieLoadError = sherr.CookieLoadError

// FilteredError 业务过滤错误
type FilteredError = sherr.FilteredError

// TaskHandledError 任务状态已处理完成错误
type TaskHandledError = sherr.TaskHandledError

// NewRetryableError 创建可重试错误
var NewRetryableError = sherr.NewRetryableError

// NewNonRetryableError 创建不可重试错误
var NewNonRetryableError = sherr.NewNonRetryableError

// NewFilteredError 创建业务过滤错误
var NewFilteredError = sherr.NewFilteredError

// NewTaskHandledError 创建任务状态已处理完成错误
var NewTaskHandledError = sherr.NewTaskHandledError

// AsTaskHandledError 返回任务已处理错误及其断言结果
var AsTaskHandledError = sherr.AsTaskHandledError

// IsFilteredError 检查是否为业务过滤错误
var IsFilteredError = sherr.IsFilteredError

// IsTaskHandledError 检查是否为任务状态已处理完成错误
var IsTaskHandledError = sherr.IsTaskHandledError

// IsRetryableError 检查错误是否可重试
var IsRetryableError = sherr.IsRetryableError

// IsAuthenticationExpiredError 检查是否为认证过期错误
var IsAuthenticationExpiredError = sherr.IsAuthenticationExpiredError

// NewCookieLoadError 创建Cookie加载错误
var NewCookieLoadError = sherr.NewCookieLoadError

// IsCookieLoadError 检查是否是Cookie加载错误
var IsCookieLoadError = sherr.IsCookieLoadError

// NewSensitiveWordsFilter 保持在 shein 包中（不涉及循环依赖）
