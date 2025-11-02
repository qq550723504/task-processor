package temu

import "fmt"

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

	// 检查是否为明确的不可重试错误
	var nonRetryableErr *NonRetryableError
	if As(err, &nonRetryableErr) {
		return false
	}

	// 检查是否为明确的可重试错误
	var retryableErr *RetryableError
	if As(err, &retryableErr) {
		return true
	}

	// 默认根据错误内容判断
	return isRetryableByContent(err)
}

// isRetryableByContent 根据错误内容判断是否可重试
func isRetryableByContent(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// 网络相关错误通常可重试
	networkErrors := []string{
		"connection refused",
		"timeout",
		"network",
		"dial",
		"EOF",
		"broken pipe",
		"no such host",
		"connection reset",
	}

	for _, netErr := range networkErrors {
		if contains(errStr, netErr) {
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
		if contains(errStr, tempErr) {
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
		"unauthorized",
		"forbidden",
		"not found",
		"bad request",
	}

	for _, valErr := range validationErrors {
		if contains(errStr, valErr) {
			return false
		}
	}

	// 默认可重试
	return true
}

// As 检查错误类型（简化版的errors.As）
func As(err error, target interface{}) bool {
	if err == nil {
		return false
	}

	switch target.(type) {
	case **RetryableError:
		if retryableErr, ok := err.(*RetryableError); ok {
			*target.(**RetryableError) = retryableErr
			return true
		}
	case **NonRetryableError:
		if nonRetryableErr, ok := err.(*NonRetryableError); ok {
			*target.(**NonRetryableError) = nonRetryableErr
			return true
		}
	}

	return false
}

// contains 检查字符串是否包含子字符串（不区分大小写）
func contains(s, substr string) bool {
	// 简化实现
	if len(substr) > len(s) {
		return false
	}

	// 转换为小写进行比较
	sLower := toLower(s)
	substrLower := toLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower 转换为小写（简化实现）
func toLower(s string) string {
	result := make([]byte, len(s))
	for i, b := range []byte(s) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32
		} else {
			result[i] = b
		}
	}
	return string(result)
}
