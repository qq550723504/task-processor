package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	applyGenerateRequestDefaults(req, s.requestDefaults)
	if err := validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	task := &Task{
		ID:         uuid.New().String(),
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
		taskCtx := context.WithoutCancel(ctx)
		if _, err := s.ProcessListingKit(taskCtx, task); err != nil {
			refreshed, getErr := s.repo.GetTask(taskCtx, task.ID)
			if getErr == nil {
				return refreshed, nil
			}
			return task, nil
		}
		refreshed, err := s.repo.GetTask(taskCtx, task.ID)
		if err == nil {
			return refreshed, nil
		}
		return task, nil
	}
	if s.taskSubmitter != nil {
		if err := s.taskSubmitter.Submit(task.ID); err != nil {
			_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
			return nil, fmt.Errorf("failed to submit task: %w", err)
		}
	}
	return task, nil
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
	tasks, total, err := s.repo.ListTasks(ctx, normalized)
	if err != nil {
		return nil, err
	}

	items := make([]TaskListItem, 0, len(tasks))
	for i := range tasks {
		items = append(items, buildTaskListItem(&tasks[i]))
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
	return nil
}
