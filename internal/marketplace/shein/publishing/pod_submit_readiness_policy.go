package publishing

import (
	"strings"

	sdspod "task-processor/internal/product/sourcing/sdspod"
)

const saveDraftPODReadinessFallback = "POD 平台处理尚未完成；当前允许先保存草稿，正式发布前仍需确认平台结果"

// PODSubmitReadiness is the SHEIN submit-readiness outcome for a POD execution.
type PODSubmitReadiness struct {
	Applicable  bool
	Ready       bool
	WarningOnly bool
	Message     string
}

// EvaluatePODSubmitReadiness maps normalized POD execution facts to a SHEIN submit-readiness decision.
func EvaluatePODSubmitReadiness(action string, execution sdspod.Execution) PODSubmitReadiness {
	execution = sdspod.NormalizeExecution(execution)
	if execution.DependencyMode == sdspod.DependencyDisabled {
		return PODSubmitReadiness{}
	}

	allowsBlockers := SubmitActionAllowsReadinessBlockers(action)
	blocked := sdspod.SubmissionBlocked(execution)
	succeeded := execution.Status == sdspod.StatusSucceeded
	message := sdspod.ReadinessMessage(execution)
	if allowsBlockers && !succeeded {
		message = firstNonEmptyPODReadinessMessage(message, saveDraftPODReadinessFallback)
	}

	return PODSubmitReadiness{
		Applicable:  true,
		Ready:       !blocked && (allowsBlockers || (execution.Status != sdspod.StatusFailedDegraded && execution.Status != sdspod.StatusBypassed)),
		WarningOnly: allowsBlockers || !blocked,
		Message:     message,
	}
}

func firstNonEmptyPODReadinessMessage(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
