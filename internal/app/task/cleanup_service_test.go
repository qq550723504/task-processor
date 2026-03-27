package task

import (
	"testing"
	"time"
)

func TestCleanupServicePerformCleanupDoesNotRemoveProcessingTasks(t *testing.T) {
	fetcher := &TaskFetcher{
		processingTasks: map[string]time.Time{
			"task-1": time.Now().Add(-40 * time.Minute),
			"task-2": time.Now().Add(-20 * time.Minute),
		},
	}
	service := NewCleanupService(fetcher, nil)

	service.performCleanup()

	if len(fetcher.processingTasks) != 2 {
		t.Fatalf("performCleanup removed tasks unexpectedly, remaining=%d", len(fetcher.processingTasks))
	}
}

func TestCleanupServiceForceCleanupAllDoesNotRemoveProcessingTasks(t *testing.T) {
	fetcher := &TaskFetcher{
		processingTasks: map[string]time.Time{
			"task-3": time.Now().Add(-10 * time.Minute),
		},
	}
	service := NewCleanupService(fetcher, nil)

	candidates := service.ForceCleanupAll(5 * time.Minute)

	if candidates != 1 {
		t.Fatalf("ForceCleanupAll candidates=%d, want 1", candidates)
	}
	if len(fetcher.processingTasks) != 1 {
		t.Fatalf("ForceCleanupAll removed tasks unexpectedly, remaining=%d", len(fetcher.processingTasks))
	}
}
