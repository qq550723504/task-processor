package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/catalog"
	submissiondomain "task-processor/internal/listing/submission"
)

func persistClassifiedTaskFailure(ctx context.Context, repo Repository, taskID string, errorMsg string, cause error) error {
	if repo == nil {
		return fmt.Errorf("repository is nil")
	}
	return submissiondomain.PersistClassifiedRetryableFailure(submissiondomain.RetryableFailurePersistenceRequest{
		DefaultRecoveryScope: submissiondomain.RetryableRecoveryScopeTask,
		ErrorMessage:         errorMsg,
		Cause:                cause,
		MarkBlockedRetryable: func(block *submissiondomain.RetryableBlockState, markedErrorMsg string) error {
			return repo.MarkBlockedRetryable(ctx, taskID, adaptSubmissionRetryableBlock(block), markedErrorMsg)
		},
		MarkFailed: func(markedErrorMsg string) error {
			return repo.MarkFailed(ctx, taskID, markedErrorMsg)
		},
	})
}

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

func effectiveCatalogProduct(result *ListingKitResult) *catalog.Product {
	if result == nil {
		return nil
	}
	if result.CatalogProduct != nil {
		return result.CatalogProduct
	}
	return catalog.BuildProduct(result.CanonicalProduct)
}

func cloneListingKitResult(result *ListingKitResult) (*ListingKitResult, error) {
	if result == nil {
		return nil, nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var cloned ListingKitResult
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func buildTaskResult(task *Task, resultPayload *ListingKitResult) *TaskResult {
	if task == nil {
		return nil
	}
	projection := buildTaskResultProjection(task, resultPayload)
	result := &TaskResult{
		TaskIdentityFields: TaskIdentityFields{
			TaskID:   task.ID,
			TenantID: task.TenantID,
		},
		TaskResultLifecycleFields: projection.Lifecycle,
		Result:                    resultPayload,
		ReviewReasons:             projection.ReviewReasons,
		RetryableBlock:            projection.RetryableBlock,
	}
	if resultPayload != nil && resultPayload.Shein != nil {
		applySheinSubmissionStatusFields(&result.SheinSubmissionStatusFields, resultPayload.Shein)
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

func markTaskCompleted(task *Task) {
	if task == nil {
		return
	}
	task.Status = TaskStatusCompleted
	task.Error = ""
	if task.Result == nil {
		return
	}
	task.Result.Status = string(TaskStatusCompleted)
	task.Result.ReviewReasons = nil
	clearWorkflowIssuesForStage(task.Result, "shein_review")
	task.Result.UpdatedAt = time.Now()
}

func clearWorkflowIssuesForStage(result *ListingKitResult, stage string) {
	if result == nil {
		return
	}
	result.WorkflowIssues = filterOutWorkflowIssuesByStage(result.WorkflowIssues, stage)
	if result.Summary != nil {
		result.Summary.NeedsReview = false
	}
	newWorkflowRecorder(result).FinalizeSummary()
}
