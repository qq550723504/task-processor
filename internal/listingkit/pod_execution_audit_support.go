package listingkit

import (
	"fmt"
	"strings"
	"time"
)

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
