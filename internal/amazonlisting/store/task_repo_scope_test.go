package store

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/shared/aiidentity"
)

func TestTaskRepositoryScopesReadsAndMutationsToExecutionTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&amazonlisting.Task{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewTaskRepository(db).(*taskRepository)
	if err := repo.CreateTask(context.Background(), &amazonlisting.Task{
		ID: "task-a", Status: amazonlisting.TaskStatusPending,
		PersistedExecutionEnvelope: aiidentity.PersistedExecutionEnvelope{ExecutionIdentityVersion: 1, ExecutionTenantID: "tenant-a", ExecutionUserID: "user-a", ExecutionSourcePlatform: "amazon", ExecutionSourceTaskType: "listing"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.CreateTask(context.Background(), &amazonlisting.Task{
		ID: "task-b", Status: amazonlisting.TaskStatusPending,
		PersistedExecutionEnvelope: aiidentity.PersistedExecutionEnvelope{ExecutionIdentityVersion: 1, ExecutionTenantID: "tenant-b", ExecutionUserID: "user-b", ExecutionSourcePlatform: "amazon", ExecutionSourceTaskType: "listing"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	ctxA := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := repo.GetTask(ctxA, "task-b"); !errors.Is(err, amazonlisting.ErrTaskNotFound) {
		t.Fatalf("cross-tenant GetTask error = %v, want ErrTaskNotFound", err)
	}
	items, err := repo.ListTasks(ctxA, nil, 0)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(items) != 1 || items[0].ID != "task-a" {
		t.Fatalf("tenant-scoped tasks = %+v, want only task-a", items)
	}
	if err := repo.IncrementRetryCount(ctxA, "task-b"); !errors.Is(err, amazonlisting.ErrTaskNotFound) {
		t.Fatalf("cross-tenant mutation error = %v, want ErrTaskNotFound", err)
	}
	var task amazonlisting.Task
	if err := db.First(&task, "id = ?", "task-b").Error; err != nil {
		t.Fatalf("load task-b: %v", err)
	}
	if task.Status != amazonlisting.TaskStatusPending || task.RetryCount != 0 {
		t.Fatalf("cross-tenant mutation changed task: status=%q retries=%d", task.Status, task.RetryCount)
	}
}
