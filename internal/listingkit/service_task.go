package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	sheinpub "task-processor/internal/publishing/shein"
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
		Request:    req,
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
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
		item := buildTaskListItem(&tasks[i])
		if normalized.SheinWorkflowStatus != "" && item.SheinWorkflowStatus != normalized.SheinWorkflowStatus {
			continue
		}
		items = append(items, item)
	}
	if normalized.SheinWorkflowStatus != "" {
		total = int64(len(items))
	}
	return &TaskListPage{
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
		Total:    total,
		Items:    items,
	}, nil
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
		item.ImageCount = len(task.Request.ImageURLs)
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
	}
	if task.Result != nil && task.Result.SDSSync != nil {
		item.SDSSyncStatus = task.Result.SDSSync.Status
	}
	if task.Result != nil && task.Result.Shein != nil {
		item.SheinWorkflowStatus = deriveSheinWorkflowStatus(task.Result.Shein)
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

func applySheinSubmissionRemoteSummary(item *TaskListItem, pkg *SheinPackage) {
	if item == nil || pkg == nil || pkg.Submission == nil {
		return
	}
	submission := pkg.Submission
	item.SheinSubmissionRemoteStatus = submission.RemoteStatus
	item.SheinSubmissionRemoteCheckedAt = submission.RemoteCheckedAt
	record := sheinSubmissionRecordForAction(submission, submission.LastAction)
	if record == nil && submission.Publish != nil {
		record = submission.Publish
	}
	if record == nil && submission.SaveDraft != nil {
		record = submission.SaveDraft
	}
	if record != nil {
		item.SheinSubmissionRemoteRecordID = record.RemoteRecordID
		if item.SheinSubmissionRemoteCheckedAt == nil {
			item.SheinSubmissionRemoteCheckedAt = record.RemoteCheckedAt
		}
	}
}

func deriveSheinWorkflowStatus(pkg *SheinPackage) string {
	if pkg == nil {
		return ""
	}
	if latest := latestSheinSubmissionEvent(pkg); latest != nil {
		if latest.Action == "publish" && latest.Status == "success" {
			return "published"
		}
		if latest.Action == "save_draft" && latest.Status == "success" {
			return "draft_saved"
		}
		if latest.Status == "failed" {
			return "publish_failed"
		}
	}
	if pkg.Submission != nil {
		if pkg.Submission.Publish != nil && pkg.Submission.Publish.Status == "success" {
			return "published"
		}
		if pkg.Submission.SaveDraft != nil && pkg.Submission.SaveDraft.Status == "success" {
			return "draft_saved"
		}
		if pkg.Submission.LastStatus == "failed" {
			return "publish_failed"
		}
	}
	readiness := buildSheinSubmitReadiness(pkg)
	if readiness != nil && readiness.Ready {
		return "ready_to_submit"
	}
	return "pending_confirmation"
}

func latestSheinSubmissionEvent(pkg *SheinPackage) *sheinpub.SubmissionEvent {
	if pkg == nil || len(pkg.SubmissionEvents) == 0 {
		return nil
	}
	return &pkg.SubmissionEvents[0]
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
	return nil
}
