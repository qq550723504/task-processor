package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type taskLifecycleServiceConfig struct {
	repo                  Repository
	requestDefaults       func() generateRequestDefaults
	taskSubmitter         func() TaskSubmitter
	standardWorkflow      func() (StandardProductWorkflowClient, bool)
	processListingKit     func(context.Context, *Task) (*ListingKitResult, error)
	resolveStoreSelection func(context.Context, *Task) (*sheinStoreSelection, error)
	buildResultPayload    func(context.Context, *Task) (*ListingKitResult, error)
}

type taskLifecycleService struct {
	repo                  Repository
	requestDefaults       func() generateRequestDefaults
	taskSubmitter         func() TaskSubmitter
	standardWorkflow      func() (StandardProductWorkflowClient, bool)
	processListingKit     func(context.Context, *Task) (*ListingKitResult, error)
	resolveStoreSelection func(context.Context, *Task) (*sheinStoreSelection, error)
	buildResultPayload    func(context.Context, *Task) (*ListingKitResult, error)
}

func newTaskLifecycleService(config taskLifecycleServiceConfig) *taskLifecycleService {
	return &taskLifecycleService{
		repo:                  config.repo,
		requestDefaults:       config.requestDefaults,
		taskSubmitter:         config.taskSubmitter,
		standardWorkflow:      config.standardWorkflow,
		processListingKit:     config.processListingKit,
		resolveStoreSelection: config.resolveStoreSelection,
		buildResultPayload:    config.buildResultPayload,
	}
}

func (s *taskLifecycleService) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	ctx, task, err := s.prepareGenerateTask(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	return s.dispatchGenerateTask(ctx, task)
}

func (s *taskLifecycleService) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	var resultPayload *ListingKitResult
	if s.buildResultPayload != nil {
		resultPayload, err = s.buildResultPayload(ctx, task)
		if err != nil {
			return nil, err
		}
	}
	return buildTaskResult(task, resultPayload), nil
}

func (s *taskLifecycleService) GetSDSBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}
	if query.TenantID != "" {
		ctx = WithTenantID(ctx, query.TenantID)
	}
	return (&sdsBaselineService{repo: s.repo}).GetReadiness(ctx, query)
}

func (s *taskLifecycleService) ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error) {
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

func (s *taskLifecycleService) enqueueOrRunStudioTask(ctx context.Context, task *Task) (*Task, error) {
	return s.dispatchStudioTask(ctx, task)
}

func (s *taskLifecycleService) runTaskInline(ctx context.Context, task *Task) (*Task, error) {
	return s.runGenerateTaskInline(ctx, task)
}

func (s *taskLifecycleService) enqueueTask(ctx context.Context, task *Task) error {
	return s.enqueueGenerateTask(ctx, task)
}

func (s *taskLifecycleService) prepareGenerateTask(ctx context.Context, req *GenerateRequest) (context.Context, *Task, error) {
	if req == nil {
		return ctx, nil, fmt.Errorf("request cannot be nil")
	}
	if req.TenantID == "" {
		req.TenantID = TenantIDFromContext(ctx)
	}
	ctx = WithTenantID(ctx, req.TenantID)
	if s.requestDefaults != nil {
		applyGenerateRequestDefaults(req, s.requestDefaults())
	}
	if err := validateRequest(req); err != nil {
		return ctx, nil, fmt.Errorf("invalid request: %w", err)
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
	s.applySheinStoreResolutionSnapshot(ctx, task)
	return ctx, task, nil
}

func (s *taskLifecycleService) applySheinStoreResolutionSnapshot(ctx context.Context, task *Task) {
	if task == nil || !taskHasPlatform(task, "shein") || s.resolveStoreSelection == nil {
		return
	}
	if selection, err := s.resolveStoreSelection(ctx, task); err == nil && selection != nil {
		task.SheinStoreResolutionSnapshot = sheinStoreResolutionSnapshotFromSelection(selection, task, nil)
	}
}

func (s *taskLifecycleService) dispatchGenerateTask(ctx context.Context, task *Task) (*Task, error) {
	if task == nil {
		return nil, nil
	}
	if s.taskSubmitter == nil || s.taskSubmitter() == nil {
		return s.runGenerateTaskInline(ctx, task)
	}
	if shouldRunStudioInline(task.Request) {
		return s.dispatchStudioTask(ctx, task)
	}
	if err := s.enqueueGenerateTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *taskLifecycleService) dispatchStudioTask(ctx context.Context, task *Task) (*Task, error) {
	if s.taskSubmitter != nil && s.taskSubmitter() != nil {
		if err := s.enqueueGenerateTask(ctx, task); err != nil {
			return nil, err
		}
		return task, nil
	}
	return s.runGenerateTaskInline(ctx, task)
}

func (s *taskLifecycleService) runGenerateTaskInline(ctx context.Context, task *Task) (*Task, error) {
	runCtx := context.WithoutCancel(ctx)
	if s.processListingKit != nil {
		if _, err := s.processListingKit(runCtx, task); err != nil {
			return s.refreshGenerateTask(runCtx, task)
		}
	}
	return s.refreshGenerateTask(runCtx, task)
}

func (s *taskLifecycleService) refreshGenerateTask(ctx context.Context, task *Task) (*Task, error) {
	if task == nil {
		return nil, nil
	}
	refreshed, err := s.repo.GetTask(ctx, task.ID)
	if err == nil {
		return refreshed, nil
	}
	return task, nil
}

func (s *taskLifecycleService) enqueueGenerateTask(ctx context.Context, task *Task) error {
	if s.standardWorkflow != nil {
		if client, enabled := s.standardWorkflow(); enabled && client != nil {
			if err := client.StartStandardProduct(ctx, StandardProductWorkflowStartInput{
				TaskID:      strings.TrimSpace(task.ID),
				RequestedAt: time.Now().UTC(),
			}); err != nil {
				_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to start standard product workflow: %v", err))
				return fmt.Errorf("failed to start standard product workflow: %w", err)
			}
			return nil
		}
	}
	if s.taskSubmitter == nil || s.taskSubmitter() == nil {
		return nil
	}
	if err := s.taskSubmitter().Submit(task.ID); err != nil {
		_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
		return fmt.Errorf("failed to submit task: %w", err)
	}
	return nil
}
