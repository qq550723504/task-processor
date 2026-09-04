package amazonlisting

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"

	worker "task-processor/internal/platform/workerpool"
	"task-processor/internal/shared/aiidentity"
)

func TestProcessorMissingExecutionEnvelopeFailsClosed(t *testing.T) {
	repo := &stubRepository{}
	task := &Task{ID: "missing-envelope", Status: TaskStatusPending, Request: &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	service, err := NewService(&ServiceConfig{Repository: repo})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	processor, err := NewProcessor(service, repo, logrus.New())
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: task.ID})
	if !errors.Is(err, aiidentity.ErrMissingIdentity) {
		t.Fatalf("error = %v, want ErrMissingIdentity", err)
	}
	updated, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask: %v", getErr)
	}
	if updated.Status != TaskStatusFailed {
		t.Fatalf("status = %q, want failed", updated.Status)
	}
	if updated.Error == "" {
		t.Fatal("expected identity-integrity error to be persisted")
	}
}

func TestProcessorDoesNotReopenFailedTaskForAutomaticRetry(t *testing.T) {
	repo := &stubRepository{}
	task := &Task{ID: "snapshot-not-ready", Status: TaskStatusPending, Request: &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"}}
	task.SetExecutionEnvelope(aiidentity.ExecutionEnvelope{
		Version: aiidentity.CurrentEnvelopeVersion, TenantID: "tenant-a", UserID: "user-a",
		BusinessTaskID: task.ID, SourcePlatform: "amazon", SourceTaskType: "listing",
	})
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	service, err := NewService(&ServiceConfig{Repository: repo})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	processor, err := NewProcessor(service, repo, logrus.New())
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}
	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: task.ID})
	if !errors.Is(err, ErrProductSnapshotNotReady) {
		t.Fatalf("ProcessTask() error = %v, want ErrProductSnapshotNotReady", err)
	}
	stored, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask: %v", getErr)
	}
	if stored.Status != TaskStatusFailed || stored.RetryCount != 0 {
		t.Fatalf("stored task = status %q retry_count %d, want terminal failed without retry", stored.Status, stored.RetryCount)
	}
}

func TestProcessListingReturnsWorkflowAndFailurePersistenceErrors(t *testing.T) {
	workflowErr := errors.New("snapshot read failed")
	persistErr := errors.New("mark failed unavailable")
	repo := &stubRepository{markFailedErr: persistErr}
	task := newWorkflowTestTask("tenant-a", "product-1")
	task.Status = TaskStatusPending
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	service, err := NewService(&ServiceConfig{
		Repository:            repo,
		ProductSnapshotReader: &stubWorkflowProductSnapshotReader{err: workflowErr},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.ProcessListing(context.Background(), task)
	if !errors.Is(err, workflowErr) || !errors.Is(err, persistErr) {
		t.Fatalf("ProcessListing() error = %v, want workflow and persistence errors", err)
	}
}

func TestProcessorReturnsIdentityAndFailurePersistenceErrors(t *testing.T) {
	persistErr := errors.New("mark identity failure unavailable")
	repo := &stubRepository{markFailedErr: persistErr}
	task := &Task{ID: "missing-envelope-persist-error", Status: TaskStatusPending, Request: &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	service, err := NewService(&ServiceConfig{Repository: repo})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	processor, err := NewProcessor(service, repo, logrus.New())
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: task.ID})
	if !errors.Is(err, aiidentity.ErrMissingIdentity) || !errors.Is(err, persistErr) {
		t.Fatalf("ProcessTask() error = %v, want identity and persistence errors", err)
	}
}

func TestManualRetryRecoversTaskWhenFailureStateCouldNotBePersisted(t *testing.T) {
	workflowErr := errors.New("snapshot read failed")
	repo := &stubRepository{markFailedErr: errors.New("mark failed unavailable")}
	task := newWorkflowTestTask("tenant-a", "product-1")
	task.Status = TaskStatusPending
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	service, err := NewService(&ServiceConfig{
		Repository:            repo,
		ProductSnapshotReader: &stubWorkflowProductSnapshotReader{err: workflowErr},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.ProcessListing(context.Background(), task)
	if !errors.Is(err, workflowErr) {
		t.Fatalf("ProcessListing() error = %v, want workflow error", err)
	}
	stored, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask: %v", getErr)
	}
	if stored.Status != TaskStatusProcessing {
		t.Fatalf("status after failed terminal persistence = %q, want processing", stored.Status)
	}

	repo.markFailedErr = nil
	if _, err := service.ReviewTask(context.Background(), task.ID, &ReviewTaskRequest{Action: "retry"}); err != nil {
		t.Fatalf("ReviewTask(retry): %v", err)
	}
	stored, getErr = repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask after retry: %v", getErr)
	}
	if stored.Status != TaskStatusPending {
		t.Fatalf("status after manual retry = %q, want pending", stored.Status)
	}
}
