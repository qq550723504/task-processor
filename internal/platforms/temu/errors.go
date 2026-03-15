package temu

import (
	"errors"
	"fmt"
	"strings"
)

// 标准错误定义
var (
	ErrProductNotFound     = errors.New("产品不存在")
	ErrProductOffline      = errors.New("产品已下架")
	ErrAuthExpired         = errors.New("认证已过期")
	ErrTooManyVariants     = errors.New("变体数量过多")
	ErrInvalidASIN         = errors.New("ASIN无效")
	ErrDuplicateProduct    = errors.New("产品重复")
	ErrPageNotFound        = errors.New("页面不存在")
	ErrMissingPageElements = errors.New("页面缺少必要元素")
)

// RetryableError 可重试错误
type RetryableError struct {
	Message string
	Cause   error
}

func (e *RetryableError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *RetryableError) Unwrap() error {
	return e.Cause
}

// NewRetryableError 创建可重试错误
func NewRetryableError(message string, cause error) *RetryableError {
	return &RetryableError{
		Message: message,
		Cause:   cause,
	}
}

// NonRetryableError 不可重试错误
type NonRetryableError struct {
	Message string
	Cause   error
}

func (e *NonRetryableError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *NonRetryableError) Unwrap() error {
	return e.Cause
}

// NewNonRetryableError 创建不可重试错误
func NewNonRetryableError(message string, cause error) *NonRetryableError {
	return &NonRetryableError{
		Message: message,
		Cause:   cause,
	}
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为认证过期错误（需要暂停，不是重试也不是终止）
	if IsAuthExpiredError(err) {
		return false
	}

	// 检查标准不可重试错误
	if errors.Is(err, ErrProductNotFound) ||
		errors.Is(err, ErrProductOffline) ||
		errors.Is(err, ErrInvalidASIN) ||
		errors.Is(err, ErrDuplicateProduct) ||
		errors.Is(err, ErrPageNotFound) ||
		errors.Is(err, ErrMissingPageElements) ||
		errors.Is(err, ErrTooManyVariants) {
		return false
	}

	// 检查是否为明确的不可重试错误
	var nonRetryableErr *NonRetryableError
	if errors.As(err, &nonRetryableErr) {
		return false
	}

	// 检查是否为handlers包中的NonRetryableError（通过类型名称判断）
	if isHandlersNonRetryableError(err) {
		return false
	}

	// 检查是否为明确的可重试错误
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return true
	}

	// 默认根据错误内容判断
	return isRetryableByContent(err)
}

// isHandlersNonRetryableError 检查是否为handlers包中的NonRetryableError
// 通过类型名称判断，避免循环导入
func isHandlersNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 获取错误类型的字符串表示
	typeName := fmt.Sprintf("%T", err)

	// 检查是否为handlers包中的NonRetryableError
	return typeName == "*handlers.NonRetryableError"
}

// IsAuthExpiredError 判断错误是否为认证过期错误
func IsAuthExpiredError(err error) bool {
	if err == nil {
		return false
	}

	// 使用 errors.Is 检查标准错误
	if errors.Is(err, ErrAuthExpired) {
		return true
	}

	// 通过接口检测（避免循环导入）
	type authExpiredMarker interface {
		IsAuthExpired() bool
	}
	var marker authExpiredMarker
	if errors.As(err, &marker) && marker.IsAuthExpired() {
		return true
	}

	// 检查错误内容是否包含Cookie相关关键词（向后兼容）
	errStr := strings.ToLower(err.Error())
	cookieErrors := []string{
		"cookie数据为空",
		"没有cookie数据",
		"从管理系统获取cookie失败",
		"请先在管理系统中设置cookie",
	}

	for _, cookieErr := range cookieErrors {
		if strings.Contains(errStr, cookieErr) {
			return true
		}
	}

	return false
}

// isRetryableByContent 根据错误内容判断是否可重试
func isRetryableByContent(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// 检查是否包含NONRETRYABLE或TERMINATED关键字（最高优先级）
	if strings.Contains(errStr, "nonretryable:") || strings.Contains(errStr, "terminated:") {
		return false
	}

	// 网络相关错误通常可重试
	networkErrors := []string{
		"connection refused",
		"timeout",
		"network",
		"dial",
		"eof",
		"broken pipe",
		"no such host",
		"connection reset",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(errStr, netErr) {
			return true
		}
	}

	// 临时性错误可重试
	temporaryErrors := []string{
		"temporary",
		"retry",
		"rate limit",
		"service unavailable",
		"internal server error",
		"502 bad gateway",
		"503 service unavailable",
		"504 gateway timeout",
	}

	for _, tempErr := range temporaryErrors {
		if strings.Contains(errStr, tempErr) {
			return true
		}
	}

	// 数据验证错误通常不可重试
	validationErrors := []string{
		"validation failed",
		"invalid data",
		"parse error",
		"format error",
		"duplicate",
		"duplicated",
		"重复",
		"已存在",
		"unauthorized",
		"forbidden",
		"not found",
		"bad request",
		"产品不存在",
		"产品页面不存在",
		"产品页面缺少必要元素",
		"page not found",
		"页面不存在(404)",
		"页面不存在",
		"asin无效",
		"产品已下架",
		"变体数量过多",
		"变体asin数量过多",
	}

	for _, valErr := range validationErrors {
		if strings.Contains(errStr, valErr) {
			return false
		}
	}

	// 默认可重试
	return true
}
