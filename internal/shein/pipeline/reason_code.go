package pipeline

import (
	"strings"

	shein "task-processor/internal/shein"
)

func buildTaskStatusErrorMessage(stage string, err error) string {
	if err == nil {
		return ""
	}

	var message string
	switch {
	case shein.IsFilteredError(err):
		message = shein.FormatTaskReasonMessage(shein.TaskReasonFilterRuleRejected, err.Error())
	case shein.IsAuthenticationExpiredError(err):
		message = shein.FormatTaskReasonMessage(shein.TaskReasonAuthExpired, err.Error())
	case isDuplicateSKUError(err):
		message = shein.FormatTaskReasonMessage(shein.TaskReasonSkuDuplicated, err.Error())
	case isCookieLoadError(err):
		message = shein.FormatTaskReasonMessage(shein.TaskReasonCookieLoadFailed, err.Error())
	case shein.IsRetryableError(err):
		message = shein.FormatTaskReasonMessage(shein.TaskReasonRetryableFailure, err.Error())
	default:
		message = shein.FormatTaskReasonMessage(shein.TaskReasonNonRetryableFailure, err.Error())
	}
	return formatStageStatusMessage(stage, message)
}

func isCookieLoadError(err error) bool {
	_, ok := shein.IsCookieLoadError(err)
	return ok
}

func isDuplicateSKUError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "卖家SKU重复")
}
