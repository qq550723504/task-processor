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
		{ID: "task-authoritative", Status: productimage.TaskStatusPending, TenantID: "tenant-a", UserID: "legacy-user", PersistedExecutionEnvelope: aiidentity.PersistedExecutionEnvelope{ExecutionIdentityVersion: 1, ExecutionTenantID: "tenant-b", ExecutionUserID: "user-b", ExecutionSourcePlatform: "productimage", ExecutionSourceTaskType: "image"}},
		{ID: "task-legacy", Status: productimage.TaskStatusPending, TenantID: "tenant-a", UserID: "legacy-user"},
	} {
		if err := repo.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("create %s: %v", task.ID, err)
		}
	}
	ctxA := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := repo.GetTask(ctxA, "task-b"); !errors.Is(err, productimage.ErrTaskNotFound) {
		t.Fatalf("cross-tenant GetTask error = %v, want ErrTaskNotFound", err)
	}
	if _, err := repo.GetTask(ctxA, "task-authoritative"); !errors.Is(err, productimage.ErrTaskNotFound) {
		t.Fatalf("legacy tenant accessed envelope-owned task: error = %v, want ErrTaskNotFound", err)
	}
	if task, err := repo.GetTask(ctxA, "task-legacy"); err != nil || task.ID != "task-legacy" {
		t.Fatalf("legacy tenant fallback GetTask = (%+v, %v), want task-legacy", task, err)
	}
	if err := repo.IncrementRetryCount(ctxA, "task-authoritative"); err == nil {
		t.Fatal("legacy tenant mutation succeeded for envelope-owned task")
	}
	var authoritative productimage.Task
	if err := db.First(&authoritative, "id = ?", "task-authoritative").Error; err != nil {
		t.Fatalf("load authoritative task: %v", err)
	}
	if authoritative.RetryCount != 0 {
		t.Fatalf("authoritative task retry count = %d, want 0", authoritative.RetryCount)
	}
	ctxB := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-b", UserID: "user-b"})
	if task, err := repo.GetTask(ctxB, "task-authoritative"); err != nil || task.ID != "task-authoritative" {
		t.Fatalf("execution tenant GetTask = (%+v, %v), want task-authoritative", task, err)
	}
}

func TestMemTaskRepositoryUsesEnvelopeTenantWithLegacyFallback(t *testing.T) {
	repo := NewMemTaskRepository()
	for _, task := range []*productimage.Task{
		{ID: "task-a", Status: productimage.TaskStatusPending, TenantID: "tenant-a", UserID: "user-a", PersistedExecutionEnvelope: aiidentity.PersistedExecutionEnvelope{ExecutionIdentityVersion: 1, ExecutionTenantID: "tenant-a", ExecutionUserID: "user-a", ExecutionSourcePlatform: "productimage", ExecutionSourceTaskType: "image"}},
		{ID: "task-b", Status: productimage.TaskStatusPending, TenantID: "tenant-b", UserID: "user-b", PersistedExecutionEnvelope: aiidentity.PersistedExecutionEnvelope{ExecutionIdentityVersion: 1, ExecutionTenantID: "tenant-b", ExecutionUserID: "user-b", ExecutionSourcePlatform: "productimage", ExecutionSourceTaskType: "image"}},
		{ID: "task-authoritative", Status: productimage.TaskStatusPending, TenantID: "tenant-a", UserID: "legacy-user", PersistedExecutionEnvelope: aiidentity.PersistedExecutionEnvelope{ExecutionIdentityVersion: 1, ExecutionTenantID: "tenant-b", ExecutionUserID: "user-b", ExecutionSourcePlatform: "productimage", ExecutionSourceTaskType: "image"}},
		{ID: "task-legacy", Status: productimage.TaskStatusPending, TenantID: "tenant-a", UserID: "legacy-user"},
	} {
		if err := repo.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("create %s: %v", task.ID, err)
		}
	}
	ctxA := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	for _, taskID := range []string{"task-b", "task-authoritative"} {
		if _, err := repo.GetTask(ctxA, taskID); !errors.Is(err, productimage.ErrTaskNotFound) {
			t.Fatalf("cross-tenant GetTask(%s) error = %v, want ErrTaskNotFound", taskID, err)
		}
	}
	if task, err := repo.GetTask(ctxA, "task-legacy"); err != nil || task.ID != "task-legacy" {
		t.Fatalf("legacy tenant fallback GetTask = (%+v, %v), want task-legacy", task, err)
	}
	if err := repo.IncrementRetryCount(ctxA, "task-authoritative"); err == nil {
		t.Fatal("legacy tenant mutation succeeded for envelope-owned task")
	}
	stored, err := repo.GetTask(context.Background(), "task-authoritative")
	if err != nil {
		t.Fatalf("unscoped GetTask: %v", err)
	}
	if stored.RetryCount != 0 {
		t.Fatalf("cross-tenant retry count = %d, want 0", stored.RetryCount)
	}
}
