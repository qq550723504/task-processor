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
	task := &Task{ID: "missing-envelope", Status: TaskStatusPending, Request: &GenerateRequest{Marketplace: "amazon", ProductURL: "https://example.com/product"}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	service, err := NewService(&ServiceConfig{Repository: repo, ProductService: &stubProductService{}})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	processor, err := NewProcessor(service, repo, logrus.New(), 2)
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
