package task

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
)

type stubImportTaskStatusClient struct {
	updateFn func(req *listingruntime.TaskStatusUpdate) error
	updates  []*listingruntime.TaskStatusUpdate
}

func (s *stubImportTaskStatusClient) UpdateTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if s.updateFn != nil {
		return s.updateFn(req)
	}

	copied := *req
	s.updates = append(s.updates, &copied)
	return nil
}

func newTestTaskFetcher(client taskstatus.ImportTaskStatusClient) *TaskFetcher {
	journalPath := filepath.Join(os.TempDir(), fmt.Sprintf("task-claim-service-test-%d.json", time.Now().UnixNano()))
	return &TaskFetcher{
		processingTasks: make(map[string]time.Time),
		claimJournal:    NewClaimJournal(journalPath),
		statusServiceFactory: func(component string) *taskstatus.Service {
			return taskstatus.NewService(component, func() taskstatus.ImportTaskStatusClient {
				return client
			})
		},
	}
}

func TestTaskClaimServiceClaimRejectsInvalidTransition(t *testing.T) {
	fetcher := newTestTaskFetcher(&stubImportTaskStatusClient{})
	service := NewTaskClaimService(fetcher)

	task := &ImportTaskRecord{
		ID:     101,
		Status: model.TaskStatusPublished.Int16(),
	}

	taskID, ok := service.Claim(task)
	if ok {
		t.Fatal("Claim should reject published task")
	}
	if taskID != "101" {
		t.Fatalf("Claim taskID = %s, want 101", taskID)
	}
	if len(fetcher.processingTasks) != 0 {
		t.Fatal("Claim should not mark invalid task as processing")
	}
}

func TestTaskClaimServiceClaimMarksProcessing(t *testing.T) {
	client := &stubImportTaskStatusClient{}
	fetcher := newTestTaskFetcher(client)
	service := NewTaskClaimService(fetcher)

	task := &ImportTaskRecord{
		ID:     202,
		Status: model.TaskStatusPending.Int16(),
	}

	taskID, ok := service.Claim(task)
	if !ok {
		t.Fatal("Claim should accept pending task")
	}
	if taskID != "202" {
		t.Fatalf("Claim taskID = %s, want 202", taskID)
	}
	if _, exists := fetcher.processingTasks["202"]; !exists {
		t.Fatal("Claim should mark task as processing")
	}
	if len(client.updates) != 1 {
		t.Fatalf("Claim updates = %d, want 1", len(client.updates))
	}
	if client.updates[0].Status != model.TaskStatusProcessing.Int16() {
		t.Fatalf("Claim update status = %d, want %d", client.updates[0].Status, model.TaskStatusProcessing.Int16())
	}
	entries, err := fetcher.claimJournal.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].TaskID != 202 {
		t.Fatalf("claim journal entries = %+v, want task 202", entries)
	}
}

func TestTaskClaimServiceClaimRollsBackWhenRemoteUpdateFails(t *testing.T) {
	client := &stubImportTaskStatusClient{
		updateFn: func(req *listingruntime.TaskStatusUpdate) error {
			return fmt.Errorf("management unavailable")
		},
	}
	fetcher := newTestTaskFetcher(client)
	service := NewTaskClaimService(fetcher)

	taskID, ok := service.Claim(&ImportTaskRecord{
		ID:     303,
		Status: model.TaskStatusPending.Int16(),
	})
	if ok {
		t.Fatal("Claim should fail when remote processing update fails")
	}
	if taskID != "303" {
		t.Fatalf("Claim taskID = %s, want 303", taskID)
	}
	if _, exists := fetcher.processingTasks["303"]; exists {
		t.Fatal("Claim should roll back local processing mark when remote update fails")
	}
}

func TestTaskClaimServiceClaimRollsBackWhenJournalPersistFails(t *testing.T) {
	client := &stubImportTaskStatusClient{}

	journalParent := filepath.Join(t.TempDir(), "broken")
	if err := os.WriteFile(journalParent, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	fetcher := &TaskFetcher{
		processingTasks: make(map[string]time.Time),
		claimJournal:    NewClaimJournal(filepath.Join(journalParent, "claims.json")),
		statusServiceFactory: func(component string) *taskstatus.Service {
			return taskstatus.NewService(component, func() taskstatus.ImportTaskStatusClient {
				return client
			})
		},
	}
	service := NewTaskClaimService(fetcher)

	taskID, ok := service.Claim(&ImportTaskRecord{
		ID:     404,
		Status: model.TaskStatusPending.Int16(),
	})
	if ok {
		t.Fatal("Claim should fail when journal persist fails")
	}
	if taskID != "404" {
		t.Fatalf("Claim taskID = %s, want 404", taskID)
	}
	if _, exists := fetcher.processingTasks["404"]; exists {
		t.Fatal("Claim should roll back local processing mark when journal persist fails")
	}
	if len(client.updates) != 2 {
		t.Fatalf("Claim updates = %d, want 2", len(client.updates))
	}
	if client.updates[0].Status != model.TaskStatusProcessing.Int16() {
		t.Fatalf("First update status = %d, want %d", client.updates[0].Status, model.TaskStatusProcessing.Int16())
	}
	if client.updates[1].Status != model.TaskStatusPending.Int16() {
		t.Fatalf("Second update status = %d, want %d", client.updates[1].Status, model.TaskStatusPending.Int16())
	}
}
