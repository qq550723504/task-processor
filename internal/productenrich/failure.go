package productenrich

import "errors"

type FailureDisposition string

const (
	FailureDispositionRetryable FailureDisposition = "retryable"
	FailureDispositionNoRetry   FailureDisposition = "no_retry"
)

type errNoRetry struct {
	cause error
}

func (e *errNoRetry) Error() string { return e.cause.Error() }
func (e *errNoRetry) Unwrap() error { return e.cause }

func isNoRetryError(err error) bool {
	var e *errNoRetry
	return errors.As(err, &e)
}

func IsNoRetryError(err error) bool {
	return isNoRetryError(err)
}

func NewNoRetryError(err error) error {
	if err == nil {
		return nil
	}
	return &errNoRetry{cause: err}
}

func ClassifyProcessFailure(err error) FailureDisposition {
	if isNoRetryError(err) {
		return FailureDispositionNoRetry
	}
	return FailureDispositionRetryable
}
