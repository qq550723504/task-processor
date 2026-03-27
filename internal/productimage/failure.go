package productimage

import "errors"

type FailureDisposition string

const (
	FailureDispositionRetryable FailureDisposition = "retryable"
	FailureDispositionNoRetry   FailureDisposition = "no_retry"
)

type errNoRetry struct {
	cause error
}

func (e *errNoRetry) Error() string {
	if e == nil || e.cause == nil {
		return "non-retryable error"
	}
	return e.cause.Error()
}

func (e *errNoRetry) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func NewNoRetryError(err error) error {
	if err == nil {
		return &errNoRetry{}
	}
	return &errNoRetry{cause: err}
}

func IsNoRetryError(err error) bool {
	var target *errNoRetry
	return errors.As(err, &target)
}

func ClassifyProcessFailure(err error) FailureDisposition {
	if IsNoRetryError(err) {
		return FailureDispositionNoRetry
	}
	return FailureDispositionRetryable
}
