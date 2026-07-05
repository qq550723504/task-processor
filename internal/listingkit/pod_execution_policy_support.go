package listingkit

import "strings"

func inferPodStatusFromSDS(sds *SDSSyncSummary, childTasks []ChildTaskState, dependencyMode string) (status string, reason string, fallback string, found bool) {
	if status, reason, fallback, found := inferActivePodStatusFromChildTasks(childTasks); found {
		return status, reason, fallback, true
	}
	if sds != nil {
		status, fallback = mapSDSStatusToPODStatus(sds.Status, dependencyMode)
		reason = strings.TrimSpace(sds.Error)
		return status, reason, fallback, true
	}
	return inferPodStatusFromChildTasks(childTasks, dependencyMode)
}

func inferActivePodStatusFromChildTasks(childTasks []ChildTaskState) (status string, reason string, fallback string, found bool) {
	for _, child := range childTasks {
		if child.Kind != "sds_design_sync" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(child.Status)) {
		case strings.ToLower(string(TaskStatusProcessing)):
			return podStatusProcessing, "", "", true
		case strings.ToLower(string(TaskStatusPending)):
			return podStatusPending, "", "", true
		}
	}
	return "", "", "", false
}

func inferPodStatusFromChildTasks(childTasks []ChildTaskState, dependencyMode string) (status string, reason string, fallback string, found bool) {
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
