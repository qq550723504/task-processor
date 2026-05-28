package listingkit

import (
	"fmt"
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
	return shouldSyncSDS(req) || shouldRunRemoteSDSDesignSync(req)
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
	if found && reason != "" {
		pod.FailureReason = reason
	}
	if found && fallback != "" {
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

func inferPodStatusFromSDS(sds *SDSSyncSummary, childTasks []ChildTaskState, dependencyMode string) (status string, reason string, fallback string, found bool) {
	if sds != nil {
		status, fallback = mapSDSStatusToPODStatus(sds.Status, dependencyMode)
		reason = strings.TrimSpace(sds.Error)
		return status, reason, fallback, true
	}
	for _, child := range childTasks {
		if child.Kind != "sds_design_sync" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(child.Status)) {
		case strings.ToLower(string(TaskStatusProcessing)):
			return podStatusProcessing, "", "", true
		case strings.ToLower(string(TaskStatusCompleted)):
			return podStatusSucceeded, "", "", true
		case strings.ToLower(string(TaskStatusFailed)):
			return podFailureStatusForMode(dependencyMode), strings.TrimSpace(child.Error), "", true
		case strings.ToLower(string(TaskStatusPending)):
			return podStatusPending, "", "", true
		}
	}
	return "", "", "", false
}

func mapSDSStatusToPODStatus(sdsStatus string, dependencyMode string) (status string, fallback string) {
	switch strings.ToLower(strings.TrimSpace(sdsStatus)) {
	case "", "pending":
		return podStatusPending, ""
	case "processing":
		return podStatusProcessing, ""
	case "completed", "success", "succeeded":
		return podStatusSucceeded, ""
	case "local_rendered":
		return podFailureStatusForMode(dependencyMode), podFallbackLocalMockup
	case "failed", "render_unavailable":
		return podFailureStatusForMode(dependencyMode), ""
	case podStatusBypassed:
		return podStatusBypassed, ""
	default:
		return podFailureStatusForMode(dependencyMode), ""
	}
}

func podFailureStatusForMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), podDependencyModeOptional) {
		return podStatusFailedDegraded
	}
	return podStatusFailedBlocking
}

func podSubmissionBlocked(pod *PodExecutionSummary) bool {
	pod = normalizePodExecutionSummary(clonePodExecutionSummary(pod))
	if pod == nil {
		return false
	}
	switch pod.DependencyMode {
	case podDependencyModeDisabled:
		return false
	case podDependencyModeRequired:
		return pod.Status != podStatusSucceeded
	case podDependencyModeOptional:
		switch pod.Status {
		case podStatusSucceeded, podStatusFailedDegraded, podStatusBypassed:
			return false
		default:
			return true
		}
	default:
		return true
	}
}

func podReadinessMessage(pod *PodExecutionSummary) string {
	pod = normalizePodExecutionSummary(clonePodExecutionSummary(pod))
	if pod == nil || pod.DependencyMode == podDependencyModeDisabled {
		return ""
	}
	providerLabel := strings.ToUpper(firstNonEmptyString(pod.Provider, "pod"))
	switch pod.DependencyMode {
	case podDependencyModeRequired:
		if pod.Status == podStatusSucceeded {
			return ""
		}
		reason := firstNonEmptyString(pod.FailureReason, providerLabel+" 平台处理尚未完成")
		return providerLabel + " 平台处理为发布前置，当前不可提交：" + reason
	case podDependencyModeOptional:
		switch pod.Status {
		case podStatusFailedDegraded:
			reason := firstNonEmptyString(pod.FailureReason, providerLabel+" 平台结果不可用")
			return providerLabel + " 平台处理失败，当前将按降级素材继续发布：" + reason
		case podStatusBypassed:
			return providerLabel + " 平台处理已人工跳过，当前将按降级素材继续发布"
		case podStatusSucceeded:
			return ""
		default:
			return providerLabel + " 平台处理尚未完成，当前还不能提交"
		}
	default:
		return providerLabel + " 平台处理状态未知，当前还不能提交"
	}
}

func podExecutionEqual(left *PodExecutionSummary, right *PodExecutionSummary) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return left.Provider == right.Provider &&
		left.DependencyMode == right.DependencyMode &&
		left.Status == right.Status &&
		left.FailureReason == right.FailureReason &&
		left.FallbackType == right.FallbackType &&
		left.DecisionSource == right.DecisionSource &&
		sameTimePtr(left.CompletedAt, right.CompletedAt) &&
		sameTimePtr(left.LastAttemptAt, right.LastAttemptAt) &&
		left.RetryCount == right.RetryCount &&
		podExecutionAuditHistoryEqual(left.History, right.History)
}

func sameTimePtr(left *time.Time, right *time.Time) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return left.Equal(*right)
}

func normalizePodExecutionAuditHistory(items []PodExecutionAuditEvent) []PodExecutionAuditEvent {
	if len(items) == 0 {
		return nil
	}
	normalized := make([]PodExecutionAuditEvent, 0, len(items))
	for _, item := range items {
		item.Kind = strings.ToLower(strings.TrimSpace(item.Kind))
		item.Code = strings.TrimSpace(item.Code)
		item.Message = strings.TrimSpace(item.Message)
		item.Detail = strings.TrimSpace(item.Detail)
		item.Provider = strings.ToLower(strings.TrimSpace(item.Provider))
		item.DependencyMode = strings.ToLower(strings.TrimSpace(item.DependencyMode))
		item.DecisionSource = strings.TrimSpace(item.DecisionSource)
		item.FromStatus = strings.ToLower(strings.TrimSpace(item.FromStatus))
		item.ToStatus = strings.ToLower(strings.TrimSpace(item.ToStatus))
		normalized = append(normalized, item)
	}
	if len(normalized) > podAuditHistoryMaxEventSize {
		normalized = append([]PodExecutionAuditEvent(nil), normalized[len(normalized)-podAuditHistoryMaxEventSize:]...)
	}
	return normalized
}

func clonePodExecutionAuditHistory(items []PodExecutionAuditEvent) []PodExecutionAuditEvent {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]PodExecutionAuditEvent, len(items))
	copy(cloned, items)
	return cloned
}

func podExecutionAuditHistoryEqual(left []PodExecutionAuditEvent, right []PodExecutionAuditEvent) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func recordPodExecutionAudit(before *PodExecutionSummary, after *PodExecutionSummary, updatedAt time.Time) {
	after = normalizePodExecutionSummary(after)
	if after == nil {
		return
	}
	at := updatedAt
	if at.IsZero() {
		at = time.Now()
	}
	history := clonePodExecutionAuditHistory(after.History)
	if podPolicyChanged(before, after) {
		history = append(history, PodExecutionAuditEvent{
			Kind:           podAuditKindPolicyDecision,
			Code:           podAuditCodePolicyApplied,
			Message:        fmt.Sprintf("POD policy resolved as %s for provider %s", after.DependencyMode, strings.ToUpper(firstNonEmptyString(after.Provider, "pod"))),
			Detail:         fmt.Sprintf("provider=%s dependency_mode=%s decision_source=%s", firstNonEmptyString(after.Provider, "none"), after.DependencyMode, after.DecisionSource),
			Provider:       after.Provider,
			DependencyMode: after.DependencyMode,
			DecisionSource: after.DecisionSource,
			OccurredAt:     at,
		})
	}
	if podStatusChanged(before, after) {
		history = append(history, PodExecutionAuditEvent{
			Kind:           podAuditKindStatusChange,
			Code:           podAuditCodeStatusChanged,
			Message:        fmt.Sprintf("POD status changed from %s to %s", firstNonEmptyString(statusValue(before), "empty"), after.Status),
			Detail:         firstNonEmptyString(after.FailureReason, after.FallbackType),
			Provider:       after.Provider,
			DependencyMode: after.DependencyMode,
			DecisionSource: after.DecisionSource,
			FromStatus:     statusValue(before),
			ToStatus:       after.Status,
			OccurredAt:     at,
		})
	}
	after.History = normalizePodExecutionAuditHistory(history)
}

func podPolicyChanged(before *PodExecutionSummary, after *PodExecutionSummary) bool {
	if after == nil {
		return false
	}
	if before == nil {
		return strings.TrimSpace(after.Provider) != "" ||
			strings.TrimSpace(after.DependencyMode) != "" ||
			strings.TrimSpace(after.DecisionSource) != ""
	}
	return before.Provider != after.Provider ||
		before.DependencyMode != after.DependencyMode ||
		before.DecisionSource != after.DecisionSource
}

func podStatusChanged(before *PodExecutionSummary, after *PodExecutionSummary) bool {
	if after == nil {
		return false
	}
	return statusValue(before) != after.Status
}

func statusValue(pod *PodExecutionSummary) string {
	if pod == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(pod.Status))
}
