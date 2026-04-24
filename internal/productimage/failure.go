package productimage

import (
	"errors"
	"strings"
)

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

type providerFailureReason interface {
	FailureReason() string
}

func isNoRetryProviderError(err error) bool {
	var providerErr providerFailureReason
	if !errors.As(err, &providerErr) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(providerErr.FailureReason())) {
	case "input_moderation", "output_moderation":
		return true
	default:
		return false
	}
}

func isNoRetryMessage(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}

	noRetrySubstrings := []string{
		"apikey error",
		"api key error",
		"invalid api key",
		"unauthorized",
		"forbidden",
		"insufficient balance",
		"insufficient quota",
		"quota exceeded",
		"insufficient credit",
		"billing",
	}
	for _, token := range noRetrySubstrings {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}

func ClassifyProcessFailure(err error) FailureDisposition {
	if IsNoRetryError(err) || isNoRetryProviderError(err) || isNoRetryMessage(err) {
		return FailureDispositionNoRetry
	}
	return FailureDispositionRetryable
}
