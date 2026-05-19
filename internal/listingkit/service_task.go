package listingkit

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if req.TenantID == "" {
		req.TenantID = TenantIDFromContext(ctx)
	}
	ctx = WithTenantID(ctx, req.TenantID)
	applyGenerateRequestDefaults(req, s.requestDefaults)
	if err := validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	task := &Task{
		ID:         uuid.New().String(),
		TenantID:   TenantIDFromContext(ctx),
		UserID:     strings.TrimSpace(req.UserID),
		Request:    req,
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
	}
	if taskHasPlatform(task, "shein") {
		if selection, err := s.resolveSheinStoreSelection(ctx, task); err == nil && selection != nil {
			task.SheinStoreResolutionSnapshot = sheinStoreResolutionSnapshotFromSelection(selection, task, nil)
		}
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	if s.taskSubmitter == nil {
		return s.runTaskInline(ctx, task)
	}
	if shouldRunStudioInline(req) {
		return s.enqueueOrRunStudioTask(ctx, task)
	}
	if err := s.enqueueTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *service) enqueueOrRunStudioTask(ctx context.Context, task *Task) (*Task, error) {
	if s.taskSubmitter != nil {
		if err := s.enqueueTask(ctx, task); err != nil {
			return nil, err
		}
		return task, nil
	}

	return s.runTaskInline(ctx, task)
}

func (s *service) runTaskInline(ctx context.Context, task *Task) (*Task, error) {
	if _, err := s.ProcessListingKit(context.WithoutCancel(ctx), task); err != nil {
		refreshed, getErr := s.repo.GetTask(context.WithoutCancel(ctx), task.ID)
		if getErr == nil {
			return refreshed, nil
		}
		return task, nil
	}
	refreshed, err := s.repo.GetTask(context.WithoutCancel(ctx), task.ID)
	if err == nil {
		return refreshed, nil
	}
	return task, nil
}

func (s *service) enqueueTask(ctx context.Context, task *Task) error {
	if s.taskSubmitter == nil {
		return nil
	}
	if err := s.taskSubmitter.Submit(task.ID); err != nil {
		_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
		return fmt.Errorf("failed to submit task: %w", err)
	}
	return nil
}

func (s *service) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	var resultPayload *ListingKitResult
	if task.Result != nil {
		copied := *task.Result
		tasks, listErr := s.listAssetGenerationTasks(ctx, task.ID)
		if listErr != nil {
			return nil, listErr
		}
		decorateListingKitResultGeneration(&copied, tasks)
		if copied.Shein != nil {
			if selection, selectionErr := s.resolveSheinStoreSelection(ctx, task); selectionErr == nil {
				copied.SheinStoreResolution = buildSheinStoreResolutionSummary(selection, task, nil)
			}
		}
		resultPayload = &copied
	}
	result := &TaskResult{
		TaskID:        task.ID,
		TenantID:      task.TenantID,
		Status:        task.Status,
		Result:        resultPayload,
		Error:         task.Error,
		ReviewReasons: reviewReasonsFromTask(task),
		CreatedAt:     task.CreatedAt,
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusFailed {
		result.CompletedAt = &task.UpdatedAt
	}
	return result, nil
}

func (s *service) ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error) {
	normalized := normalizeTaskListQuery(query)
	if normalized.TenantID != "" {
		ctx = WithTenantID(ctx, normalized.TenantID)
	}
	tasks, total, err := s.repo.ListTasks(ctx, normalized)
	if err != nil {
		return nil, err
	}

	items := make([]TaskListItem, 0, len(tasks))
	for i := range tasks {
		items = append(items, buildTaskListItem(&tasks[i]))
	}
	var summary *TaskListSummary
	if source, ok := s.repo.(TaskListSummarySource); ok {
		summaryTasks, summaryErr := source.ListTaskSummaryTasks(ctx, summaryTaskListQuery(normalized))
		if summaryErr != nil {
			return nil, summaryErr
		}
		summary = buildTaskListSummary(summaryTasks)
	}
	return &TaskListPage{
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
		Total:    total,
		Summary:  summary,
		Taxonomy: BuildTaskListTaxonomy(),
		Items:    items,
	}, nil
}

func TaskMatchesListQuery(task *Task, query *TaskListQuery) bool {
	if task == nil {
		return false
	}
	if query == nil {
		return true
	}
	if query.Status != "" && string(task.Status) != query.Status {
		return false
	}
	if query.Platform != "" && !taskHasPlatform(task, query.Platform) {
		return false
	}
	if query.SheinWorkflowStatus != "" {
		if buildTaskListItem(task).SheinWorkflowStatus != query.SheinWorkflowStatus {
			return false
		}
	}
	if query.SheinBlockerKey != "" {
		item := buildTaskListItem(task)
		matched := false
		for _, key := range item.SheinBlockingKeys {
			if key == query.SheinBlockerKey {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if query.SheinWarningKey != "" {
		item := buildTaskListItem(task)
		matched := false
		for _, key := range item.SheinWarningKeys {
			if key == query.SheinWarningKey {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if query.SheinWorkQueue != "" {
		if buildTaskListItem(task).SheinWorkQueue != query.SheinWorkQueue {
			return false
		}
	}
	if query.SheinActionQueue != "" {
		if buildTaskListItem(task).SheinActionQueue != query.SheinActionQueue {
			return false
		}
	}
	return true
}

func taskHasPlatform(task *Task, platform string) bool {
	if task == nil || task.Request == nil {
		return false
	}
	for _, candidate := range task.Request.Platforms {
		if candidate == platform {
			return true
		}
	}
	return false
}

func normalizeTaskListQuery(query *TaskListQuery) *TaskListQuery {
	normalized := &TaskListQuery{Page: 1, PageSize: 20}
	if query != nil {
		*normalized = *query
	}
	if normalized.Page <= 0 {
		normalized.Page = 1
	}
	if normalized.PageSize <= 0 {
		normalized.PageSize = 20
	}
	if normalized.PageSize > 100 {
		normalized.PageSize = 100
	}
	return normalized
}

func summaryTaskListQuery(query *TaskListQuery) *TaskListQuery {
	if query == nil {
		return nil
	}
	return &TaskListQuery{
		TenantID: query.TenantID,
		Status:   query.Status,
		Platform: query.Platform,
	}
}

func buildTaskListItem(task *Task) TaskListItem {
	if task == nil {
		return TaskListItem{}
	}
	item := TaskListItem{
		TaskID:     task.ID,
		TenantID:   task.TenantID,
		Status:     task.Status,
		Error:      task.Error,
		CreatedAt:  task.CreatedAt,
		UpdatedAt:  task.UpdatedAt,
		ImageCount: 0,
	}
	if task.Request != nil {
		item.Platforms = append([]string(nil), task.Request.Platforms...)
		item.Title = task.Request.Text
		if item.Title == "" {
			item.Title = task.Request.ProductURL
		}
		if task.Request.Options != nil && task.Request.Options.SDS != nil {
			item.ProductName = task.Request.Options.SDS.ProductName
			item.VariantLabel = strings.TrimSpace(strings.Join([]string{
				task.Request.Options.SDS.VariantColor,
				task.Request.Options.SDS.VariantSize,
				task.Request.Options.SDS.VariantSKU,
			}, " "))
			if item.Title == "" {
				item.Title = task.Request.Options.SDS.ProductName
			}
		}
		if task.Request.SheinStoreID > 0 {
			item.SheinStoreID = task.Request.SheinStoreID
		}
		if site := strings.TrimSpace(task.Request.Country); site != "" {
			item.SheinStoreSite = site
		}
	}
	if snapshot := task.SheinStoreResolutionSnapshot; snapshot != nil {
		if snapshot.StoreID > 0 {
			item.SheinStoreID = snapshot.StoreID
		}
		if site := strings.TrimSpace(snapshot.Site); site != "" {
			item.SheinStoreSite = site
		}
		if snapshot.MatchedProfileID > 0 {
			item.SheinStoreProfileID = snapshot.MatchedProfileID
		}
		if !snapshot.ResolvedAt.IsZero() {
			resolvedAt := snapshot.ResolvedAt
			item.SheinStoreResolvedAt = &resolvedAt
		}
		item.SheinStoreStrategy = strings.TrimSpace(snapshot.Strategy)
		item.SheinStoreReason = strings.TrimSpace(snapshot.Reason)
		item.SheinStoreMatchedRuleKinds = append([]string(nil), snapshot.MatchedRuleKinds...)
		item.SheinStoreManualOverride = snapshot.ManualOverride
		item.SheinStoreFallback = snapshot.Fallback
	}
	item.ImageCount = taskListImageCount(task)
	if task.Result != nil && task.Result.SDSSync != nil {
		item.SDSSyncStatus = task.Result.SDSSync.Status
	}
	if taskHasPlatform(task, "shein") || (task.Result != nil && task.Result.Shein != nil) {
		item.SheinWorkQueue = deriveSheinWorkQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview)
	}
	if task.Result != nil && task.Result.Shein != nil {
		if len(task.Result.Shein.SiteList) > 0 {
			if site := strings.TrimSpace(task.Result.Shein.SiteList[0].MainSite); site != "" {
				item.SheinStoreSite = site
			}
		}
		item.SheinWorkflowStatus = deriveSheinWorkflowStatus(task.Result.Shein)
		item.SheinBlockingKeys = sheinBlockingKeys(task.Result.Shein)
		item.SheinWarningKeys = sheinWarningKeys(task.Result.Shein)
		item.SheinStatusOverview = buildSheinTaskStatusOverview(task.Result.Shein)
		item.SheinWorkQueue = deriveSheinWorkQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview)
		item.SheinActionQueue = deriveSheinActionQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview, item.SheinBlockingKeys, item.SheinWarningKeys)
		if latest := latestSheinSubmissionEvent(task.Result.Shein); latest != nil {
			item.SheinLatestSubmissionStatus = latest.Status
			item.SheinLatestSubmissionError = latest.ErrorMessage
		} else if task.Result.Shein.Submission != nil {
			item.SheinLatestSubmissionStatus = task.Result.Shein.Submission.LastStatus
			item.SheinLatestSubmissionError = task.Result.Shein.Submission.LastError
		}
		applySheinSubmissionRemoteSummary(&item, task.Result.Shein)
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusFailed {
		completedAt := task.UpdatedAt
		item.CompletedAt = &completedAt
	}
	return item
}

func validateRequest(req *GenerateRequest) error {
	if len(req.ImageURLs) == 0 && strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.ProductURL) == "" {
		return fmt.Errorf("at least one of image_urls, text, or product_url must be provided")
	}
	if len(req.ImageURLs) > 10 {
		return fmt.Errorf("too many image URLs (max 10)")
	}
	if len(req.Platforms) == 0 {
		return fmt.Errorf("at least one platform is required")
	}
	if err := validateSheinStudioAspectRatio(req); err != nil {
		return err
	}
	return nil
}

func validateSheinStudioAspectRatio(req *GenerateRequest) error {
	if req == nil || req.Options == nil || req.Options.SheinStudio == nil || req.Options.SDS == nil {
		return nil
	}
	studio := req.Options.SheinStudio
	sds := req.Options.SDS
	if studio.SourceDesignWidth <= 0 || studio.SourceDesignHeight <= 0 || sds.PrintableWidth <= 0 || sds.PrintableHeight <= 0 {
		return nil
	}
	sourceRatio := float64(studio.SourceDesignWidth) / float64(studio.SourceDesignHeight)
	targetRatio := float64(sds.PrintableWidth) / float64(sds.PrintableHeight)
	if math.Abs(sourceRatio-targetRatio)/targetRatio > 0.25 {
		return fmt.Errorf("shein studio source image ratio differs too much from SDS printable area ratio")
	}
	return nil
}
