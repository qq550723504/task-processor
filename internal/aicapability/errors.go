package aicapability

import "errors"

type ErrorCategory string

const (
	ErrorInvalidInput            ErrorCategory = "invalid_input"
	ErrorPolicyDenied            ErrorCategory = "policy_denied"
	ErrorCapabilityUnavailable   ErrorCategory = "capability_unavailable"
	ErrorCredentialUnavailable   ErrorCategory = "credential_unavailable"
	ErrorRateLimited             ErrorCategory = "rate_limited"
	ErrorProviderTimeout         ErrorCategory = "provider_timeout"
	ErrorProviderUnavailable     ErrorCategory = "provider_unavailable"
	ErrorProviderRejected        ErrorCategory = "provider_rejected"
	ErrorInvalidProviderResponse ErrorCategory = "invalid_provider_response"
	ErrorStructuredOutputInvalid ErrorCategory = "structured_output_invalid"
	ErrorBudgetExceeded          ErrorCategory = "budget_exceeded"
	ErrorAgentStepLimitExceeded  ErrorCategory = "agent_step_limit_exceeded"
	ErrorAgentToolDenied         ErrorCategory = "agent_tool_denied"
	ErrorUnknownRemoteState      ErrorCategory = "unknown_remote_state"
	ErrorUnknown                 ErrorCategory = "unknown"
)

type CapabilityError struct {
	Category  ErrorCategory
	Operation string
	Err       error
}

func (e *CapabilityError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Category)
	}
	return string(e.Category) + ": " + e.Err.Error()
}

func (e *CapabilityError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewError(category ErrorCategory, operation string, err error) error {
	return &CapabilityError{Category: category, Operation: operation, Err: err}
}

func CategoryOf(err error) ErrorCategory {
	if err == nil {
		return ""
	}
	var capabilityErr *CapabilityError
	if errors.As(err, &capabilityErr) && capabilityErr.Category != "" {
		return capabilityErr.Category
	}
	return ErrorUnknown
}
