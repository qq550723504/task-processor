package shein

import (
	"fmt"
	"strings"
)

const (
	TaskReasonDraftSavedValidationFailed = "DRAFT_SAVED_VALIDATION_FAILED"
	TaskReasonFilterRuleRejected         = "FILTER_RULE_REJECTED"
	TaskReasonAuthExpired                = "AUTH_EXPIRED"
	TaskReasonCookieLoadFailed           = "COOKIE_LOAD_FAILED"
	TaskReasonDailyLimitReached          = "DAILY_LIMIT_REACHED"
	TaskReasonShelfQuotaExhausted        = "SHELF_QUOTA_EXHAUSTED"
	TaskReasonSkuDuplicated              = "SKU_DUPLICATED"
	TaskReasonRetryableFailure           = "RETRYABLE_FAILURE"
	TaskReasonNonRetryableFailure        = "NON_RETRYABLE_FAILURE"
)

// FormatTaskReasonMessage prefixes a stable reason code while preserving the original message.
func FormatTaskReasonMessage(reasonCode, message string) string {
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

func FormatTaskStageMessage(stage, message string) string {
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
