package alibaba1688

import (
	"errors"
	"fmt"
)

type PublicAccessFailureKind string

const (
	PublicAccessFailureBrowser       PublicAccessFailureKind = "browser"
	PublicAccessFailureChallenge     PublicAccessFailureKind = "challenge"
	PublicAccessFailureMissingFields PublicAccessFailureKind = "missing_fields"
	PublicAccessFailureValidation    PublicAccessFailureKind = "validation"
	PublicAccessFailureInvalidURL    PublicAccessFailureKind = "invalid_url"
	PublicAccessFailureTransport     PublicAccessFailureKind = "transport"
)

type PublicAccessError struct {
	Kind PublicAccessFailureKind
	Err  error
}

func NewPublicAccessError(kind PublicAccessFailureKind, err error) error {
	return &PublicAccessError{Kind: kind, Err: err}
}

func (e *PublicAccessError) Error() string {
	if e == nil || e.Err == nil {
		return "public 1688 access failed"
	}
	return e.Err.Error()
}

func (e *PublicAccessError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsAccountFallbackEligible(err error) bool {
	var accessErr *PublicAccessError
	if !errors.As(err, &accessErr) || accessErr == nil {
		return false
	}
	return accessErr.Kind == PublicAccessFailureChallenge || accessErr.Kind == PublicAccessFailureMissingFields
}

type sourceAccessError struct {
	code    string
	message string
}

func (e *sourceAccessError) Error() string {
	if e == nil || e.message == "" {
		return "public 1688 source is unavailable"
	}
	return e.message
}

func newSourceAccessError(code string) error {
	message := "public 1688 source is unavailable"
	if code == "source_account_disabled" {
		message = "source account is disabled"
	}
	if code == "source_account_unavailable" {
		message = "source account is unavailable"
	}
	return &sourceAccessError{code: code, message: message}
}

func sourceAccessErrorCode(err error) string {
	var accessErr *sourceAccessError
	if !errors.As(err, &accessErr) || accessErr == nil {
		return ""
	}
	return accessErr.code
}

func newPublicUnavailableError() error {
	return newSourceAccessError("source_public_unavailable")
}

func newAccountUnavailableError() error {
	return newSourceAccessError("source_account_unavailable")
}

func newAccountDisabledError() error {
	return newSourceAccessError("source_account_disabled")
}

func sourceFallbackReason(err error) string {
	var accessErr *PublicAccessError
	if !errors.As(err, &accessErr) || accessErr == nil {
		return ""
	}
	if accessErr.Kind == "" {
		return ""
	}
	return fmt.Sprintf("public_%s", accessErr.Kind)
}
