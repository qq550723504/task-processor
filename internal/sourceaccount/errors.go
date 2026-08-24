package sourceaccount

import "errors"

const (
	SourceAccountUnavailable = "source_account_unavailable"
	SourceAccountDisabled    = "source_account_disabled"
)

type accessError struct {
	code string
}

func (e *accessError) Error() string {
	if e == nil {
		return "source account is unavailable"
	}
	if e.code == SourceAccountDisabled {
		return "source account is disabled"
	}
	return "source account is unavailable"
}

func NewUnavailableError(_ string) error {
	return &accessError{code: SourceAccountUnavailable}
}

func NewDisabledError() error {
	return &accessError{code: SourceAccountDisabled}
}

func ErrorCode(err error) string {
	var accessErr *accessError
	if !errors.As(err, &accessErr) || accessErr == nil {
		return ""
	}
	return accessErr.code
}
