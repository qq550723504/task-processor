package commercetool

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolErrorDoesNotExposeCauseInMessage(t *testing.T) {
	err := NewError(ErrorPermissionDenied, "tool permission denied", errors.New("secret database detail"))

	require.Equal(t, "permission_denied: tool permission denied", err.Error())
	require.NotContains(t, err.Error(), "secret database detail")
	require.Equal(t, ErrorPermissionDenied, CodeOf(err))
	require.ErrorContains(t, errors.Unwrap(err), "secret database detail")
}

func TestErrorRetryabilityIsFixedByCode(t *testing.T) {
	require.True(t, IsRetryable(ErrorDeadlineExceeded))
	require.True(t, IsRetryable(ErrorDependencyUnavailable))
	require.False(t, IsRetryable(ErrorPermissionDenied))
	require.False(t, IsRetryable(ErrorUnknownExecutionState))
}

func TestCodeOfFindsWrappedToolError(t *testing.T) {
	err := fmt.Errorf("transport boundary: %w", fmt.Errorf("adapter boundary: %w", NewError(ErrorNotFound, "resource not found", nil)))

	require.Equal(t, ErrorNotFound, CodeOf(err))
}

func TestErrorCodeValuesMatchWireContract(t *testing.T) {
	tests := []struct {
		name  string
		code  ErrorCode
		value string
	}{
		{"invalid input", ErrorInvalidInput, "invalid_input"},
		{"identity integrity", ErrorIdentityIntegrity, "identity_integrity"},
		{"permission denied", ErrorPermissionDenied, "permission_denied"},
		{"tool not allowed", ErrorToolNotAllowed, "tool_not_allowed"},
		{"not found", ErrorNotFound, "not_found"},
		{"failed precondition", ErrorFailedPrecondition, "failed_precondition"},
		{"conflict", ErrorConflict, "conflict"},
		{"deadline exceeded", ErrorDeadlineExceeded, "deadline_exceeded"},
		{"dependency unavailable", ErrorDependencyUnavailable, "dependency_unavailable"},
		{"output invalid", ErrorOutputInvalid, "output_invalid"},
		{"budget exceeded", ErrorBudgetExceeded, "budget_exceeded"},
		{"unknown execution state", ErrorUnknownExecutionState, "unknown_execution_state"},
		{"internal", ErrorInternal, "internal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.value, string(tt.code))
		})
	}
}

func TestNewErrorNormalizesEmptyAndUnknownCodesAtConstruction(t *testing.T) {
	tests := []struct {
		name string
		code ErrorCode
	}{
		{"empty", ""},
		{"unknown", ErrorCode("unrecognized")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.code, "safe message", nil)
			require.IsType(t, &ToolError{}, err)

			toolError := err.(*ToolError)
			require.Equal(t, ErrorInternal, toolError.Code)
		})
	}
}

func TestCodeOfSafelyClassifiesNilOrdinaryAndUnknownErrors(t *testing.T) {
	require.Equal(t, ErrorInternal, CodeOf(nil))
	require.Equal(t, ErrorInternal, CodeOf(errors.New("ordinary error")))
	require.Equal(t, ErrorInternal, CodeOf(&ToolError{Code: ErrorCode("unrecognized")}))
}

func TestIsRetryableRejectsEveryNonRetryableCode(t *testing.T) {
	nonRetryable := []ErrorCode{
		ErrorInvalidInput,
		ErrorIdentityIntegrity,
		ErrorPermissionDenied,
		ErrorToolNotAllowed,
		ErrorNotFound,
		ErrorFailedPrecondition,
		ErrorConflict,
		ErrorOutputInvalid,
		ErrorBudgetExceeded,
		ErrorUnknownExecutionState,
		ErrorInternal,
		ErrorCode("unrecognized"),
		"",
	}

	for _, code := range nonRetryable {
		require.False(t, IsRetryable(code), "code %q must not be retried automatically", code)
	}
}
