package listingkit

import (
	"context"
	"fmt"
)

func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	ctx, task, err := s.prepareGenerateTask(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	return s.dispatchGenerateTask(ctx, task)
}

func (s *service) enqueueOrRunStudioTask(ctx context.Context, task *Task) (*Task, error) {
	return s.dispatchStudioTask(ctx, task)
}

func (s *service) runTaskInline(ctx context.Context, task *Task) (*Task, error) {
	return s.runGenerateTaskInline(ctx, task)
}

func (s *service) enqueueTask(ctx context.Context, task *Task) error {
	return s.enqueueGenerateTask(ctx, task)
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
