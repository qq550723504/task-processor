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

func TestNewErrorNormalizesAndClassifiesEveryDefinedCode(t *testing.T) {
	tests := []struct {
		name string
		code ErrorCode
	}{
		{"invalid input", ErrorInvalidInput},
		{"identity integrity", ErrorIdentityIntegrity},
		{"permission denied", ErrorPermissionDenied},
		{"tool not allowed", ErrorToolNotAllowed},
		{"not found", ErrorNotFound},
		{"failed precondition", ErrorFailedPrecondition},
		{"conflict", ErrorConflict},
		{"deadline exceeded", ErrorDeadlineExceeded},
		{"dependency unavailable", ErrorDependencyUnavailable},
		{"output invalid", ErrorOutputInvalid},
		{"budget exceeded", ErrorBudgetExceeded},
		{"unknown execution state", ErrorUnknownExecutionState},
		{"internal", ErrorInternal},
		{"empty", ""},
		{"unknown", ErrorCode("unrecognized")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.code
			if tt.code == "" || tt.code == "unrecognized" {
				want = ErrorInternal
			}

			require.Equal(t, want, CodeOf(NewError(tt.code, "safe message", nil)))
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
