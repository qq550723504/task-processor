package task

import (
	"context"
	"testing"

	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/worker"
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

	success, isQueueFull := dispatcher.Dispatch(context.Background(), &api.ProductImportTaskRespDTO{
		ID:        1,
		TenantID:  10,
		StoreID:   20,
		ProductID: "P-1",
		Platform:  "temu",
	}, &api.StoreRespDTO{
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

	success, isQueueFull := dispatcher.Dispatch(context.Background(), &api.ProductImportTaskRespDTO{
		ID:        2,
		TenantID:  10,
		StoreID:   30,
		ProductID: "P-2",
		Platform:  "shein",
	}, &api.StoreRespDTO{
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
