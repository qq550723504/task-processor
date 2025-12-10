package amazon

import (
	"errors"
	"fmt"
)

// 定义Amazon平台特定的错误类型

var (
	// ErrInvalidASIN ASIN无效
	ErrInvalidASIN = errors.New("invalid ASIN")

	// ErrInvalidSKU SKU无效
	ErrInvalidSKU = errors.New("invalid SKU")

	// ErrProductNotFound 产品未找到
	ErrProductNotFound = errors.New("product not found")

	// ErrAPIRateLimit API速率限制
	ErrAPIRateLimit = errors.New("API rate limit exceeded")

	// ErrAuthenticationFailed 认证失败
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrInsufficientInventory 库存不足
	ErrInsufficientInventory = errors.New("insufficient inventory")

	// ErrCategoryRestricted 分类受限
	ErrCategoryRestricted = errors.New("category restricted")
)

// APIError Amazon API错误
type APIError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

// Error 实现error接口
func (e *APIError) Error() string {
	return fmt.Sprintf("Amazon API错误 [%s]: %s", e.Code, e.Message)
}

// NewAPIError 创建API错误
func NewAPIError(code, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

// Error 实现error接口
func (e *ValidationError) Error() string {
	return fmt.Sprintf("验证失败 [%s]: %s", e.Field, e.Message)
}

// NewValidationError 创建验证错误
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// API速率限制可重试
	if errors.Is(err, ErrAPIRateLimit) {
		return true
	}

	// API错误根据错误码判断
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "Throttled", "ServiceUnavailable", "InternalError":
			return true
		}
	}

	return false
}
