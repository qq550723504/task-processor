package listingkit

import (
	"strings"
	"time"

	sdspod "task-processor/internal/product/sourcing/sdspod"
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
	applyPodExecutionPolicyState(pod, sdspod.NormalizeExecution(podExecutionPolicyState(pod)))
	pod.History = normalizePodExecutionAuditHistory(pod.History)
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

func podExecutionPolicyState(pod *PodExecutionSummary) sdspod.Execution {
	if pod == nil {
		return sdspod.Execution{}
	}
	return sdspod.Execution{
		Provider:       pod.Provider,
		DependencyMode: pod.DependencyMode,
		Status:         pod.Status,
		FailureReason:  pod.FailureReason,
		FallbackType:   pod.FallbackType,
		DecisionSource: pod.DecisionSource,
	}
}

func applyPodExecutionPolicyState(pod *PodExecutionSummary, state sdspod.Execution) *PodExecutionSummary {
	if pod == nil {
		pod = &PodExecutionSummary{}
	}
	pod.Provider = state.Provider
	pod.DependencyMode = state.DependencyMode
	pod.Status = state.Status
	pod.FailureReason = state.FailureReason
	pod.FallbackType = state.FallbackType
	pod.DecisionSource = state.DecisionSource
	return pod
}

func podExecutionPolicySDS(sds *SDSSyncSummary) *sdspod.SDSResult {
	if sds == nil {
		return nil
	}
	return &sdspod.SDSResult{Status: sds.Status, Error: sds.Error}
}

func podExecutionPolicyChildren(children []ChildTaskState) []sdspod.ChildTask {
	if len(children) == 0 {
		return nil
	}
	result := make([]sdspod.ChildTask, 0, len(children))
	for _, child := range children {
		result = append(result, sdspod.ChildTask{Kind: child.Kind, Status: child.Status, Error: child.Error})
	}
	return result
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

	pod = applyPodExecutionPolicyState(pod, sdspod.DeriveExecution(sdspod.DeriveInput{
		Current:  podExecutionPolicyState(pod),
		SDS:      podExecutionPolicySDS(sds),
		Children: podExecutionPolicyChildren(childTasks),
	}))
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
