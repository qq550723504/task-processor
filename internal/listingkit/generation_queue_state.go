package listingkit

import (
	"strings"

	assetgeneration "task-processor/internal/asset/generation"
	common "task-processor/internal/publishing/common"
)

func generationQueueSlotStateReason(slot common.BundleSlot) string {
	switch strings.ToLower(strings.TrimSpace(slot.StateLabel)) {
	case "fallback_in_use":
		if value := strings.TrimSpace(slot.FallbackFrom); value != "" {
			return "using fallback asset while waiting for " + value
		}
		return "using fallback asset"
	case "ready":
		if value := strings.TrimSpace(slot.SatisfiedBy); value != "" {
			return "slot satisfied by " + value
		}
	}
	return ""
}

func generationQueueTaskStateReason(task assetgeneration.Task) string {
	switch strings.ToLower(strings.TrimSpace(task.ExecutionStatus)) {
	case "failed":
		return "generation task failed"
	case "running", "processing", "in_progress":
		return "generation task is running"
	case "planned", "pending", "queued":
		return "generation task is queued"
	case "completed":
		if task.ExecutionMode == assetgeneration.ExecutionModeDeferredStub {
			return "completed with stub fallback"
		}
		if task.ExecutionMode == assetgeneration.ExecutionModeRendererBacked {
			return "completed with renderer output"
		}
		return "generation task completed"
	}
	return ""
}

func generationQueueSlotExecutionQuality(slot common.BundleSlot) string {
	switch strings.ToLower(strings.TrimSpace(slot.StateLabel)) {
	case "fallback_in_use":
		return "fallback_asset"
	case "ready":
		return "exact_asset"
	default:
		return ""
	}
}

func generationQueueTaskExecutionQuality(task assetgeneration.Task) string {
	switch task.ExecutionMode {
	case assetgeneration.ExecutionModeRendererBacked:
		if strings.EqualFold(strings.TrimSpace(task.ExecutionStatus), "completed") {
			return "renderer_output"
		}
	case assetgeneration.ExecutionModeDeferredStub:
		if strings.EqualFold(strings.TrimSpace(task.ExecutionStatus), "completed") {
			return "stub_fallback"
		}
	case assetgeneration.ExecutionModePipelineBacked:
		if strings.EqualFold(strings.TrimSpace(task.ExecutionStatus), "completed") {
			return "pipeline_output"
		}
	case assetgeneration.ExecutionModeNativeAlias:
		if strings.EqualFold(strings.TrimSpace(task.ExecutionStatus), "completed") {
			return "alias_output"
		}
	}
	switch strings.ToLower(strings.TrimSpace(task.ExecutionStatus)) {
	case "failed":
		return "failed"
	case "running", "processing", "in_progress":
		return "running"
	case "planned", "pending", "queued":
		return "queued"
	}
	return ""
}
