package listingkit

import (
	"context"
	"fmt"
)

type taskLifecycleServiceConfig struct {
	repo                        Repository
	sdsBaselineReadinessService sdsBaselineReadinessService
	validateSheinStoreAccess    func(context.Context, int64, int64) error
	requestDefaults             func() generateRequestDefaults
	taskSubmitter               func() TaskSubmitter
	standardWorkflow            func() (StandardProductWorkflowClient, bool)
	processListingKit           func(context.Context, *Task) (*ListingKitResult, error)
	resolveStoreSelection       func(context.Context, *Task) (*sheinStoreSelection, error)
	buildResultPayload          func(context.Context, *Task) (*ListingKitResult, error)
}

type taskLifecycleService struct {
	repo                        Repository
	sdsBaselineReadinessService sdsBaselineReadinessService
	validateSheinStoreAccess    func(context.Context, int64, int64) error
	requestDefaults             func() generateRequestDefaults
	taskSubmitter               func() TaskSubmitter
	standardWorkflow            func() (StandardProductWorkflowClient, bool)
	processListingKit           func(context.Context, *Task) (*ListingKitResult, error)
	resolveStoreSelection       func(context.Context, *Task) (*sheinStoreSelection, error)
	buildResultPayload          func(context.Context, *Task) (*ListingKitResult, error)
}

func newTaskLifecycleService(config taskLifecycleServiceConfig) *taskLifecycleService {
	return &taskLifecycleService{
		repo:                        config.repo,
		sdsBaselineReadinessService: config.sdsBaselineReadinessService,
		validateSheinStoreAccess:    config.validateSheinStoreAccess,
		requestDefaults:             config.requestDefaults,
		taskSubmitter:               config.taskSubmitter,
		standardWorkflow:            config.standardWorkflow,
		processListingKit:           config.processListingKit,
		resolveStoreSelection:       config.resolveStoreSelection,
		buildResultPayload:          config.buildResultPayload,
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
	dispatched, err := s.dispatchGenerateTask(ctx, task)
	if err != nil {
		return task, err
	}
	return dispatched, nil
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
	if s.sdsBaselineReadinessService == nil {
		return nil, fmt.Errorf("sds baseline readiness service is not configured")
	}
	return s.sdsBaselineReadinessService.GetReadiness(ctx, query)
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
	if normalized.IncludeSummary {
		source, ok := s.repo.(TaskListSummarySource)
		if ok {
			summaryTasks, summaryErr := source.ListTaskSummaryTasks(ctx, summaryTaskListQuery(normalized))
			if summaryErr != nil {
				return nil, summaryErr
			}
			summary = buildTaskListSummary(summaryTasks)
		}
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

func (s *taskLifecycleService) ListSheinSourceSDSMetadata(ctx context.Context, query *SheinSourceSDSMetadataQuery) ([]SheinSourceSDSMetadataRecord, error) {
	if query == nil {
		return []SheinSourceSDSMetadataRecord{}, nil
	}
	if query.StoreID <= 0 {
		return []SheinSourceSDSMetadataRecord{}, nil
	}
	source, ok := s.repo.(SheinSourceSDSMetadataSource)
	if !ok {
		return nil, fmt.Errorf("shein source SDS metadata source is not configured")
	}
	return source.ListSheinSourceSDSMetadata(ctx, query)
}
