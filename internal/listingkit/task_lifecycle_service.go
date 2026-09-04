package listingkit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"task-processor/internal/listingkit/core"
)

type taskLifecycleServiceConfig struct {
	repo                        Repository
	productSnapshots            ProductSnapshotReader
	sdsBaselineReadinessService sdsBaselineReadinessService
	validateSheinStoreAccess    func(context.Context, int64, int64) error
	taskSubmitter               func() TaskSubmitter
	standardWorkflow            func() (StandardProductWorkflowClient, bool)
	processListingKit           func(context.Context, *Task) (*ListingKitResult, error)
	resolveStoreSelection       func(context.Context, *Task) (*sheinStoreSelection, error)
	buildResultPayload          func(context.Context, *Task) (*ListingKitResult, error)
}

type taskLifecycleService struct {
	repo                        Repository
	productSnapshots            ProductSnapshotReader
	sdsBaselineReadinessService sdsBaselineReadinessService
	validateSheinStoreAccess    func(context.Context, int64, int64) error
	taskSubmitter               func() TaskSubmitter
	standardWorkflow            func() (StandardProductWorkflowClient, bool)
	processListingKit           func(context.Context, *Task) (*ListingKitResult, error)
	resolveStoreSelection       func(context.Context, *Task) (*sheinStoreSelection, error)
	buildResultPayload          func(context.Context, *Task) (*ListingKitResult, error)
}

func newTaskLifecycleService(config taskLifecycleServiceConfig) *taskLifecycleService {
	return &taskLifecycleService{
		repo:                        config.repo,
		productSnapshots:            config.productSnapshots,
		sdsBaselineReadinessService: config.sdsBaselineReadinessService,
		validateSheinStoreAccess:    config.validateSheinStoreAccess,
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
		if replayed, replayErr := s.replayIdempotentGenerateTask(ctx, task); replayErr != nil {
			if errors.Is(replayErr, ErrGenerateTaskIdempotencyConflict) {
				return nil, replayErr
			}
			return nil, fmt.Errorf("failed to create task: %w", err)
		} else if replayed != nil {
			if replayed.Status == core.TaskStatusPending {
				return s.dispatchGenerateTask(ctx, replayed)
			}
			return replayed, nil
		}
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	dispatched, err := s.dispatchGenerateTask(ctx, task)
	if err != nil {
		if task != nil && errors.Is(err, context.Canceled) {
			if persistErr := markCanceledTaskFailedIfActive(DetachedRequestContext(ctx), s.repo, task.ID, err.Error()); persistErr != nil {
				return task, errors.Join(err, fmt.Errorf("failed to persist canceled task failure: %w", persistErr))
			}
		}
		return task, err
	}
	return dispatched, nil
}

// replayIdempotentGenerateTask resolves an idempotent replay after
// CreateTask failed, which for deterministic source-handoff task IDs means the
// task row already exists. It returns the existing task when the recorded
// payload matches the replayed request, ErrGenerateTaskIdempotencyConflict
// when the same key carries a different target payload, and (nil, nil) when
// no idempotency key is present or no task exists so the caller surfaces the
// original creation error. The caller re-dispatches a matching pending task to
// close the crash window between the task commit and workflow submission; the
// stable task/workflow identity makes that redelivery idempotent.
func (s *taskLifecycleService) replayIdempotentGenerateTask(ctx context.Context, task *Task) (*Task, error) {
	if task == nil || task.Request == nil || strings.TrimSpace(task.Request.IdempotencyKey) == "" {
		return nil, nil
	}
	existing, err := s.repo.GetTask(ctx, task.ID)
	if err != nil {
		return nil, nil
	}
	if existing == nil {
		return nil, nil
	}
	if !generateTaskPayloadsEquivalent(existing, task) {
		return nil, fmt.Errorf("%w: task %s already exists with a different target payload", ErrGenerateTaskIdempotencyConflict, task.ID)
	}
	return existing, nil
}

// generateTaskPayloadsEquivalent compares the complete persisted target
// payload, not just the product identity: a corrected retry that changes
// the store, market, language, category or brand hints, options, text,
// source reference, or the billing/user identity must surface an
// idempotency conflict instead of silently replaying the first task.
func generateTaskPayloadsEquivalent(existing, candidate *Task) bool {
	if existing == nil || candidate == nil {
		return false
	}
	if existing.TenantID != candidate.TenantID ||
		existing.UserID != candidate.UserID ||
		existing.BillingTenantID != candidate.BillingTenantID ||
		existing.SourceSnapshotVersion != candidate.SourceSnapshotVersion {
		return false
	}
	if existing.Request == nil || candidate.Request == nil {
		return existing.Request == nil && candidate.Request == nil
	}
	return generateRequestTargetPayloadEquivalent(existing.Request, candidate.Request)
}

func generateRequestTargetPayloadEquivalent(existing, candidate *GenerateRequest) bool {
	if existing.ProductKey != candidate.ProductKey ||
		existing.Text != candidate.Text ||
		existing.Country != candidate.Country ||
		existing.Language != candidate.Language ||
		existing.SheinStoreID != candidate.SheinStoreID ||
		existing.TargetCategoryHint != candidate.TargetCategoryHint ||
		existing.BrandHint != candidate.BrandHint {
		return false
	}
	if !sourceReferencesEquivalent(existing.Source, candidate.Source) {
		return false
	}
	if !generateOptionsEquivalent(existing.Options, candidate.Options) {
		return false
	}
	return stringSlicesEqualIgnoreOrder(existing.Platforms, candidate.Platforms)
}

func sourceReferencesEquivalent(existing, candidate *SourceReference) bool {
	if existing == nil || candidate == nil {
		return existing == nil && candidate == nil
	}
	return reflect.DeepEqual(*existing, *candidate)
}

func generateOptionsEquivalent(existing, candidate *GenerateOptions) bool {
	if existing == nil || candidate == nil {
		return existing == nil && candidate == nil
	}
	return reflect.DeepEqual(*existing, *candidate)
}

func stringSlicesEqualIgnoreOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, value := range a {
		counts[value]++
	}
	for _, value := range b {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}

func markCanceledTaskFailedIfActive(ctx context.Context, repo Repository, taskID, errorMsg string) error {
	if repo == nil || strings.TrimSpace(taskID) == "" {
		return nil
	}
	current, err := repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if current == nil || (current.Status != core.TaskStatusPending && current.Status != core.TaskStatusProcessing) {
		return nil
	}
	return markFailedTaskState(ctx, repo, taskID, errorMsg)
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
	result := buildTaskResult(task, resultPayload)
	if source, ok := s.repo.(SDSChildRetryJobStatusSource); ok {
		jobs, err := source.ListSDSChildRetries(ctx, task.ID)
		if err != nil {
			return nil, err
		}
		result.ChildRetries = projectSDSChildRetryStatuses(jobs, time.Now().UTC())
	}
	return result, nil
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
