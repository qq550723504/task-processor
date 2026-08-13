package listingkit

import (
	"context"
	"task-processor/internal/listingkit/core"
	"testing"
	"time"
)

type sdsChildRetryTestRepository struct {
	Repository
	jobs     map[string]SDSChildRetryJob
	retryCtx context.Context
}

func (r *sdsChildRetryTestRepository) GetTask(ctx context.Context, taskID string) (*Task, error) {
	r.retryCtx = ctx
	return r.Repository.GetTask(ctx, taskID)
}

func (r *sdsChildRetryTestRepository) ScheduleSDSChildRetry(_ context.Context, job *SDSChildRetryJob) (*SDSChildRetryJob, error) {
	if r.jobs == nil {
		r.jobs = make(map[string]SDSChildRetryJob)
	}
	for _, existing := range r.jobs {
		if existing.TaskID == job.TaskID && existing.Kind == job.Kind {
			copy := existing
			return &copy, nil
		}
	}
	copy := *job
	copy.ID = "job-" + job.TaskID
	r.jobs[copy.ID] = copy
	return &copy, nil
}

func (r *sdsChildRetryTestRepository) ListDueSDSChildRetries(context.Context, time.Time, int) ([]SDSChildRetryJob, error) {
	return nil, nil
}

func (r *sdsChildRetryTestRepository) ClaimDueSDSChildRetries(context.Context, time.Time, int, string, time.Time) ([]SDSChildRetryJob, error) {
	return nil, nil
}

func (r *sdsChildRetryTestRepository) SaveSDSChildRetry(context.Context, *SDSChildRetryJob) error {
	return nil
}

func (r *sdsChildRetryTestRepository) PrepareSDSChildRetryRepair(_ context.Context, taskID string, kind SDSChildRetryKind) error {
	now := time.Now()
	for id, job := range r.jobs {
		if job.TaskID != taskID || job.Kind != kind {
			continue
		}
		if job.Status == SDSChildRetryJobStatusPending && job.LeaseUntil != nil && job.LeaseUntil.After(now) {
			return ErrSDSRepairRetryInProgress
		}
		if job.Status == SDSChildRetryJobStatusPending || job.Status == SDSChildRetryJobStatusExhausted {
			job.Status = SDSChildRetryJobStatusCancelled
			job.LeaseOwner = ""
			job.LeaseUntil = nil
			r.jobs[id] = job
		}
	}
	return nil
}

func (r *sdsChildRetryTestRepository) ReplaceTaskSDSOptionsForRetry(ctx context.Context, taskID string, options *SDSSyncOptions, audit PodExecutionAuditEvent) (*Task, error) {
	return r.Repository.(TaskSDSRepairRepository).ReplaceTaskSDSOptionsForRetry(ctx, taskID, options, audit)
}

func (r *sdsChildRetryTestRepository) ListSDSChildRetries(_ context.Context, taskID string) ([]SDSChildRetryJob, error) {
	jobs := make([]SDSChildRetryJob, 0)
	for _, job := range r.jobs {
		if job.TaskID == taskID {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func TestScheduleStudioBatchSDSChildRetriesQueuesOnlyFailedSDSChildren(t *testing.T) {
	ctx := context.Background()
	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest()}
	failed := &Task{
		ID:       "task-failed",
		TenantID: "tenant-1",
		Request:  &GenerateRequest{SheinStoreID: 177},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{
			Kind: string(SDSChildRetryKindDesignSync), Status: string(core.TaskStatusFailed), Error: "upload timed out",
		}}},
	}
	completed := &Task{
		ID: "task-completed", Request: &GenerateRequest{SheinStoreID: 177},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: string(SDSChildRetryKindDesignSync), Status: string(core.TaskStatusCompleted)}}},
	}
	if err := repo.CreateTask(ctx, failed); err != nil {
		t.Fatalf("create failed task: %v", err)
	}
	if err := repo.CreateTask(ctx, completed); err != nil {
		t.Fatalf("create completed task: %v", err)
	}
	links := NewMemStudioBatchTaskLinkRepository()
	for _, taskID := range []string{failed.ID, completed.ID} {
		if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
			ID: taskID + "-link", BatchID: "batch-1", ItemID: taskID + "-item", DesignID: taskID + "-design",
			ListingKitTaskID: taskID, CandidateKey: taskID + "-candidate", Status: studioBatchTaskLinkStatusCreated, SheinStoreID: 177,
		}); err != nil {
			t.Fatalf("create task link %q: %v", taskID, err)
		}
	}

	svc := &service{repo: repo}
	svc.SetStudioBatchTaskLinkRepository(links)
	result, err := svc.ScheduleStudioBatchSDSChildRetries(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ScheduleStudioBatchSDSChildRetries() error = %v", err)
	}
	if result.Scheduled != 1 || result.Skipped != 1 || len(result.Failures) != 0 {
		t.Fatalf("result = %#v, want one scheduled and one skipped", result)
	}
	job, ok := repo.jobs["job-task-failed"]
	if !ok {
		t.Fatalf("scheduled jobs = %#v, want task-failed", repo.jobs)
	}
	if job.ReasonCode != "manual_studio_batch_sds_retry" || job.NextRetryAt.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("job = %#v, want immediate manual retry", job)
	}
}

func TestScheduleTaskChildRetryQueuesSDSDesignSyncWithoutRunningRemoteWork(t *testing.T) {
	ctx := context.Background()
	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest()}
	task := &Task{
		ID:       "task-manual-retry",
		TenantID: "tenant-1",
		Status:   core.TaskStatusNeedsReview,
		Request:  &GenerateRequest{SheinStoreID: 1038},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{
			Kind: string(SDSChildRetryKindDesignSync), Status: string(core.TaskStatusFailed), Error: "SHEIN cookie unavailable",
		}}},
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := (&service{repo: repo}).ScheduleTaskChildRetry(ctx, task.ID, &RetryChildTaskRequest{Kind: string(SDSChildRetryKindDesignSync)})
	if err != nil {
		t.Fatalf("ScheduleTaskChildRetry() error = %v", err)
	}
	if result == nil || result.TaskID != task.ID || result.Kind != string(SDSChildRetryKindDesignSync) || result.Status != "queued" {
		t.Fatalf("result = %#v, want queued retry acknowledgement", result)
	}
	if len(repo.jobs) != 1 {
		t.Fatalf("scheduled jobs = %#v, want one job", repo.jobs)
	}
	for _, job := range repo.jobs {
		if job.ReasonCode != "manual_child_task_retry" {
			t.Fatalf("job reason code = %q, want manual_child_task_retry", job.ReasonCode)
		}
		if job.NextRetryAt.After(time.Now().UTC().Add(time.Second)) {
			t.Fatalf("job next retry at = %s, want immediate scheduling", job.NextRetryAt)
		}
	}
}

func TestScheduleTaskChildRetryQueuesSDSCatalogProduct(t *testing.T) {
	ctx := context.Background()
	const catalogKind = SDSChildRetryKindCatalogProduct
	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest()}
	task := &Task{
		ID:       "task-catalog-retry",
		TenantID: "tenant-1",
		Status:   core.TaskStatusNeedsReview,
		Request:  &GenerateRequest{SheinStoreID: 1038},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{
			Kind: string(catalogKind), Status: string(core.TaskStatusFailed), Error: "catalog failed",
		}}},
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := (&service{repo: repo}).ScheduleTaskChildRetry(ctx, task.ID, &RetryChildTaskRequest{Kind: string(catalogKind)})
	if err != nil {
		t.Fatalf("ScheduleTaskChildRetry() error = %v", err)
	}
	if result == nil || result.TaskID != task.ID || result.Kind != string(catalogKind) || result.Status != "queued" {
		t.Fatalf("result = %#v, want queued catalog retry acknowledgement", result)
	}
	job, ok := repo.jobs["job-"+task.ID]
	if !ok || job.Kind != catalogKind {
		t.Fatalf("scheduled jobs = %#v, want catalog product retry", repo.jobs)
	}
}

func TestRunSDSChildRetryRestoresTenantContext(t *testing.T) {
	ctx := context.Background()
	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest()}
	task := &Task{
		ID:       "task-tenant-retry",
		TenantID: "tenant-policy-1",
		Status:   core.TaskStatusNeedsReview,
		Request:  &GenerateRequest{SheinStoreID: 1038},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{
			Kind: string(SDSChildRetryKindDesignSync), Status: string(core.TaskStatusFailed), Error: "retry failed",
		}}},
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	job := &SDSChildRetryJob{TaskID: task.ID, TenantID: task.TenantID, Kind: SDSChildRetryKindDesignSync}
	if err := (&service{repo: repo}).runSDSChildRetry(ctx, job); err != nil {
		t.Fatalf("runSDSChildRetry() error = %v", err)
	}
	if got := TenantIDFromContext(repo.retryCtx); got != task.TenantID {
		t.Fatalf("retry tenant context = %q, want %q", got, task.TenantID)
	}
}

func TestRunSDSChildRetryPreservesDomainFailure(t *testing.T) {
	ctx := context.Background()
	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest()}
	task := &Task{
		ID:       "task-domain-failure",
		TenantID: "tenant-1",
		Status:   core.TaskStatusNeedsReview,
		Request:  &GenerateRequest{Options: &GenerateOptions{}},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{
			Kind: string(SDSChildRetryKindDesignSync), Status: string(core.TaskStatusFailed),
		}}},
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	job := &SDSChildRetryJob{TaskID: task.ID, TenantID: task.TenantID, Kind: SDSChildRetryKindDesignSync}
	if err := (&service{repo: repo}).runSDSChildRetry(ctx, job); err != nil {
		t.Fatalf("runSDSChildRetry() error = %v", err)
	}
	if job.LastError != core.ErrChildTaskNotRetryable.Error() {
		t.Fatalf("job.LastError = %q, want domain error %q", job.LastError, core.ErrChildTaskNotRetryable.Error())
	}
}

func TestGetTaskResultIncludesDurableSDSChildRetryStatus(t *testing.T) {
	ctx := context.Background()
	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest(), jobs: map[string]SDSChildRetryJob{
		"job-task-retry-status": {
			ID:        "job-task-retry-status",
			TaskID:    "task-retry-status",
			Kind:      SDSChildRetryKindDesignSync,
			Status:    SDSChildRetryJobStatusExhausted,
			Attempt:   3,
			LastError: "SDS options are missing",
		},
	}}
	task := &Task{
		ID:       "task-retry-status",
		Status:   core.TaskStatusNeedsReview,
		Request:  &GenerateRequest{},
		Result:   &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: string(SDSChildRetryKindDesignSync), Status: string(core.TaskStatusFailed)}}},
		TenantID: "tenant-1",
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := (&service{repo: repo}).GetTaskResult(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskResult() error = %v", err)
	}
	if len(result.ChildRetries) != 1 || result.ChildRetries[0].Status != string(SDSChildRetryJobStatusExhausted) || result.ChildRetries[0].LastError != "SDS options are missing" {
		t.Fatalf("child retries = %#v, want durable exhausted status and error", result.ChildRetries)
	}
}
