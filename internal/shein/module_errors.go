package shein

import (
	"errors"
)

// RetryableError 可重试错误接口
type RetryableError interface {
	error
	IsRetryable() bool
}

// retryableError 可重试错误实现
type retryableError struct {
	message    string
	retryable  bool
	wrappedErr error
}

// Error 实现error接口
func (e *retryableError) Error() string {
	if e.wrappedErr != nil {
		return e.message + ": " + e.wrappedErr.Error()
	}
	return e.message
}

// IsRetryable 实现RetryableError接口
func (e *retryableError) IsRetryable() bool {
	return e.retryable
}

// Unwrap 实现错误包装接口
func (e *retryableError) Unwrap() error {
	return e.wrappedErr
}

// NewRetryableError 创建可重试错误
// 自动检查是否为认证过期错误，如果是则直接返回原错误而不包装
func NewRetryableError(message string, err error) error {
	// 如果包装的错误是认证过期错误，直接返回原错误，不进行包装
	if isAuthenticationExpiredError(err) {
		return err
	}

	return &retryableError{
		message:    message,
		retryable:  true,
		wrappedErr: err,
	}
}

// NewNonRetryableError 创建不可重试错误
func NewNonRetryableError(message string, err error) error {
	return &retryableError{
		message:    message,
		retryable:  false,
		wrappedErr: err,
	}
}

// FilteredError 业务过滤错误（非真正的错误，只是不符合筛选条件）
type FilteredError struct {
	message string
}

func (e *FilteredError) Error() string {
	return e.message
}

func (e *FilteredError) IsRetryable() bool {
	return false
}

// NewFilteredError 创建业务过滤错误
func NewFilteredError(message string) error {
	return &FilteredError{message: message}
}

// IsFilteredError 检查是否为业务过滤错误
func IsFilteredError(err error) bool {
	if err == nil {
		return false
	}

	// 直接检查类型
	if _, ok := err.(*FilteredError); ok {
		return true
	}

	// 检查错误消息中的关键字（筛选规则相关）
	errMsg := err.Error()
	filterKeywords := []string{
		"低于筛选规则",
		"高于筛选规则",
		"超过筛选规则",
		"筛选规则最低",
		"筛选规则最高",
	}

	for _, keyword := range filterKeywords {
		if contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

// IsRetryableError 检查错误是否可重试
func IsRetryableError(err error) bool {
	// 首先检查是否为认证过期错误，认证过期错误应该触发特殊处理而不是重试
	if isAuthenticationExpired(err) {
		return false
	}

	// 检查是否为明确的不可重试错误
	if isNonRetryableError(err) {
		return false
	}

	if retryableErr, ok := err.(RetryableError); ok {
		return retryableErr.IsRetryable()
	}

	// 默认情况下，如果未明确标记，认为是可重试的
	return true
}

// isAuthenticationExpired 内部函数，检查是否为认证过期错误
// 避免循环导入，这里重新实现认证过期检查逻辑
func isAuthenticationExpired(err error) bool {
	// 检查错误消息中是否包含认证过期的关键字
	if err != nil {
		errMsg := err.Error()
		// 检查是否包含认证过期的关键信息
		if contains(errMsg, "20302") && contains(errMsg, "子系统登录重定向") {
			return true
		}
		if contains(errMsg, "认证已过期") || contains(errMsg, "需要重新登录") {
			return true
		}
	}

	// 递归检查包装的错误
	for err != nil {
		errMsg := err.Error()
		if contains(errMsg, "20302") && contains(errMsg, "子系统登录重定向") {
			return true
		}
		if contains(errMsg, "认证已过期") || contains(errMsg, "需要重新登录") {
			return true
		}

		// 检查是否实现了 Unwrap 方法
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
	}

	return false
}

// isAuthenticationExpiredError 检查是否为认证过期错误（用于NewRetryableError）
func isAuthenticationExpiredError(err error) bool {
	if err == nil {
		return false
	}

	// 检查错误消息中是否包含认证过期的关键字
	errMsg := err.Error()

	// 添加调试日志
	// if contains(errMsg, "20302") {
	//     // 这里可以添加日志，但为了避免循环导入，我们先注释掉
	//     // logrus.Debugf("检测到20302错误: %s", errMsg)
	// }

	// 检查是否包含认证过期的关键信息
	if contains(errMsg, "20302") && contains(errMsg, "子系统登录重定向") {
		return true
	}
	if contains(errMsg, "认证已过期") || contains(errMsg, "需要重新登录") {
		return true
	}

	// 递归检查包装的错误
	for err != nil {
		errMsg := err.Error()
		if contains(errMsg, "20302") && contains(errMsg, "子系统登录重定向") {
			return true
		}
		if contains(errMsg, "认证已过期") || contains(errMsg, "需要重新登录") {
			return true
		}

		// 检查是否实现了 Unwrap 方法
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
	}

	return false
}

// isNonRetryableError 检查是否为不可重试错误
func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// 首先使用 errors.As 检查是否为 retryableError 类型且不可重试
	var retryableErr *retryableError
	if errors.As(err, &retryableErr) {
		if !retryableErr.IsRetryable() {
			return true
		}
	}

	// 检查是否为 FilteredError 类型（业务过滤错误，不可重试）
	var filteredErr *FilteredError
	if errors.As(err, &filteredErr) {
		if !filteredErr.IsRetryable() {
			return true
		}
	}

	// 检查是否为Cookie加载失败错误（不可重试）
	if contains(errMsg, "Cookie加载失败") {
		return true
	}

	// 检查是否包含404类错误（产品不存在，不可重试）
	notFoundPatterns := []string{
		"不是有效的产品页面",
		"产品页面不存在",
		"产品页面缺少必要元素",
		"页面不存在(404)",
		"页面不存在",
		"页面未准备就绪: 页面不存在",
		"product not found",
		"Product not found",
		"404",
		"not found",
		"Not Found",
	}

	for _, pattern := range notFoundPatterns {
		if contains(errMsg, pattern) {
			return true
		}
	}

	// 检查是否包含"卖家SKU重复"错误
	if contains(errMsg, "卖家SKU重复") {
		return true
	}

	// 检查是否包含"变体ASIN数量过多"错误
	if contains(errMsg, "变体ASIN数量过多") {
		return true
	}

	// 递归检查包装的错误
	for err != nil {
		errMsg := err.Error()

		// 检查404类错误
		for _, pattern := range notFoundPatterns {
			if contains(errMsg, pattern) {
				return true
			}
		}

		if contains(errMsg, "卖家SKU重复") {
			return true
		}
		if contains(errMsg, "Cookie加载失败") {
			return true
		}
		if contains(errMsg, "变体ASIN数量过多") {
			return true
		}

		// 检查是否实现了 Unwrap 方法
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
	}

	return false
}

// contains 检查字符串是否包含子字符串（简单实现）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
