// Package main 测试内存回退实现（委托给 productenrich 包）
package main

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/productenrich"
)

// 本文件的测试直接使用 productenrich 包导出的内存实现，
// 不再依赖 adapters.go 中的本地函数。
func newMemRedisClient() productenrich.RedisClient       { return productenrich.NewMemRedisClient() }
func newMemTaskRepository() productenrich.TaskRepository { return productenrich.NewMemTaskRepository() }

// =============================================================================
// memRedisClient 单元测试
// =============================================================================

func TestMemRedisClient_SetAndGet(t *testing.T) {
	rc := newMemRedisClient()
	ctx := context.Background()

	if err := rc.Set(ctx, "k1", "v1", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := rc.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "v1" {
		t.Errorf("Get = %q, want %q", got, "v1")
	}
}

func TestMemRedisClient_Get_NotFound(t *testing.T) {
	rc := newMemRedisClient()
	_, err := rc.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestMemRedisClient_Delete(t *testing.T) {
	rc := newMemRedisClient()
	ctx := context.Background()

	_ = rc.Set(ctx, "k2", "v2", 0)
	if err := rc.Delete(ctx, "k2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := rc.Get(ctx, "k2"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestMemRedisClient_Push(t *testing.T) {
	rc := newMemRedisClient()
	ctx := context.Background()

	if err := rc.Push(ctx, "queue", "task-1"); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if err := rc.Push(ctx, "queue", "task-2"); err != nil {
		t.Fatalf("Push second: %v", err)
	}
	// 内存实现不提供 Pop，只验证 Push 不报错
}

func TestMemRedisClient_TTL_Expiry(t *testing.T) {
	rc := newMemRedisClient()
	ctx := context.Background()

	if err := rc.Set(ctx, "ttl-key", "val", 50*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// 过期前可以读到
	if _, err := rc.Get(ctx, "ttl-key"); err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := rc.Get(ctx, "ttl-key"); err == nil {
		t.Error("expected error after TTL expiry")
	}
}

func TestMemRedisClient_Overwrite(t *testing.T) {
	rc := newMemRedisClient()
	ctx := context.Background()

	_ = rc.Set(ctx, "k", "old", 0)
	_ = rc.Set(ctx, "k", "new", 0)

	got, err := rc.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "new" {
		t.Errorf("Get = %q, want %q", got, "new")
	}
}

// =============================================================================
// memTaskRepository 单元测试
// =============================================================================

func TestMemTaskRepository_CreateAndGet(t *testing.T) {
	repo := newMemTaskRepository()
	ctx := context.Background()

	task := &productenrich.Task{
		ID:      "t-001",
		Request: &productenrich.GenerateRequest{Text: "test"},
		Status:  productenrich.TaskStatusPending,
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	got, err := repo.GetTask(ctx, "t-001")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.ID != "t-001" {
		t.Errorf("ID = %q, want %q", got.ID, "t-001")
	}
	if got.Status != productenrich.TaskStatusPending {
		t.Errorf("Status = %q, want pending", got.Status)
	}
}

func TestMemTaskRepository_GetTask_NotFound(t *testing.T) {
	repo := newMemTaskRepository()
	_, err := repo.GetTask(context.Background(), "nonexistent")
	if err != productenrich.ErrTaskNotFound {
		t.Errorf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestMemTaskRepository_CreateTask_NilTask(t *testing.T) {
	repo := newMemTaskRepository()
	if err := repo.CreateTask(context.Background(), nil); err == nil {
		t.Error("expected error for nil task")
	}
}

func TestMemTaskRepository_UpdateTaskStatus(t *testing.T) {
	repo := newMemTaskRepository()
	ctx := context.Background()

	task := &productenrich.Task{ID: "t-002", Request: &productenrich.GenerateRequest{Text: "x"}, Status: productenrich.TaskStatusPending}
	_ = repo.CreateTask(ctx, task)

	if err := repo.UpdateTaskStatus(ctx, "t-002", productenrich.TaskStatusProcessing); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}
	got, _ := repo.GetTask(ctx, "t-002")
	if got.Status != productenrich.TaskStatusProcessing {
		t.Errorf("Status = %q, want processing", got.Status)
	}
}

func TestMemTaskRepository_UpdateTaskError(t *testing.T) {
	repo := newMemTaskRepository()
	ctx := context.Background()

	task := &productenrich.Task{ID: "t-003", Request: &productenrich.GenerateRequest{Text: "x"}, Status: productenrich.TaskStatusProcessing}
	_ = repo.CreateTask(ctx, task)

	if err := repo.UpdateTaskError(ctx, "t-003", "something failed"); err != nil {
		t.Fatalf("UpdateTaskError: %v", err)
	}
	got, _ := repo.GetTask(ctx, "t-003")
	if got.Status != productenrich.TaskStatusFailed {
		t.Errorf("Status = %q, want failed", got.Status)
	}
	if got.Error != "something failed" {
		t.Errorf("Error = %q, want %q", got.Error, "something failed")
	}
}

func TestMemTaskRepository_SaveTaskResult(t *testing.T) {
	repo := newMemTaskRepository()
	ctx := context.Background()

	task := &productenrich.Task{ID: "t-004", Request: &productenrich.GenerateRequest{Text: "x"}, Status: productenrich.TaskStatusProcessing}
	_ = repo.CreateTask(ctx, task)

	result := &productenrich.ProductJSON{Title: "Widget", Description: "A fine widget"}
	if err := repo.SaveTaskResult(ctx, "t-004", result); err != nil {
		t.Fatalf("SaveTaskResult: %v", err)
	}
	got, _ := repo.GetTask(ctx, "t-004")
	if got.Status != productenrich.TaskStatusCompleted {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if got.Result == nil || got.Result.Title != "Widget" {
		t.Errorf("Result.Title = %q, want Widget", got.Result.Title)
	}
}

func TestMemTaskRepository_IncrementRetryCount(t *testing.T) {
	repo := newMemTaskRepository()
	ctx := context.Background()

	task := &productenrich.Task{ID: "t-005", Request: &productenrich.GenerateRequest{Text: "x"}, Status: productenrich.TaskStatusFailed}
	_ = repo.CreateTask(ctx, task)

	_ = repo.IncrementRetryCount(ctx, "t-005")
	_ = repo.IncrementRetryCount(ctx, "t-005")

	got, _ := repo.GetTask(ctx, "t-005")
	if got.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", got.RetryCount)
	}
}

func TestMemTaskRepository_ResetForRetry_PreservesError(t *testing.T) {
	repo := newMemTaskRepository()
	ctx := context.Background()

	task := &productenrich.Task{
		ID:      "t-006",
		Request: &productenrich.GenerateRequest{Text: "x"},
		Status:  productenrich.TaskStatusFailed,
		Error:   "previous error",
	}
	_ = repo.CreateTask(ctx, task)

	if err := repo.ResetForRetry(ctx, "t-006"); err != nil {
		t.Fatalf("ResetForRetry: %v", err)
	}
	got, _ := repo.GetTask(ctx, "t-006")
	if got.Status != productenrich.TaskStatusPending {
		t.Errorf("Status = %q, want pending", got.Status)
	}
	// error 字段必须保留，不能被清空
	if got.Error != "previous error" {
		t.Errorf("Error = %q, want %q", got.Error, "previous error")
	}
}

func TestMemTaskRepository_ConcurrentAccess(t *testing.T) {
	repo := newMemTaskRepository()
	ctx := context.Background()

	// 并发写入，验证不会 data race（需配合 -race 运行）
	const n = 50
	done := make(chan struct{}, n)
	for i := range n {
		go func(i int) {
			id := "concurrent-" + string(rune('a'+i%26))
			task := &productenrich.Task{
				ID:      id,
				Request: &productenrich.GenerateRequest{Text: "x"},
				Status:  productenrich.TaskStatusPending,
			}
			_ = repo.CreateTask(ctx, task)
			_ = repo.UpdateTaskStatus(ctx, id, productenrich.TaskStatusProcessing)
			done <- struct{}{}
		}(i)
	}
	for range n {
		<-done
	}
}
