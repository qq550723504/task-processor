package listingkit

import (
	"context"
	"strings"
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
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")
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
	markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
	appendWarning(result, warningPrefix+err.Error())
	finishSDSStageWithError(stage, recorder, code, message, err)
	ensureResultPodExecution(result, req)
}

func finalizeSDSSyncSummary(ctx context.Context, result *ListingKitResult, req *GenerateRequest, recorder *workflowRecorder, stage *workflowStageHandle, summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	result.SDSDesignResult = summary
	if needsLocalSDSMockupFallback(result.SDSDesignResult, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSDesignResult, options) {
		result.SDSDesignResult.Status = "failed"
		result.SDSDesignResult.Error = "SDS render returned blank template"
		result.SDSDesignResult.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
		stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
		ensureResultPodExecution(result, req)
		return false
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	stage.Complete()
	ensureResultPodExecution(result, req)
	return true
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
