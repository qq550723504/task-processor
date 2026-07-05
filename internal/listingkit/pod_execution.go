package listingkit

import (
	"strings"
	"time"
)

const (
	podProviderSDS = "sds"

	podDependencyModeRequired = "required"
	podDependencyModeOptional = "optional"
	podDependencyModeDisabled = "disabled"

	podStatusNotApplicable  = "not_applicable"
	podStatusPending        = "pending"
	podStatusProcessing     = "processing"
	podStatusSucceeded      = "succeeded"
	podStatusFailedBlocking = "failed_blocking"
	podStatusFailedDegraded = "failed_degraded"
	podStatusBypassed       = "bypassed"

	podFallbackLocalMockup = "local_mockup"

	podAuditKindPolicyDecision  = "policy_decision"
	podAuditKindStatusChange    = "status_transition"
	podAuditCodePolicyApplied   = "pod_policy_applied"
	podAuditCodeStatusChanged   = "pod_status_changed"
	podAuditHistoryMaxEventSize = 20
)

func normalizePodExecutionSummary(pod *PodExecutionSummary) *PodExecutionSummary {
	if pod == nil {
		return nil
	}
	pod.Provider = strings.ToLower(strings.TrimSpace(pod.Provider))
	pod.DependencyMode = strings.ToLower(strings.TrimSpace(pod.DependencyMode))
	pod.Status = strings.ToLower(strings.TrimSpace(pod.Status))
	pod.FailureReason = strings.TrimSpace(pod.FailureReason)
	pod.FallbackType = strings.ToLower(strings.TrimSpace(pod.FallbackType))
	pod.DecisionSource = strings.TrimSpace(pod.DecisionSource)
	pod.History = normalizePodExecutionAuditHistory(pod.History)
	if pod.DecisionSource == "" {
		pod.DecisionSource = "system_rule"
	}
	if pod.DependencyMode == "" {
		pod.DependencyMode = podDependencyModeDisabled
	}
	if pod.Status == "" {
		switch pod.DependencyMode {
		case podDependencyModeDisabled:
			pod.Status = podStatusNotApplicable
		default:
			pod.Status = podStatusPending
		}
	}
	return pod
}

func clonePodExecutionSummary(pod *PodExecutionSummary) *PodExecutionSummary {
	if pod == nil {
		return nil
	}
	copy := *pod
	copy.History = clonePodExecutionAuditHistory(pod.History)
	return &copy
}

func shouldUsePODPlatform(req *GenerateRequest) bool {
	return shouldRunRemoteSDSDesignSync(req) || shouldRunImageResultSDSDesignSync(req)
}

func shouldRunImageResultSDSDesignSync(req *GenerateRequest) bool {
	return shouldSyncSDS(req) && shouldProcessImages(req)
}

func inferPODProvider(req *GenerateRequest) string {
	return determinePODExecutionPolicy(req).Provider
}

func inferPODDependencyMode(req *GenerateRequest) string {
	return determinePODExecutionPolicy(req).DependencyMode
}

func ensureTaskPodExecution(task *Task) bool {
	if task == nil {
		return false
	}
	if task.Result == nil {
		task.Result = &ListingKitResult{}
	}
	return ensureResultPodExecution(task.Result, task.Request)
}

func ensureResultPodExecution(result *ListingKitResult, req *GenerateRequest) bool {
	result = normalizeListingKitResultSemanticFields(result)
	if result == nil {
		return false
	}
	before := clonePodExecutionSummary(result.PodExecution)
	result.PodExecution = derivePodExecutionSummary(result.PodExecution, result.SDSDesignResult, result.ChildTasks, req, result.UpdatedAt)
	recordPodExecutionAudit(before, result.PodExecution, result.UpdatedAt)
	if result.StandardProductSnapshot != nil {
		result.StandardProductSnapshot.PodExecution = clonePodExecutionSummary(result.PodExecution)
	}
	return !podExecutionEqual(before, result.PodExecution)
}

func derivePodExecutionSummary(current *PodExecutionSummary, sds *SDSSyncSummary, childTasks []ChildTaskState, req *GenerateRequest, updatedAt time.Time) *PodExecutionSummary {
	pod := clonePodExecutionSummary(current)
	if pod == nil {
		pod = &PodExecutionSummary{}
	}
	if pod.Provider == "" {
		pod.Provider = determinePODExecutionPolicy(req).Provider
	}
	if pod.DependencyMode == "" {
		pod.DependencyMode = determinePODExecutionPolicy(req).DependencyMode
	}
	if strings.TrimSpace(pod.DecisionSource) == "" {
		pod.DecisionSource = determinePODExecutionPolicy(req).DecisionSource
	}
	if pod.Provider == "" || pod.DependencyMode == podDependencyModeDisabled {
		pod.Provider = ""
		pod.DependencyMode = podDependencyModeDisabled
		pod.Status = podStatusNotApplicable
		pod.FailureReason = ""
		pod.FallbackType = ""
		pod.CompletedAt = nil
		return normalizePodExecutionSummary(pod)
	}

	status, reason, fallback, found := inferPodStatusFromSDS(sds, childTasks, pod.DependencyMode)
	if found && status != "" {
		pod.Status = status
	}
	if found {
		pod.FailureReason = reason
	}
	if found {
		pod.FallbackType = fallback
	}
	if !found && strings.TrimSpace(pod.Status) == "" {
		pod.Status = podStatusPending
	}
	if pod.LastAttemptAt == nil && pod.Status != podStatusPending {
		at := updatedAt
		if at.IsZero() {
			at = time.Now()
		}
		pod.LastAttemptAt = &at
	}
	if pod.CompletedAt == nil && pod.Status == podStatusSucceeded {
		at := updatedAt
		if at.IsZero() {
			at = time.Now()
		}
		pod.CompletedAt = &at
	}
	if pod.Status != podStatusSucceeded {
		pod.CompletedAt = nil
	}
	return normalizePodExecutionSummary(pod)
}

func markPodExecutionStatus(result *ListingKitResult, status string, at time.Time) {
	result = normalizeListingKitResultSemanticFields(result)
	if result == nil || result.PodExecution == nil {
		return
	}
	before := clonePodExecutionSummary(result.PodExecution)
	result.PodExecution.Status = strings.ToLower(strings.TrimSpace(status))
	clearPodExecutionFailureDetailsForStatus(result.PodExecution)
	if at.IsZero() {
		at = time.Now()
	}
	result.PodExecution.LastAttemptAt = &at
	if result.PodExecution.Status == podStatusSucceeded {
		result.PodExecution.CompletedAt = &at
	} else {
		result.PodExecution.CompletedAt = nil
	}
	result.PodExecution = normalizePodExecutionSummary(result.PodExecution)
	recordPodExecutionAudit(before, result.PodExecution, at)
	if result.StandardProductSnapshot != nil {
		result.StandardProductSnapshot.PodExecution = clonePodExecutionSummary(result.PodExecution)
	}
}

func clearPodExecutionFailureDetailsForStatus(pod *PodExecutionSummary) {
	if pod == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(pod.Status)) {
	case podStatusFailedBlocking, podStatusFailedDegraded:
		return
	default:
		pod.FailureReason = ""
		pod.FallbackType = ""
	}
}
