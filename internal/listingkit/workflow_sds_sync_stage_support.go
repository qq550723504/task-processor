package listingkit

import (
	"strings"
	"task-processor/internal/listingkit/core"
	"time"
)

func normalizeSDSSyncRecorder(result *ListingKitResult, recorder *workflowRecorder) *workflowRecorder {
	if recorder == nil {
		return newWorkflowRecorder(result)
	}
	return recorder
}

func beginSDSSyncStage(result *ListingKitResult, req *GenerateRequest, recorder *workflowRecorder) (*workflowRecorder, *workflowStageHandle) {
	recorder = normalizeSDSSyncRecorder(result, recorder)
	stage := recorder.Start("sds_design_sync", "")
	markChildTask(result, "sds_design_sync", "", string(core.TaskStatusProcessing), "")
	ensureResultPodExecution(result, req)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())
	return recorder, stage
}

func failSDSSyncStage(result *ListingKitResult, req *GenerateRequest, recorder *workflowRecorder, stage *workflowStageHandle, variantID int64, warningPrefix, code, message string, err error) {
	result.SDSDesignResult = &SDSSyncSummary{
		VariantID: variantID,
		Status:    "failed",
		Error:     err.Error(),
	}
	markChildTask(result, "sds_design_sync", "", string(core.TaskStatusFailed), err.Error())
	appendWarning(result, warningPrefix+err.Error())
	finishSDSStageWithError(stage, recorder, code, message, err)
	ensureResultPodExecution(result, req)
}

func finalizeSDSSyncSummary(result *ListingKitResult, req *GenerateRequest, recorder *workflowRecorder, stage *workflowStageHandle, summary *SDSSyncSummary, options *SDSSyncOptions) {
	result.SDSDesignResult = summary
	if sdsRenderedImageSetIncomplete(result.SDSDesignResult, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
	}
	markChildTask(result, "sds_design_sync", "", string(core.TaskStatusCompleted), "")
	stage.Complete()
	ensureResultPodExecution(result, req)
}

func sdsRenderedImageSetIncomplete(summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	if summary == nil || options == nil || len(options.MockupImageURLs) == 0 {
		return false
	}
	renderedCount := len(uniqueNonEmptyStrings(summary.MockupImageURLs))
	if renderedCount == 0 {
		return true
	}
	expectedCount := len(uniqueNonEmptyStrings(options.MockupImageURLs))
	return expectedCount > 1 && renderedCount < expectedCount
}

func failedSDSVariantSyncSummary(variant SDSSyncVariantOption, errorMsg string) SDSSyncSummary {
	return SDSSyncSummary{
		VariantID:    variant.VariantID,
		ProductID:    variant.VariantID,
		VariantSKU:   strings.TrimSpace(variant.VariantSKU),
		VariantSize:  strings.TrimSpace(variant.Size),
		VariantColor: strings.TrimSpace(variant.Color),
		Status:       "failed",
		Error:        errorMsg,
	}
}

func emptySDSVariantSyncSummary(variant SDSSyncVariantOption) SDSSyncSummary {
	return failedSDSVariantSyncSummary(variant, "SDS template render returned empty result")
}
