package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || req == nil || strings.TrimSpace(req.Kind) == "" {
		return nil, ErrChildTaskRetryInvalidRequest
	}
	if s == nil || s.repo == nil {
		return nil, ErrTaskNotFound
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrTaskNotFound
	}
	if task.Status == TaskStatusPending || task.Status == TaskStatusProcessing {
		return nil, ErrChildTaskRetryConflict
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}

	result, err := cloneListingKitResult(task.Result)
	if err != nil {
		return nil, err
	}
	result = normalizeListingKitResultSemanticFields(result)
	kind := strings.TrimSpace(req.Kind)
	if !childTaskRetrySupportedKind(kind) {
		return nil, ErrChildTaskNotRetryable
	}
	state, ok := childTaskStateByKind(result, kind)
	if !ok {
		return nil, ErrChildTaskNotFound
	}
	if state.Status == string(TaskStatusProcessing) || state.Status == string(TaskStatusPending) {
		return nil, ErrChildTaskRetryConflict
	}
	if state.Status != string(TaskStatusFailed) && state.Status != string(TaskStatusCompleted) {
		return nil, ErrChildTaskNotRetryable
	}

	pruneChildTaskRetryArtifacts(result, kind)
	recorder := newWorkflowRecorder(result)
	switch kind {
	case "sds_catalog_product":
		err = s.retrySDSCatalogProduct(ctx, task, result, recorder)
	case "sds_design_sync":
		err = s.retrySDSDesignSync(ctx, task, result, recorder)
	default:
		err = ErrChildTaskNotRetryable
	}
	if err != nil {
		markChildTask(result, kind, state.TaskID, string(TaskStatusFailed), err.Error())
		return s.persistRetriedChildTaskResult(ctx, task, result, kind, err)
	}
	return s.persistRetriedChildTaskResult(ctx, task, result, kind, nil)
}

func (s *service) retrySDSCatalogProduct(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) error {
	stage := recorder.Start("sds_catalog_product", "")
	canonicalProduct := buildStudioFallbackCanonicalProduct(task)
	if canonicalProduct == nil {
		stage.Fail("sds_catalog_product_failed", "Failed to build SDS studio product", "")
		return fmt.Errorf("failed to build SDS studio product")
	}
	result.CanonicalProduct = canonicalProduct
	result.CatalogProduct = catalog.BuildProduct(canonicalProduct)
	if result.AssetBundle == nil {
		result.AssetBundle = asset.BuildBundle(canonicalProduct, result.ImageAssets)
	}
	result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
	markChildTask(result, "sds_catalog_product", "", string(TaskStatusCompleted), "")
	stage.Complete()

	var sdsOptions *SDSSyncOptions
	if task.Request != nil && task.Request.Options != nil {
		sdsOptions = task.Request.Options.SDS
	}
	if applySDSSyncMetadataToCanonical(canonicalProduct, result.SDSDesignResult, sdsOptions) {
		result.CatalogProduct = catalog.BuildProduct(canonicalProduct)
		if result.AssetBundle == nil {
			result.AssetBundle = asset.BuildBundle(canonicalProduct, result.ImageAssets)
		}
		result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
	}
	result.Summary = ensureGenerationSummary(result.Summary)
	result.Summary.NeedsReview = false
	snapshot := buildStandardProductSnapshot(result)
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, canonicalProduct)
	final := s.runPlatformAdaptation(ctx, task, snapshot, recipesByPlatform, nil, nil, nil, shouldGenerateAssets(task.Request), sdsOptions)
	*result = *final
	return nil
}

func (s *service) retrySDSDesignSync(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) error {
	if task == nil || task.Request == nil {
		return ErrTaskResultUnavailable
	}
	var sdsOptions *SDSSyncOptions
	if task.Request != nil && task.Request.Options != nil {
		sdsOptions = task.Request.Options.SDS
	}
	if sdsOptions == nil {
		return ErrChildTaskNotRetryable
	}
	if result.ImageAssets != nil {
		s.syncSDSDesign(ctx, task, result, result.ImageAssets, recorder)
	} else if shouldRunRemoteSDSDesignSync(task.Request) {
		s.syncSDSDesignFromRemote(ctx, task, result, recorder)
	} else {
		return ErrChildTaskNotRetryable
	}
	if result.CanonicalProduct != nil {
		if applySDSSyncMetadataToCanonical(result.CanonicalProduct, result.SDSDesignResult, sdsOptions) {
			result.CatalogProduct = catalog.BuildProduct(result.CanonicalProduct)
			if result.AssetBundle == nil {
				result.AssetBundle = asset.BuildBundle(result.CanonicalProduct, result.ImageAssets)
			}
			result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
		}
	}
	result.Summary = ensureGenerationSummary(result.Summary)
	result.Summary.NeedsReview = false
	snapshot := buildStandardProductSnapshot(result)
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, snapshot.CanonicalProduct)
	final := s.runPlatformAdaptation(ctx, task, snapshot, recipesByPlatform, nil, nil, nil, shouldGenerateAssets(task.Request), sdsOptions)
	*result = *final
	return nil
}

func (s *service) persistRetriedChildTaskResult(ctx context.Context, task *Task, result *ListingKitResult, kind string, retryErr error) (*TaskResult, error) {
	if result == nil || task == nil {
		return nil, ErrTaskResultUnavailable
	}
	result.Status = string(TaskStatusProcessing)
	if retryErr != nil {
		result.Summary = ensureGenerationSummary(result.Summary)
		result.Summary.NeedsReview = true
		appendWarning(result, retryErr.Error())
	}
	newWorkflowRecorder(result).FinalizeSummary()
	if retryErr != nil || childTaskHasFailed(result, kind) {
		if result.Summary != nil {
			result.Summary.NeedsReview = true
		}
	}
	if childTaskHasFailed(result, kind) {
		if err := s.repo.MarkNeedsReview(ctx, task.ID, result, taskNeedsReviewReason(result)); err != nil {
			return nil, err
		}
		task.Status = TaskStatusNeedsReview
		task.Error = taskNeedsReviewReason(result)
	} else if result.Summary != nil && result.Summary.NeedsReview {
		if err := s.repo.MarkNeedsReview(ctx, task.ID, result, taskNeedsReviewReason(result)); err != nil {
			return nil, err
		}
		task.Status = TaskStatusNeedsReview
		task.Error = taskNeedsReviewReason(result)
	} else {
		if err := s.repo.MarkCompleted(ctx, task.ID, result); err != nil {
			return nil, err
		}
		task.Status = TaskStatusCompleted
		task.Error = ""
	}
	task.Result = result
	task.UpdatedAt = time.Now()
	return s.GetTaskResult(ctx, task.ID)
}

func childTaskRetrySupportedKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "sds_catalog_product", "sds_design_sync":
		return true
	default:
		return false
	}
}

func childTaskStateByKind(result *ListingKitResult, kind string) (*ChildTaskState, bool) {
	if result == nil {
		return nil, false
	}
	for i := range result.ChildTasks {
		if result.ChildTasks[i].Kind == kind {
			return &result.ChildTasks[i], true
		}
	}
	return nil, false
}

func childTaskHasFailed(result *ListingKitResult, kind string) bool {
	state, ok := childTaskStateByKind(result, kind)
	return ok && state.Status == string(TaskStatusFailed)
}

func pruneChildTaskRetryArtifacts(result *ListingKitResult, kind string) {
	if result == nil {
		return
	}
	removedIssueTexts := workflowIssueTextsByStage(result.WorkflowIssues, kind)
	result.WorkflowIssues = filterWorkflowIssuesByStage(result.WorkflowIssues, kind)
	if result.Summary != nil && len(result.Summary.Warnings) > 0 {
		result.Summary.Warnings = filterOutSummaryWarnings(result.Summary.Warnings, removedIssueTexts)
	}
}

func filterWorkflowIssuesByStage(issues []WorkflowIssue, stage string) []WorkflowIssue {
	if len(issues) == 0 {
		return nil
	}
	out := make([]WorkflowIssue, 0, len(issues))
	for _, issue := range issues {
		if issue.Stage == stage {
			continue
		}
		out = append(out, issue)
	}
	return out
}

func workflowIssueTextsByStage(issues []WorkflowIssue, stage string) []string {
	if len(issues) == 0 {
		return nil
	}
	values := make([]string, 0, len(issues)*2)
	for _, issue := range issues {
		if issue.Stage != stage {
			continue
		}
		if message := strings.TrimSpace(issue.Message); message != "" {
			values = append(values, message)
		}
		if detail := strings.TrimSpace(issue.Detail); detail != "" {
			values = append(values, detail)
		}
	}
	return normalizeReviewReasons(values)
}

func filterOutSummaryWarnings(warnings []string, removals []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	if len(removals) == 0 {
		return append([]string(nil), warnings...)
	}
	removalSet := make(map[string]struct{}, len(removals))
	for _, value := range removals {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		removalSet[trimmed] = struct{}{}
	}
	filtered := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		trimmed := strings.TrimSpace(warning)
		if trimmed == "" {
			continue
		}
		if _, exists := removalSet[trimmed]; exists {
			continue
		}
		filtered = append(filtered, warning)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func ensureGenerationSummary(summary *GenerationSummary) *GenerationSummary {
	if summary != nil {
		return summary
	}
	return &GenerationSummary{}
}
