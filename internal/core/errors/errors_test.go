// Package errors provides unit tests for application errors
package errors

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAppError_New tests creating a new error
func TestAppError_New(t *testing.T) {
	err := New(ErrCodeSystem, "system error")
	
	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeSystem, err.Code)
	assert.Equal(t, "system error", err.Message)
	assert.NotZero(t, err.Timestamp)
}

// TestAppError_Newf tests formatted error creation
func TestAppError_Newf(t *testing.T) {
	err := Newf(ErrCodeConfig, "config error: %s", "missing field")
	
	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeConfig, err.Code)
	assert.Contains(t, err.Message, "config error: missing field")
}

// TestAppError_Wrap tests wrapping errors
func TestAppError_Wrap(t *testing.T) {
	original := errors.New("original error")
	err := Wrap(original, ErrCodeNetwork, "network failed")
	
	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeNetwork, err.Code)
	assert.Equal(t, "network failed", err.Message)
	assert.Equal(t, original, err.Cause)
}

// TestAppError_Wrapf tests formatted wrap
func TestAppError_Wrapf(t *testing.T) {
	original := errors.New("original error")
	err := Wrapf(original, ErrCodeAuth, "auth failed: %s", "token expired")
	
	assert.NotNil(t, err)
	assert.Contains(t, err.Message, "auth failed: token expired")
	assert.Equal(t, original, err.Cause)
}

// TestAppError_Error tests error message format
func TestAppError_Error(t *testing.T) {
	err := &AppError{
		Code:    ErrCodeTaskNotFound,
		Message: "task not found",
		Cause:   errors.New("db error"),
	}
	
	errStr := err.Error()
	assert.Contains(t, errStr, "TASK_NOT_FOUND")
	assert.Contains(t, errStr, "task not found")
	assert.Contains(t, errStr, "db error")
}

// TestAppError_Is tests error matching
func TestAppError_Is(t *testing.T) {
	err1 := New(ErrCodePlatformError, "platform error")
	err2 := New(ErrCodePlatformError, "another platform error")
	err3 := New(ErrCodeTaskNotFound, "task not found")
	
	assert.True(t, errors.Is(err1, New(ErrCodePlatformError, "")))
	assert.True(t, errors.Is(err1, err2))
	assert.False(t, errors.Is(err1, err3))
}

// TestAppError_Unwrap tests error unwrapping
func TestAppError_Unwrap(t *testing.T) {
	original := errors.New("original")
	err := Wrap(original, ErrCodeSystem, "wrapped")
	
	unwrapped := errors.Unwrap(err)
	assert.Equal(t, original, unwrapped)
}

// TestAppError_WithDetails tests adding details
func TestAppError_WithDetails(t *testing.T) {
	err := New(ErrCodeValidation, "validation failed").
		WithDetails("field 'name' is required")
	
	assert.Equal(t, "validation failed", err.Message)
	assert.Equal(t, "field 'name' is required", err.Details)
}

// TestAppError_WithFileLine tests adding file and line
func TestAppError_WithFileLine(t *testing.T) {
	err := New(ErrCodeSystem, "error").
		WithFileLine("main.go", 42)
	
	assert.Equal(t, "main.go", err.File)
	assert.Equal(t, 42, err.Line)
}

// TestErrorCodes tests error code constants
func TestErrorCodes(t *testing.T) {
	assert.Equal(t, ErrorCode("SYSTEM_ERROR"), ErrCodeSystem)
	assert.Equal(t, ErrorCode("CONFIG_ERROR"), ErrCodeConfig)
	assert.Equal(t, ErrorCode("AUTH_ERROR"), ErrCodeAuth)
	assert.Equal(t, ErrorCode("NETWORK_ERROR"), ErrCodeNetwork)
	assert.Equal(t, ErrorCode("TIMEOUT_ERROR"), ErrCodeTimeout)
	assert.Equal(t, ErrorCode("RESOURCE_LIMIT_ERROR"), ErrCodeResourceLimit)
	assert.Equal(t, ErrorCode("TASK_NOT_FOUND"), ErrCodeTaskNotFound)
	assert.Equal(t, ErrorCode("TASK_DUPLICATE"), ErrCodeTaskDuplicate)
	assert.Equal(t, ErrorCode("TASK_PROCESSING"), ErrCodeTaskProcessing)
	assert.Equal(t, ErrorCode("PLATFORM_ERROR"), ErrCodePlatformError)
	assert.Equal(t, ErrorCode("VALIDATION_ERROR"), ErrCodeValidation)
	assert.Equal(t, ErrorCode("EXTERNAL_API_ERROR"), ErrCodeExternalAPI)
	assert.Equal(t, ErrorCode("AMAZON_API_ERROR"), ErrCodeAmazonAPI)
	assert.Equal(t, ErrorCode("MANAGEMENT_API_ERROR"), ErrCodeManagementAPI)
	assert.Equal(t, ErrorCode("OPENAI_API_ERROR"), ErrCodeOpenAIAPI)
}

// TestAppError_Timestamp tests timestamp setting
func TestAppError_Timestamp(t *testing.T) {
	before := time.Now()
	err := New(ErrCodeSystem, "test error")
	after := time.Now()
	
	assert.True(t, err.Timestamp.After(before) || err.Timestamp.Equal(before))
	assert.True(t, err.Timestamp.Before(after) || err.Timestamp.Equal(after))
}

// TestAppError_ErrorWithoutCause tests error message without cause
func TestAppError_ErrorWithoutCause(t *testing.T) {
	err := &AppError{
		Code:    ErrCodeConfig,
		Message: "config missing",
	}
	
	errStr := err.Error()
	assert.Contains(t, errStr, "CONFIG_ERROR")
	assert.Contains(t, errStr, "config missing")
	assert.NotContains(t, errStr, ":")
}
