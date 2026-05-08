package store_test

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
)

func TestMemTaskRepositoryScopesTasksByTenant(t *testing.T) {
	repo := store.NewMemTaskRepository()
	ctxA := listingkit.WithTenantID(context.Background(), "tenant-a")
	ctxB := listingkit.WithTenantID(context.Background(), "tenant-b")

	if err := repo.CreateTask(ctxA, &listingkit.Task{ID: "task-a", Status: listingkit.TaskStatusPending, Request: &listingkit.GenerateRequest{Text: "a"}}); err != nil {
		t.Fatalf("CreateTask tenant-a: %v", err)
	}
	if err := repo.CreateTask(ctxB, &listingkit.Task{ID: "task-b", Status: listingkit.TaskStatusPending, Request: &listingkit.GenerateRequest{Text: "b"}}); err != nil {
		t.Fatalf("CreateTask tenant-b: %v", err)
	}

	tasks, total, err := repo.ListTasks(ctxA, &listingkit.TaskListQuery{})
	if err != nil {
		t.Fatalf("ListTasks tenant-a: %v", err)
	}
	if total != 1 || len(tasks) != 1 || tasks[0].ID != "task-a" || tasks[0].TenantID != "tenant-a" {
		t.Fatalf("tenant-a list = total %d tasks %#v", total, tasks)
	}

	if _, err := repo.GetTask(ctxB, "task-a"); !errors.Is(err, listingkit.ErrTaskNotFound) {
		t.Fatalf("GetTask cross tenant error = %v, want ErrTaskNotFound", err)
	}
	if err := repo.MarkFailed(ctxB, "task-a", "failed"); !errors.Is(err, listingkit.ErrTaskNotFound) {
		t.Fatalf("MarkFailed cross tenant error = %v, want ErrTaskNotFound", err)
	}
}
