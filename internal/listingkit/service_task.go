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
	resultPayload, err := s.buildTaskResultPayload(ctx, task)
	if err != nil {
		return nil, err
	}
	return buildTaskResult(task, resultPayload), nil
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
