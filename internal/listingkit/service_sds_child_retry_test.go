package listingkit

import (
	"context"
	"testing"
	"time"
)

type sdsChildRetryTestRepository struct {
	Repository
	jobs map[string]SDSChildRetryJob
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

func TestScheduleStudioBatchSDSChildRetriesQueuesOnlyFailedSDSChildren(t *testing.T) {
	ctx := context.Background()
	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest()}
	failed := &Task{
		ID:       "task-failed",
		TenantID: "tenant-1",
		Request:  &GenerateRequest{SheinStoreID: 177},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{
			Kind: string(SDSChildRetryKindDesignSync), Status: string(TaskStatusFailed), Error: "upload timed out",
		}}},
	}
	completed := &Task{
		ID: "task-completed", Request: &GenerateRequest{SheinStoreID: 177},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: string(SDSChildRetryKindDesignSync), Status: string(TaskStatusCompleted)}}},
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
