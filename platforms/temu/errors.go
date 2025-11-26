package temu

import (
	"task-processor/platforms/temu/types"
)

// 为了向后兼容，导出types包中的错误定义和函数
var (
	ErrProductNotFound     = types.ErrProductNotFound
	ErrProductOffline      = types.ErrProductOffline
	ErrAuthExpired         = types.ErrAuthExpired
	ErrTooManyVariants     = types.ErrTooManyVariants
	ErrInvalidASIN         = types.ErrInvalidASIN
	ErrDuplicateProduct    = types.ErrDuplicateProduct
	ErrPageNotFound        = types.ErrPageNotFound
	ErrMissingPageElements = types.ErrMissingPageElements
)

// 错误类型别名
type RetryableError = types.RetryableError
type NonRetryableError = types.NonRetryableError

// 错误构造函数别名
var (
	NewRetryableError    = types.NewRetryableError
	NewNonRetryableError = types.NewNonRetryableError
	IsRetryableError     = types.IsRetryableError
	IsAuthExpiredError   = types.IsAuthExpiredError
)
