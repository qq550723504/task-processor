package pipeline

import (
	"errors"
	"fmt"
	"strings"

	amazonModel "task-processor/internal/amazon/model"
)

const (
	amazonTaskReasonAuthExpired       = "AUTH_EXPIRED"
	amazonTaskReasonDailyLimitReached = "DAILY_LIMIT_REACHED"
	amazonTaskReasonStoreNotFound     = "STORE_NOT_FOUND"
	amazonTaskReasonProductNotFound   = "PRODUCT_NOT_FOUND"
	amazonTaskReasonRateLimit         = "RATE_LIMIT"
	amazonTaskReasonValidationFailed  = "VALIDATION_FAILED"
	amazonTaskReasonRetryableFailure  = "RETRYABLE_FAILURE"
	amazonTaskReasonNonRetryable      = "NON_RETRYABLE_FAILURE"
	amazonTaskStageParseSourceData    = "parse_source_data"
	amazonTaskStageValidateInput      = "validate_input"
	amazonTaskStageInitStore          = "init_store"
	amazonTaskStageCheckDailyLimit    = "check_daily_limit"
	amazonTaskStageResolveProductType = "resolve_product_type"
	amazonTaskStageMapAttributes      = "map_attributes"
	amazonTaskStageLoadProductData    = "load_product_data"
	amazonTaskStageProcessImages      = "process_images"
	amazonTaskStageBuildVariants      = "build_variants"
	amazonTaskStageCreateListing      = "create_listing"
	amazonTaskStageSetPricing         = "set_pricing"
	amazonTaskStageSetInventory       = "set_inventory"
)

func normalizeTaskStatusError(stage string, err error) error {
	if err == nil {
		return nil
	}

	message := strings.TrimSpace(err.Error())
	reasonCode, retryable := classifyAmazonTaskError(err)

	if reasonCode != "" && !strings.Contains(message, "["+reasonCode+"]") {
		message = formatAmazonTaskReasonMessage(reasonCode, message)
	}
	if stage != "" && !strings.Contains(message, "[stage:") {
		message = formatAmazonTaskStageMessage(stage, message)
	}
	if !retryable && !hasStatusPrefix(message, "NONRETRYABLE:") && !hasStatusPrefix(message, "TERMINATED:") {
		message = "NONRETRYABLE: " + message
	}

	return errors.New(message)
}

func newNonRetryableStageError(stage string, reasonCode string, format string, args ...any) error {
	return normalizeTaskStatusError(stage, fmt.Errorf(format, args...))
}

func classifyAmazonTaskError(err error) (reasonCode string, retryable bool) {
	if err == nil {
		return "", false
	}

	lowerMessage := strings.ToLower(err.Error())

	switch {
	case strings.Contains(lowerMessage, "daily_limit_reached") || strings.Contains(lowerMessage, "每日上架限额"):
		return amazonTaskReasonDailyLimitReached, false
	case isAmazonAuthError(err, lowerMessage):
		return amazonTaskReasonAuthExpired, false
	case strings.Contains(lowerMessage, "店铺不存在"):
		return amazonTaskReasonStoreNotFound, false
	case errors.Is(err, amazonModel.ErrProductNotFound):
		return amazonTaskReasonProductNotFound, false
	case isAmazonRateLimitError(err):
		return amazonTaskReasonRateLimit, true
	case amazonModel.IsValidationError(err):
		return amazonTaskReasonValidationFailed, false
	case amazonModel.IsRetryableError(err):
		return amazonTaskReasonRetryableFailure, true
	default:
		return amazonTaskReasonNonRetryable, false
	}
}

func isAmazonAuthError(err error, lowerMessage string) bool {
	if errors.Is(err, amazonModel.ErrAuthenticationFailed) {
		return true
	}

	var amazonErr *amazonModel.AmazonError
	if errors.As(err, &amazonErr) {
		return amazonErr.Code == amazonModel.ErrorCodeUnauthorized || amazonErr.Code == amazonModel.ErrorCodeForbidden
	}

	authMarkers := []string{
		"auth expired",
		"authentication failed",
		"unauthorized",
		"forbidden",
	}
	for _, marker := range authMarkers {
		if strings.Contains(lowerMessage, marker) {
			return true
		}
	}
	return false
}

func isAmazonRateLimitError(err error) bool {
	if errors.Is(err, amazonModel.ErrAPIRateLimit) {
		return true
	}

	var amazonErr *amazonModel.AmazonError
	if errors.As(err, &amazonErr) {
		return amazonErr.Code == amazonModel.ErrorCodeRateLimit
	}

	return false
}

func formatAmazonTaskReasonMessage(reasonCode string, message string) string {
	reasonCode = strings.TrimSpace(reasonCode)
	message = strings.TrimSpace(message)
	if reasonCode == "" {
		return message
	}
	if message == "" {
		return fmt.Sprintf("[%s]", reasonCode)
	}
	return fmt.Sprintf("[%s] %s", reasonCode, message)
}

func formatAmazonTaskStageMessage(stage string, message string) string {
	stage = strings.TrimSpace(stage)
	message = strings.TrimSpace(message)
	if stage == "" {
		return message
	}
	if message == "" {
		return fmt.Sprintf("[stage:%s]", stage)
	}
	return fmt.Sprintf("[stage:%s] %s", stage, message)
}

func hasStatusPrefix(message string, prefix string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(message)), strings.ToUpper(prefix))
}
