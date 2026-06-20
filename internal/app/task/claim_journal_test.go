package task

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/model"
)

func TestTaskFetcherRecoverInterruptedClaims(t *testing.T) {
	journalPath := filepath.Join(t.TempDir(), "claims.json")
	journal := NewClaimJournal(journalPath)
	if err := journal.Record(ClaimJournalEntry{
		TaskID:     9001,
		ClaimedAt:  time.Now().Add(-time.Minute),
		FromStatus: model.TaskStatusPending,
		ProductID:  "P-9001",
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	client := &stubImportTaskStatusClient{}
	fetcher := &TaskFetcher{
		claimJournal: journal,
		statusServiceFactory: func(component string) *taskstatus.Service {
			return taskstatus.NewService(component, func() taskstatus.ImportTaskStatusClient {
				return client
			})
		},
	}

	if err := fetcher.recoverInterruptedClaims(); err != nil {
		t.Fatalf("recoverInterruptedClaims returned error: %v", err)
	}
	if len(client.updates) != 1 {
		t.Fatalf("recoverInterruptedClaims updates=%d, want 1", len(client.updates))
	}
	if client.updates[0].Status != model.TaskStatusPendingRetry.Int16() {
		t.Fatalf("recovered status=%d, want %d", client.updates[0].Status, model.TaskStatusPendingRetry.Int16())
	}
	entries, err := journal.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("journal entries should be cleared after recovery, got %+v", entries)
	}
}

func TestRemoveProcessingTaskAlsoRemovesClaimJournalEntry(t *testing.T) {
	journalPath := filepath.Join(t.TempDir(), "claims.json")
	journal := NewClaimJournal(journalPath)
	if err := journal.Record(ClaimJournalEntry{
		TaskID:     9002,
		ClaimedAt:  time.Now(),
		FromStatus: model.TaskStatusPending,
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	fetcher := &TaskFetcher{
		processingTasks: map[string]time.Time{"9002": time.Now()},
		claimJournal:    journal,
	}

	fetcher.RemoveProcessingTask("9002")

	if _, exists := fetcher.processingTasks["9002"]; exists {
		t.Fatal("RemoveProcessingTask should remove local processing mark")
	}
	entries, err := journal.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("journal entries should be removed after completion, got %+v", entries)
	}
}

func TestTaskStatusRecoveryInputUsesFallbackMessage(t *testing.T) {
	input := taskStatusRecoveryInput(ClaimJournalEntry{TaskID: 77})
	if input.TaskID != 77 {
		t.Fatalf("TaskID=%d, want 77", input.TaskID)
	}
	if input.Status != model.TaskStatusPendingRetry {
		t.Fatalf("Status=%v, want pending_retry", input.Status)
	}
	if input.ErrorMessage == "" {
		t.Fatal("ErrorMessage should not be empty")
	}
}

func TestNewClaimJournalUsesDefaultPath(t *testing.T) {
	journal := NewClaimJournal("")
	if journal == nil || journal.path == "" {
		t.Fatal("NewClaimJournal should set a default path")
	}
	if filepath.Base(journal.path) != defaultClaimJournalFile {
		t.Fatalf("journal path=%s, want base %s", journal.path, defaultClaimJournalFile)
	}
}

func TestClaimJournalRemoveMissingFile(t *testing.T) {
	journal := NewClaimJournal(filepath.Join(t.TempDir(), "missing.json"))
	if err := journal.Remove(1); err != nil {
		t.Fatalf("Remove on missing file returned error: %v", err)
	}
	if _, err := os.Stat(journal.path); err != nil {
		t.Fatalf("expected journal file to exist after remove, got error: %v", err)
	}
}

func TestRecoveryPreservesExistingErrorMessage(t *testing.T) {
	entry := ClaimJournalEntry{
		TaskID:       88,
		ErrorMessage: "processor crashed",
	}
	input := taskStatusRecoveryInput(entry)
	if input.ErrorMessage != "processor crashed" {
		t.Fatalf("ErrorMessage=%q, want processor crashed", input.ErrorMessage)
	}
}

