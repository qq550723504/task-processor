// Package task 提供任务处理相关的错误类型
package task

import "fmt"

// ErrorCode 错误代码
type ErrorCode string

const (
	// 任务相关错误
	ErrCodeInvalidTask      ErrorCode = "INVALID_TASK"
	ErrCodeTaskNotFound     ErrorCode = "TASK_NOT_FOUND"
	ErrCodeDuplicateTask    ErrorCode = "DUPLICATE_TASK"
	ErrCodePlatformMismatch ErrorCode = "PLATFORM_MISMATCH"

	// 处理相关错误
	ErrCodeProcessingFailed ErrorCode = "PROCESSING_FAILED"
	ErrCodeConversionFailed ErrorCode = "CONVERSION_FAILED"
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"

	// 资源相关错误
	ErrCodeStoreNotFound   ErrorCode = "STORE_NOT_FOUND"
	ErrCodeProductNotFound ErrorCode = "PRODUCT_NOT_FOUND"
	ErrCodeAccessDenied    ErrorCode = "ACCESS_DENIED"

	// 网络相关错误
	ErrCodeNetworkError     ErrorCode = "NETWORK_ERROR"
	ErrCodeTimeout          ErrorCode = "TIMEOUT"
	ErrCodeConnectionFailed ErrorCode = "CONNECTION_FAILED"
)

// TaskError 任务处理错误
type TaskError struct {
	Code      ErrorCode
	Message   string
	TaskID    int64
	Operation string
	Err       error
}

// Error 实现 error 接口
func (e *TaskError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] task %d failed during %s: %s (caused by: %v)",
			e.Code, e.TaskID, e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] task %d failed during %s: %s",
		e.Code, e.TaskID, e.Operation, e.Message)
}

// Unwrap 支持错误链
func (e *TaskError) Unwrap() error {
	return e.Err
}

// NewTaskError 创建任务错误
func NewTaskError(code ErrorCode, taskID int64, operation, message string, err error) *TaskError {
	return &TaskError{
		Code:      code,
		TaskID:    taskID,
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}

// IsRetryable 判断错误是否可重试
func (e *TaskError) IsRetryable() bool {
	switch e.Code {
	case ErrCodeNetworkError, ErrCodeTimeout, ErrCodeConnectionFailed:
		return true
	case ErrCodeInvalidTask, ErrCodeProductNotFound, ErrCodeAccessDenied:
		return false
	default:
		return true // 默认可重试
	}
}

// 便捷构造函数

// NewInvalidTaskError 创建无效任务错误
func NewInvalidTaskError(taskID int64, message string) *TaskError {
	return NewTaskError(ErrCodeInvalidTask, taskID, "validation", message, nil)
}

// NewPlatformMismatchError 创建平台不匹配错误
func NewPlatformMismatchError(taskID int64, taskPlatform, processorPlatform string) *TaskError {
	message := fmt.Sprintf("task platform %s does not match processor platform %s",
		taskPlatform, processorPlatform)
	return NewTaskError(ErrCodePlatformMismatch, taskID, "validation", message, nil)
}

// NewProcessingError 创建处理失败错误
func NewProcessingError(taskID int64, operation string, err error) *TaskError {
	return NewTaskError(ErrCodeProcessingFailed, taskID, operation, "processing failed", err)
}

// NewStoreNotFoundError 创建店铺未找到错误
func NewStoreNotFoundError(taskID, storeID int64, err error) *TaskError {
	message := fmt.Sprintf("store %d not found", storeID)
	return NewTaskError(ErrCodeStoreNotFound, taskID, "store_access", message, err)
}

// NewConversionError 创建转换失败错误
func NewConversionError(taskID int64, err error) *TaskError {
	return NewTaskError(ErrCodeConversionFailed, taskID, "conversion", "message conversion failed", err)
}
