package task

import (
	"testing"
	"time"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
)

func TestTaskClaimServiceClaimRejectsInvalidTransition(t *testing.T) {
	fetcher := &TaskFetcher{
		processingTasks: make(map[string]time.Time),
	}
	service := NewTaskClaimService(fetcher)

	task := &managementapi.ProductImportTaskRespDTO{
		ID:     101,
		Status: model.TaskStatusPublished.Int16(),
	}

	taskID, ok := service.Claim(task)
	if ok {
		t.Fatal("Claim should reject published task")
	}
	if taskID != "101" {
		t.Fatalf("Claim taskID = %s, want 101", taskID)
	}
	if len(fetcher.processingTasks) != 0 {
		t.Fatal("Claim should not mark invalid task as processing")
	}
}

func TestTaskClaimServiceClaimMarksProcessing(t *testing.T) {
	fetcher := &TaskFetcher{
		processingTasks: make(map[string]time.Time),
	}
	service := NewTaskClaimService(fetcher)

	task := &managementapi.ProductImportTaskRespDTO{
		ID:     202,
		Status: model.TaskStatusPending.Int16(),
	}

	taskID, ok := service.Claim(task)
	if !ok {
		t.Fatal("Claim should accept pending task")
	}
	if taskID != "202" {
		t.Fatalf("Claim taskID = %s, want 202", taskID)
	}
	if _, exists := fetcher.processingTasks["202"]; !exists {
		t.Fatal("Claim should mark task as processing")
	}
}
