package task

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/infra/worker"
	"task-processor/internal/model"
)

type stubTaskSubmitter struct {
	err error
}

func (s stubTaskSubmitter) SubmitTask(ctx context.Context, taskData string) error {
	return s.err
}

func (s stubTaskSubmitter) GetPlatform() string {
	return "temu"
}

func (s stubTaskSubmitter) GetAvailableSlots() int {
	return 1
}

func (s stubTaskSubmitter) GetQueueStats() worker.QueueStats {
	return worker.QueueStats{}
}

func TestTaskDispatcherDispatchQueueFull(t *testing.T) {
	dispatcher := NewTaskDispatcher(&TaskFetcher{
		submitters: map[string]TaskSubmitter{
			"temu": stubTaskSubmitter{err: worker.ErrQueueFull},
		},
	})

	success, isQueueFull := dispatcher.Dispatch(context.Background(), &ImportTaskRecord{
		ID:        1,
		TenantID:  10,
		StoreID:   20,
		ProductID: "P-1",
		Platform:  "temu",
	}, &StoreInfo{
		ID:       20,
		Platform: "temu",
	})

	if success {
		t.Fatal("Dispatch should not succeed when queue is full")
	}
	if !isQueueFull {
		t.Fatal("Dispatch should report queue full")
	}
}

func TestTaskDispatcherDispatchMissingSubmitter(t *testing.T) {
	dispatcher := NewTaskDispatcher(&TaskFetcher{
		submitters: map[string]TaskSubmitter{},
	})

	success, isQueueFull := dispatcher.Dispatch(context.Background(), &ImportTaskRecord{
		ID:        2,
		TenantID:  10,
		StoreID:   30,
		ProductID: "P-2",
		Platform:  "shein",
	}, &StoreInfo{
		ID:       30,
		Platform: "shein",
	})

	if success {
		t.Fatal("Dispatch should fail when submitter is missing")
	}
	if isQueueFull {
		t.Fatal("Dispatch should not report queue full when submitter is missing")
	}
}

func TestRollbackClaimStateRestoresRemoteStatusAndLocalMarker(t *testing.T) {
	client := &stubImportTaskStatusClient{}
	fetcher := &TaskFetcher{
		processingTasks: map[string]time.Time{"404": time.Now()},
		statusServiceFactory: func(component string) *taskstatus.Service {
			return taskstatus.NewService(component, func() taskstatus.ImportTaskStatusClient {
				return client
			})
		},
	}

	task := &ImportTaskRecord{
		ID:           404,
		Status:       model.TaskStatusPendingRetry.Int16(),
		ErrorMessage: "previous error",
	}

	fetcher.rollbackClaimState("404", task, "queue full")

	if _, exists := fetcher.processingTasks["404"]; exists {
		t.Fatal("rollbackClaimState should clear local processing mark")
	}
	if len(client.updates) != 1 {
		t.Fatalf("rollbackClaimState updates = %d, want 1", len(client.updates))
	}
	if client.updates[0].Status != model.TaskStatusPendingRetry.Int16() {
		t.Fatalf("rollbackClaimState status = %d, want %d", client.updates[0].Status, model.TaskStatusPendingRetry.Int16())
	}
	if client.updates[0].ErrorMessage != "previous error" {
		t.Fatalf("rollbackClaimState errorMessage = %q, want previous error", client.updates[0].ErrorMessage)
	}
}
