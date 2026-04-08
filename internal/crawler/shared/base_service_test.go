package shared

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type stubResultStore struct {
	mu     sync.Mutex
	values map[string]string
	setErr error
}

func newStubResultStore() *stubResultStore {
	return &stubResultStore{values: make(map[string]string)}
}

func (s *stubResultStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.values[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return value, nil
}

func (s *stubResultStore) Set(_ context.Context, key string, value string, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	return nil
}

func (s *stubResultStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
	return nil
}

func TestBaseServiceGetTaskFallsBackToSharedStore(t *testing.T) {
	store := newStubResultStore()

	var writer BaseService
	writer.ConfigureSharedResultStore(store, "crawler:test", time.Hour)
	result := NewCrawlerResult("task-1")
	if err := writer.StoreResult("task-1", result); err != nil {
		t.Fatalf("StoreResult() error = %v", err)
	}

	var reader BaseService
	reader.ConfigureSharedResultStore(store, "crawler:test", time.Hour)

	got, err := reader.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want task-1", got.TaskID)
	}
	if got.Status != StatusPending {
		t.Fatalf("Status = %q, want %q", got.Status, StatusPending)
	}
}

func TestBaseServiceUpdateResultReadsAndWritesSharedStore(t *testing.T) {
	store := newStubResultStore()

	var writer BaseService
	writer.ConfigureSharedResultStore(store, "crawler:test", time.Hour)
	result := NewCrawlerResult("task-2")
	if err := writer.StoreResult("task-2", result); err != nil {
		t.Fatalf("StoreResult() error = %v", err)
	}

	var updater BaseService
	updater.ConfigureSharedResultStore(store, "crawler:test", time.Hour)
	if err := updater.UpdateResult("task-2", func(r *CrawlerResult) {
		r.MarkFailed(fmt.Errorf("boom"))
	}); err != nil {
		t.Fatalf("UpdateResult() error = %v", err)
	}

	var reader BaseService
	reader.ConfigureSharedResultStore(store, "crawler:test", time.Hour)

	got, err := reader.GetTask("task-2")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("Status = %q, want %q", got.Status, StatusFailed)
	}
	if got.Error != "boom" {
		t.Fatalf("Error = %q, want boom", got.Error)
	}
}

func TestBaseServiceDeleteTaskRemovesSharedStore(t *testing.T) {
	store := newStubResultStore()

	var svc BaseService
	svc.ConfigureSharedResultStore(store, "crawler:test", time.Hour)
	if err := svc.StoreResult("task-3", NewCrawlerResult("task-3")); err != nil {
		t.Fatalf("StoreResult() error = %v", err)
	}
	svc.DeleteTask("task-3")

	if _, err := svc.GetTask("task-3"); err == nil {
		t.Fatal("GetTask() error = nil, want ErrTaskNotFound")
	}
}

func TestBaseServiceStoreResultReturnsErrorWhenSharedStoreWriteFails(t *testing.T) {
	store := newStubResultStore()
	store.setErr = fmt.Errorf("redis write failed")

	var svc BaseService
	svc.ConfigureSharedResultStore(store, "crawler:test", time.Hour)

	if err := svc.StoreResult("task-4", NewCrawlerResult("task-4")); err == nil {
		t.Fatal("StoreResult() error = nil, want write failure")
	}
}
