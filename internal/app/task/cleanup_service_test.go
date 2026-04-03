package task

import (
	"path/filepath"
	"testing"
	"time"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/model"
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

func TestCleanupServiceRecoversClaimedTaskAfterForceThreshold(t *testing.T) {
	journalPath := filepath.Join(t.TempDir(), "claims.json")
	journal := NewClaimJournal(journalPath)
	claimedAt := time.Now().Add(-35 * time.Minute)
	if err := journal.Record(ClaimJournalEntry{
		TaskID:     8001,
		ClaimedAt:  claimedAt,
		FromStatus: model.TaskStatusPending,
		ProductID:  "P-8001",
	}); err != nil {
		t.Fatalf("record journal failed: %v", err)
	}

	client := &stubImportTaskStatusClient{}
	fetcher := &TaskFetcher{
		processingTasks: map[string]time.Time{
			"8001": claimedAt,
		},
		claimJournal: journal,
		statusServiceFactory: func(component string) *taskstatus.Service {
			return taskstatus.NewService(component, func() taskstatus.ImportTaskStatusClient {
				return client
			})
		},
	}
	service := NewCleanupService(fetcher, nil)

	service.performCleanup()

	if len(client.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(client.updates))
	}
	if client.updates[0].Status != model.TaskStatusPendingRetry.Int16() {
		t.Fatalf("status=%d, want %d", client.updates[0].Status, model.TaskStatusPendingRetry.Int16())
	}
	if _, exists := fetcher.processingTasks["8001"]; exists {
		t.Fatal("processing task should be removed after automatic recovery")
	}
	entries, err := journal.LoadAll()
	if err != nil {
		t.Fatalf("load journal failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("journal should be empty after recovery, got %+v", entries)
	}
}

func TestCleanupServiceDoesNotRecoverTaskWithoutClaimJournal(t *testing.T) {
	fetcher := &TaskFetcher{
		processingTasks: map[string]time.Time{
			"8002": time.Now().Add(-35 * time.Minute),
		},
		claimJournal: NewClaimJournal(filepath.Join(t.TempDir(), "claims.json")),
	}
	service := NewCleanupService(fetcher, nil)

	service.performCleanup()

	if _, exists := fetcher.processingTasks["8002"]; !exists {
		t.Fatal("task without claim journal should not be auto recovered")
	}
}
