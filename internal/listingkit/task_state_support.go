package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/workspace/shein"
)

func buildSheinTaskStatusOverview(pkg *SheinPackage) *sheinworkspace.StatusOverview {
	return buildSheinTaskStatusOverviewWithPod(pkg, nil)
}

func buildSheinTaskStatusOverviewWithPod(pkg *SheinPackage, pod *PodExecutionSummary) *sheinworkspace.StatusOverview {
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	if projection == nil {
		return nil
	}
	return projection.StatusOverview
}

func sheinBlockingKeys(pkg *SheinPackage) []string {
	return sheinBlockingKeysWithPod(pkg, nil)
}

func sheinBlockingKeysWithPod(pkg *SheinPackage, pod *PodExecutionSummary) []string {
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	if projection == nil {
		return nil
	}
	readiness := projection.Readiness
	if readiness == nil || len(readiness.BlockingItems) == 0 {
		return nil
	}
	return uniqueNonEmptyStrings(sheinworkspace.FindKeys(readiness.BlockingItems))
}

func sheinWarningKeys(pkg *SheinPackage) []string {
	return sheinWarningKeysWithPod(pkg, nil)
}

func sheinWarningKeysWithPod(pkg *SheinPackage, pod *PodExecutionSummary) []string {
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	if projection == nil {
		return nil
	}
	readiness := projection.Readiness
	if readiness == nil || len(readiness.WarningItems) == 0 {
		return nil
	}
	return uniqueNonEmptyStrings(sheinworkspace.FindKeys(readiness.WarningItems))
}

func deriveSheinWorkQueue(task *Task, workflowStatus string, overview *sheinworkspace.StatusOverview) string {
	if task == nil {
		return ""
	}
	switch task.Status {
	case TaskStatusPending, TaskStatusProcessing:
		return SheinWorkQueueGeneration
	case TaskStatusFailed:
		return SheinWorkQueueGenerationFailed
	}
	switch workflowStatus {
	case SheinWorkflowStatusPublished:
		return SheinWorkQueuePublished
	case SheinWorkflowStatusDraftSaved:
		return SheinWorkQueueDraft
	case SheinWorkflowStatusPublishFailed:
		return SheinWorkQueueSubmitFailed
	}
	if overview == nil {
		return ""
	}
	switch overview.Status {
	case "blocked":
		return SheinWorkQueueRepair
	case "ready_with_warnings":
		return SheinWorkQueueReview
	case "ready":
		return SheinWorkQueueSubmitReady
	default:
		return ""
	}
}

func deriveSheinActionQueue(task *Task, workflowStatus string, overview *sheinworkspace.StatusOverview, blockingKeys []string, warningKeys []string) string {
	if task == nil {
		return ""
	}
	switch task.Status {
	case TaskStatusPending, TaskStatusProcessing, TaskStatusFailed:
		return ""
	}
	switch workflowStatus {
	case SheinWorkflowStatusPublished, SheinWorkflowStatusDraftSaved, SheinWorkflowStatusPublishFailed:
		return ""
	}
	for _, key := range blockingKeys {
		if queue := sheinActionQueueForKey(key); queue != "" {
			return queue
		}
	}
	for _, key := range warningKeys {
		if queue := sheinActionQueueForKey(key); queue != "" {
			return queue
		}
	}
	if overview != nil && overview.Status == "ready" {
		return SheinActionQueueSubmitReady
	}
	return ""
}

func sheinActionQueueForKey(key string) string {
	switch strings.TrimSpace(key) {
	case sheinCookieUnavailableIssueCode:
		return SheinActionQueueStoreAuth
	case "category", "category_review":
		return SheinActionQueueClassification
	case "attributes", "attribute_review":
		return SheinActionQueueAttributes
	case "sale_attributes", "variants":
		return SheinActionQueueVariant
	case "images", "final_images", "variant_image_coverage":
		return SheinActionQueueMedia
	case "pod_platform":
		return SheinActionQueueMedia
	case "pricing":
		return SheinActionQueuePricing
	case "final_review":
		return SheinActionQueueFinalReview
	case "source_facts":
		return SheinActionQueueSourceReview
	case "request_draft", "preview_product":
		return SheinActionQueuePayloadRebuild
	case "manual_notes":
		return SheinActionQueueManualReview
	default:
		return ""
	}
}
