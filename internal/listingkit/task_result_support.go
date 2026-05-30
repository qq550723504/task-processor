package listingkit

import (
	"context"
	"strings"
)

func (s *service) buildTaskResultPayload(ctx context.Context, task *Task) (*ListingKitResult, error) {
	if task == nil || task.Result == nil {
		return nil, nil
	}
	copied := cloneListingKitResultForReadState(task.Result)
	ensureResultPodExecution(copied, task.Request)
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	decorateListingKitResultGeneration(copied, tasks)
	s.refreshSheinTaskResultState(ctx, task, copied)
	if copied.Shein != nil {
		if selection, selectionErr := s.resolveSheinStoreSelection(ctx, task); selectionErr == nil {
			copied.SheinStoreResolution = buildSheinStoreResolutionSummary(selection, task, nil)
		}
	}
	return copied, nil
}

func buildTaskResult(task *Task, resultPayload *ListingKitResult) *TaskResult {
	if task == nil {
		return nil
	}
	reviewReasons := reviewReasonsFromTask(task)
	if resultPayload != nil {
		if reasons := reviewReasonsFromResult(resultPayload); len(reasons) > 0 {
			reviewReasons = reasons
		}
	}
	effectiveError := task.Error
	if task.Status == TaskStatusNeedsReview && len(reviewReasons) > 0 {
		effectiveError = strings.Join(reviewReasons, "; ")
	}
	result := &TaskResult{
		TaskIdentityFields: TaskIdentityFields{
			TaskID:   task.ID,
			TenantID: task.TenantID,
		},
		TaskResultLifecycleFields: TaskResultLifecycleFields{
			Status:    task.Status,
			Error:     effectiveError,
			CreatedAt: task.CreatedAt,
		},
		Result:        resultPayload,
		ReviewReasons: reviewReasons,
	}
	if resultPayload != nil && resultPayload.Shein != nil {
		applySheinSubmissionStatusFields(&result.SheinSubmissionStatusFields, resultPayload.Shein)
	}
	if taskStatusIsTerminal(task.Status) {
		result.CompletedAt = &task.UpdatedAt
	}
	return result
}

func taskStatusIsTerminal(status TaskStatus) bool {
	return status == TaskStatusCompleted || status == TaskStatusNeedsReview || status == TaskStatusFailed
}

func (s *service) refreshSheinTaskResultState(ctx context.Context, task *Task, result *ListingKitResult) {
	if s == nil || task == nil || result == nil || result.Shein == nil {
		return
	}

	stripSheinCookieUnavailableReviewNotes(result.Shein)
	note := strings.TrimSpace(s.resolveSheinCookieAvailabilityNote(ctx, task))
	if note != "" {
		refreshSheinReviewState(result.Shein, note)
	} else {
		refreshSheinReviewState(result.Shein)
	}

	if result.Summary == nil {
		result.Summary = &GenerationSummary{}
	}
	result.Summary.Warnings = filterOutSheinCookieUnavailableReviewNotes(result.Summary.Warnings)
	result.Summary.NeedsReview = false
	result.ReviewReasons = nil
	result.WorkflowIssues = filterOutWorkflowIssuesByStage(result.WorkflowIssues, "shein_review")

	applySheinInspectionReviewToSummary(result)
	applySheinVariantCoverageReviewToSummary(result)
	addSheinReviewWorkflowIssues(result)
	newWorkflowRecorder(result).FinalizeSummary()
}

func cloneListingKitResultForReadState(src *ListingKitResult) *ListingKitResult {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.ReviewReasons = append([]string(nil), src.ReviewReasons...)
	cloned.WorkflowIssues = append([]WorkflowIssue(nil), src.WorkflowIssues...)
	cloned.WorkflowStages = append([]WorkflowStage(nil), src.WorkflowStages...)
	if src.Summary != nil {
		summary := *src.Summary
		summary.Warnings = append([]string(nil), src.Summary.Warnings...)
		cloned.Summary = &summary
	}
	cloned.PodExecution = clonePodExecutionSummary(src.PodExecution)
	if src.Shein != nil {
		cloned.Shein = cloneSheinPackageForReadState(src.Shein)
	}
	if src.StandardProductSnapshot != nil {
		snapshot := *src.StandardProductSnapshot
		snapshot.PodExecution = clonePodExecutionSummary(src.StandardProductSnapshot.PodExecution)
		cloned.StandardProductSnapshot = &snapshot
	}
	return &cloned
}

func cloneSheinPackageForReadState(src *SheinPackage) *SheinPackage {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.ReviewNotes = append([]string(nil), src.ReviewNotes...)
	if src.CategoryResolution != nil {
		category := *src.CategoryResolution
		category.ReviewNotes = append([]string(nil), src.CategoryResolution.ReviewNotes...)
		cloned.CategoryResolution = &category
	}
	if src.AttributeResolution != nil {
		attributes := *src.AttributeResolution
		attributes.ReviewNotes = append([]string(nil), src.AttributeResolution.ReviewNotes...)
		cloned.AttributeResolution = &attributes
	}
	if src.SaleAttributeResolution != nil {
		saleAttributes := *src.SaleAttributeResolution
		saleAttributes.ReviewNotes = append([]string(nil), src.SaleAttributeResolution.ReviewNotes...)
		cloned.SaleAttributeResolution = &saleAttributes
	}
	return &cloned
}

func filterOutWorkflowIssuesByStage(items []WorkflowIssue, stage string) []WorkflowIssue {
	if len(items) == 0 {
		return nil
	}
	stage = strings.TrimSpace(stage)
	filtered := make([]WorkflowIssue, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Stage) == stage {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
