package store

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/productimage"
	"task-processor/internal/shared/aiidentity"
)

func TestTaskRepositoryScopesReadsToExecutionTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&productimage.Task{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewTaskRepository(db).(*taskRepository)
	for _, task := range []*productimage.Task{
		{ID: "task-a", Status: productimage.TaskStatusPending, TenantID: "tenant-a", UserID: "user-a", PersistedExecutionEnvelope: aiidentity.PersistedExecutionEnvelope{ExecutionIdentityVersion: 1, ExecutionTenantID: "tenant-a", ExecutionUserID: "user-a", ExecutionSourcePlatform: "productimage", ExecutionSourceTaskType: "image"}},
		{ID: "task-b", Status: productimage.TaskStatusPending, TenantID: "tenant-b", UserID: "user-b", PersistedExecutionEnvelope: aiidentity.PersistedExecutionEnvelope{ExecutionIdentityVersion: 1, ExecutionTenantID: "tenant-b", ExecutionUserID: "user-b", ExecutionSourcePlatform: "productimage", ExecutionSourceTaskType: "image"}},
	} {
		if err := repo.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("create %s: %v", task.ID, err)
		}
	}
	ctxA := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := repo.GetTask(ctxA, "task-b"); !errors.Is(err, productimage.ErrTaskNotFound) {
		t.Fatalf("cross-tenant GetTask error = %v, want ErrTaskNotFound", err)
	}
}
