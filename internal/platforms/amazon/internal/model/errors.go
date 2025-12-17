// Package model 提供Amazon平台错误定义
package model

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

	// ErrInvalidProductType 无效的产品类型
	ErrInvalidProductType = errors.New("invalid product type")

	// ErrMissingRequiredAttribute 缺少必需属性
	ErrMissingRequiredAttribute = errors.New("missing required attribute")

	// ErrInvalidAttributeValue 无效的属性值
	ErrInvalidAttributeValue = errors.New("invalid attribute value")

	// ErrSchemaValidationFailed Schema验证失败
	ErrSchemaValidationFailed = errors.New("schema validation failed")

	// ErrImageProcessingFailed 图片处理失败
	ErrImageProcessingFailed = errors.New("image processing failed")

	// ErrS3UploadFailed S3上传失败
	ErrS3UploadFailed = errors.New("S3 upload failed")

	// ErrVariantValidationFailed 变体验证失败
	ErrVariantValidationFailed = errors.New("variant validation failed")
)

// AmazonError Amazon平台错误
type AmazonError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Error 实现error接口
func (e *AmazonError) Error() string {
	if len(e.Details) > 0 {
		return fmt.Sprintf("Amazon错误 [%s]: %s (详情: %v)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("Amazon错误 [%s]: %s", e.Code, e.Message)
}

// NewAmazonError 创建Amazon错误
func NewAmazonError(code, message string) *AmazonError {
	return &AmazonError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithDetails 添加错误详情
func (e *AmazonError) WithDetails(key string, value interface{}) *AmazonError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// Error 实现error接口
func (e *ValidationError) Error() string {
	return fmt.Sprintf("验证错误 [%s]: %s (值: %s)", e.Field, e.Message, e.Value)
}

// NewValidationError 创建验证错误
func NewValidationError(field, value, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// MultiError 多个错误的集合
type MultiError struct {
	Errors []error `json:"errors"`
}

// Error 实现error接口
func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "无错误"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("发生 %d 个错误: %s (等)", len(e.Errors), e.Errors[0].Error())
}

// Add 添加错误
func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// HasErrors 检查是否有错误
func (e *MultiError) HasErrors() bool {
	return len(e.Errors) > 0
}

// ToError 转换为error（如果有错误的话）
func (e *MultiError) ToError() error {
	if e.HasErrors() {
		return e
	}
	return nil
}

// NewMultiError 创建多错误集合
func NewMultiError() *MultiError {
	return &MultiError{
		Errors: make([]error, 0),
	}
}

// 常用错误代码
const (
	ErrorCodeInvalidRequest   = "INVALID_REQUEST"
	ErrorCodeUnauthorized     = "UNAUTHORIZED"
	ErrorCodeForbidden        = "FORBIDDEN"
	ErrorCodeNotFound         = "NOT_FOUND"
	ErrorCodeRateLimit        = "RATE_LIMIT"
	ErrorCodeInternalError    = "INTERNAL_ERROR"
	ErrorCodeValidationFailed = "VALIDATION_FAILED"
	ErrorCodeSchemaError      = "SCHEMA_ERROR"
	ErrorCodeImageError       = "IMAGE_ERROR"
	ErrorCodeUploadError      = "UPLOAD_ERROR"
	ErrorCodeVariantError     = "VARIANT_ERROR"
)

// IsRetryableError 检查错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查Amazon错误
	if amazonErr, ok := err.(*AmazonError); ok {
		retryableCodes := []string{
			ErrorCodeRateLimit,
			ErrorCodeInternalError,
		}
		for _, code := range retryableCodes {
			if amazonErr.Code == code {
				return true
			}
		}
	}

	// 检查标准错误
	retryableErrors := []error{
		ErrAPIRateLimit,
	}
	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	return false
}

// IsValidationError 检查是否是验证错误
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}

	// 检查ValidationError类型
	if _, ok := err.(*ValidationError); ok {
		return true
	}

	// 检查Amazon错误中的验证错误
	if amazonErr, ok := err.(*AmazonError); ok {
		return amazonErr.Code == ErrorCodeValidationFailed
	}

	// 检查标准验证错误
	validationErrors := []error{
		ErrInvalidASIN,
		ErrInvalidSKU,
		ErrInvalidProductType,
		ErrMissingRequiredAttribute,
		ErrInvalidAttributeValue,
		ErrSchemaValidationFailed,
		ErrVariantValidationFailed,
	}
	for _, validationErr := range validationErrors {
		if errors.Is(err, validationErr) {
			return true
		}
	}

	return false
}
