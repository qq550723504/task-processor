package amazon

import (
	"fmt"
	"strings"

	browserpkg "task-processor/internal/crawler/amazon/browser"
)

const (
	FetchErrorTypeNone                 = "none"
	FetchErrorTypeInvalidRequest       = "invalid_request"
	FetchErrorTypeProductNotFound      = "product_not_found"
	FetchErrorTypeAuthentication       = "authentication"
	FetchErrorTypeCaptcha              = "captcha"
	FetchErrorTypeBrowserCrash         = "browser_crash"
	FetchErrorTypeServerError          = "server_error"
	FetchErrorTypeNetwork              = "network"
	FetchErrorTypeTimeout              = "timeout"
	FetchErrorTypeProductQuality       = "product_quality"
	FetchErrorTypeCrawlInProgress      = "crawl_in_progress"
	FetchErrorTypeRegionCircuitOpen    = "region_circuit_open"
	FetchErrorTypeSystemBusy           = "system_busy"
	FetchErrorTypeProcessorUnavailable = "processor_unavailable"
	FetchErrorTypeUnknown              = "unknown"
)

var sharedErrorDetector = browserpkg.NewErrorDetector()

type classifiedFetchError interface {
	error
	FetchErrorTypeValue() string
	FetchRetryableValue() bool
}

type typedFetchError struct {
	message   string
	fetchType string
	retryable bool
	cause     error
}

func (e *typedFetchError) Error() string {
	if e == nil {
		return ""
	}
	if e.message != "" {
		return e.message
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.fetchType
}

func (e *typedFetchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *typedFetchError) FetchErrorTypeValue() string {
	if e == nil {
		return FetchErrorTypeUnknown
	}
	return e.fetchType
}

func (e *typedFetchError) FetchRetryableValue() bool {
	if e == nil {
		return false
	}
	return e.retryable
}

func newTypedFetchError(fetchType string, retryable bool, message string, cause error) error {
	return &typedFetchError{
		message:   message,
		fetchType: fetchType,
		retryable: retryable,
		cause:     cause,
	}
}

func newInvalidRequestError(message string) error {
	return newTypedFetchError(FetchErrorTypeInvalidRequest, false, message, nil)
}

func newProcessorUnavailableError(message string, cause error) error {
	return newTypedFetchError(FetchErrorTypeProcessorUnavailable, true, message, cause)
}

func newCrawlInProgressError() error {
	return newTypedFetchError(FetchErrorTypeCrawlInProgress, true, "crawl already in progress and shared result timed out", nil)
}

func newSystemBusyError(message string, cause error) error {
	return newTypedFetchError(FetchErrorTypeSystemBusy, true, message, cause)
}

type FetchError struct {
	Type      string
	Retryable bool
	Cause     error
}

func (e *FetchError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Type
	}
	return fmt.Sprintf("%s: %v", e.Type, e.Cause)
}

func (e *FetchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *FetchError) ErrorType() string {
	if e == nil || e.Type == "" {
		return FetchErrorTypeUnknown
	}
	return e.Type
}

func (e *FetchError) RetryableError() bool {
	if e == nil {
		return false
	}
	return e.Retryable
}

func ClassifyFetchError(err error) *FetchError {
	if err == nil {
		return nil
	}

	if existing, ok := err.(*FetchError); ok {
		return existing
	}
	if existing, ok := err.(classifiedFetchError); ok {
		return &FetchError{
			Type:      existing.FetchErrorTypeValue(),
			Retryable: existing.FetchRetryableValue(),
			Cause:     err,
		}
	}

	if isProcessorUnavailableError(err) {
		return &FetchError{
			Type:      FetchErrorTypeProcessorUnavailable,
			Retryable: true,
			Cause:     err,
		}
	}

	if isInvalidRequestError(err) {
		return &FetchError{
			Type:      FetchErrorTypeInvalidRequest,
			Retryable: false,
			Cause:     err,
		}
	}

	if isProductQualityError(err) {
		return &FetchError{
			Type:      FetchErrorTypeProductQuality,
			Retryable: true,
			Cause:     err,
		}
	}

	if strings.Contains(strings.ToLower(err.Error()), "crawl already in progress and shared result timed out") {
		return &FetchError{
			Type:      FetchErrorTypeCrawlInProgress,
			Retryable: true,
			Cause:     err,
		}
	}
	if strings.Contains(strings.ToLower(err.Error()), "crawler concurrency limit exceeded") ||
		strings.Contains(strings.ToLower(err.Error()), "crawler concurrency acquire timeout") {
		return &FetchError{
			Type:      FetchErrorTypeSystemBusy,
			Retryable: true,
			Cause:     err,
		}
	}

	errorType := sharedErrorDetector.GetErrorType(err)
	if errorType == "" || errorType == FetchErrorTypeNone {
		errorType = FetchErrorTypeUnknown
	}

	return &FetchError{
		Type:      errorType,
		Retryable: sharedErrorDetector.ShouldRetry(err),
		Cause:     err,
	}
}

func isInvalidRequestError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	patterns := []string{
		"url or asin is required",
		"amazon crawler is not initialized",
	}
	for _, pattern := range patterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}
	return false
}

func isProcessorUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	patterns := []string{
		"amazon处理器不可用",
		"初始化浏览器池失败",
	}
	for _, pattern := range patterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}
	return false
}
