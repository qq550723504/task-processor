package amazon

import (
	"fmt"
	"strings"

	browserpkg "task-processor/internal/crawler/amazon/browser"
)

const (
	FetchErrorTypeNone              = "none"
	FetchErrorTypeInvalidRequest    = "invalid_request"
	FetchErrorTypeProductNotFound   = "product_not_found"
	FetchErrorTypeAuthentication    = "authentication"
	FetchErrorTypeCaptcha           = "captcha"
	FetchErrorTypeBrowserCrash      = "browser_crash"
	FetchErrorTypeServerError       = "server_error"
	FetchErrorTypeNetwork           = "network"
	FetchErrorTypeTimeout           = "timeout"
	FetchErrorTypeProductQuality    = "product_quality"
	FetchErrorTypeCrawlInProgress   = "crawl_in_progress"
	FetchErrorTypeRegionCircuitOpen = "region_circuit_open"
	FetchErrorTypeUnknown           = "unknown"
)

var sharedErrorDetector = browserpkg.NewErrorDetector()

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
