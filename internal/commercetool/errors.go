package commercetool

import (
	"errors"
)

type ErrorCode string

const (
	ErrorInvalidInput          ErrorCode = "invalid_input"
	ErrorIdentityIntegrity     ErrorCode = "identity_integrity"
	ErrorPermissionDenied      ErrorCode = "permission_denied"
	ErrorToolNotAllowed        ErrorCode = "tool_not_allowed"
	ErrorNotFound              ErrorCode = "not_found"
	ErrorFailedPrecondition    ErrorCode = "failed_precondition"
	ErrorConflict              ErrorCode = "conflict"
	ErrorDeadlineExceeded      ErrorCode = "deadline_exceeded"
	ErrorDependencyUnavailable ErrorCode = "dependency_unavailable"
	ErrorOutputInvalid         ErrorCode = "output_invalid"
	ErrorBudgetExceeded        ErrorCode = "budget_exceeded"
	ErrorUnknownExecutionState ErrorCode = "unknown_execution_state"
	ErrorInternal              ErrorCode = "internal"
)

// ToolError is a safe error boundary for deterministic tool execution.
type ToolError struct {
	Code        ErrorCode
	SafeMessage string
	cause       error
}

func NewError(code ErrorCode, safeMessage string, cause error) error {
	return &ToolError{
		Code:        normalizeErrorCode(code),
		SafeMessage: safeMessage,
		cause:       cause,
	}
}

func (err *ToolError) Error() string {
	if err == nil {
		return string(ErrorInternal)
	}

	return string(normalizeErrorCode(err.Code)) + ": " + err.SafeMessage
}

func (err *ToolError) Unwrap() error {
	if err == nil {
		return nil
	}

	return err.cause
}

func CodeOf(err error) ErrorCode {
	var toolError *ToolError
	if errors.As(err, &toolError) && toolError != nil {
		return normalizeErrorCode(toolError.Code)
	}

	return ErrorInternal
}

func IsRetryable(code ErrorCode) bool {
	switch code {
	case ErrorDeadlineExceeded, ErrorDependencyUnavailable:
		return true
	default:
		return false
	}
}

func normalizeErrorCode(code ErrorCode) ErrorCode {
	switch code {
	case ErrorInvalidInput,
		ErrorIdentityIntegrity,
		ErrorPermissionDenied,
		ErrorToolNotAllowed,
		ErrorNotFound,
		ErrorFailedPrecondition,
		ErrorConflict,
		ErrorDeadlineExceeded,
		ErrorDependencyUnavailable,
		ErrorOutputInvalid,
		ErrorBudgetExceeded,
		ErrorUnknownExecutionState,
		ErrorInternal:
		return code
	default:
		return ErrorInternal
	}
}
