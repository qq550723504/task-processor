package store_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/store"
)

func TestTaskRepositoryCanonicalSubjectAccessMatrix(t *testing.T) {
	db := newCanonicalSubjectDB(t)
	repository := store.NewTaskRepository(db)
	reader, ok := repository.(listingtask.CanonicalSubjectReader)
	if !ok {
		t.Fatal("NewTaskRepository result does not implement CanonicalSubjectReader")
	}

	tasks := []*listingkit.Task{
		canonicalSubjectTask("task-owner", "tenant-a", "owner-a", "product-a", 7),
		canonicalSubjectTask("task-other", "tenant-a", "owner-b", "product-b", 8),
		canonicalSubjectTask("task-foreign", "tenant-b", "owner-b", "product-c", 9),
	}
	for _, task := range tasks {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	tests := []struct {
		name    string
		actor   listingtask.Actor
		taskID  string
		wantErr error
	}{
		{name: "owner", actor: actor("tenant-a", "owner-a", "listingkit_operator"), taskID: "task-owner"},
		{name: "cross owner", actor: actor("tenant-a", "owner-a", "listingkit_operator"), taskID: "task-other", wantErr: listingtask.ErrCanonicalSubjectNotFound},
		{name: "tenant admin same tenant", actor: actor("tenant-a", "admin-a", "listingkit_admin"), taskID: "task-other"},
		{name: "platform admin same tenant", actor: actor("tenant-a", "platform-a", "platform_admin"), taskID: "task-other"},
		{name: "platform admin cross tenant", actor: actor("tenant-a", "platform-a", "platform_admin"), taskID: "task-foreign", wantErr: listingtask.ErrCanonicalSubjectNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, err := reader.ReadCanonicalSubject(context.Background(), tt.actor, tt.taskID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReadCanonicalSubject() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && subject.TaskID != tt.taskID {
				t.Fatalf("subject.TaskID = %q, want %q", subject.TaskID, tt.taskID)
			}
		})
	}

	subject, err := reader.ReadCanonicalSubject(context.Background(), actor("tenant-a", "owner-a", "listingkit_operator"), "task-owner")
	if err != nil {
		t.Fatalf("owner read: %v", err)
	}
	if subject.TenantID != "tenant-a" || subject.OwnerUserID != "owner-a" || subject.ProductKey != "product-a" || subject.SnapshotVersion != 7 {
		t.Fatalf("subject = %#v", subject)
	}
	if subject.Source == nil || subject.Source.Key != "source-product-a" || subject.Source.Platform != "1688" {
		t.Fatalf("subject.Source = %#v", subject.Source)
	}
}

func TestTaskRepositoryCanonicalSubjectLegacyOwnerAndDefensiveSource(t *testing.T) {
	db := newCanonicalSubjectDB(t)
	reader := store.NewTaskRepository(db).(listingtask.CanonicalSubjectReader)
	task := canonicalSubjectTask("task-legacy", "tenant-a", "", "product-legacy", 0)
	task.Request.UserID = "legacy-owner"
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create legacy task: %v", err)
	}

	subject, err := reader.ReadCanonicalSubject(context.Background(), actor("tenant-a", "legacy-owner", "listingkit_operator"), task.ID)
	if err != nil {
		t.Fatalf("ReadCanonicalSubject(): %v", err)
	}
	if subject.OwnerUserID != "legacy-owner" {
		t.Fatalf("OwnerUserID = %q", subject.OwnerUserID)
	}
	subject.Source.Key = "mutated"

	again, err := reader.ReadCanonicalSubject(context.Background(), actor("tenant-a", "legacy-owner", "listingkit_operator"), task.ID)
	if err != nil {
		t.Fatalf("ReadCanonicalSubject() again: %v", err)
	}
	if again.Source == nil || again.Source.Key != "source-product-legacy" {
		t.Fatalf("repository leaked mutable source pointer: %#v", again.Source)
	}
}

func TestTaskRepositoryCanonicalSubjectRejectsInvalidOrUnready(t *testing.T) {
	db := newCanonicalSubjectDB(t)
	reader := store.NewTaskRepository(db).(listingtask.CanonicalSubjectReader)

	if _, err := reader.ReadCanonicalSubject(context.Background(), listingtask.Actor{}, "task-missing"); !errors.Is(err, listingtask.ErrInvalidActor) {
		t.Fatalf("invalid actor error = %v", err)
	}
	if _, err := reader.ReadCanonicalSubject(context.Background(), actor("tenant-a", "owner-a", "listingkit_operator"), " "); !errors.Is(err, listingtask.ErrInvalidTaskID) {
		t.Fatalf("invalid task error = %v", err)
	}

	task := canonicalSubjectTask("task-unready", "tenant-a", "owner-a", "", 0)
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create unready task: %v", err)
	}
	if _, err := reader.ReadCanonicalSubject(context.Background(), actor("tenant-a", "owner-a", "listingkit_operator"), task.ID); !errors.Is(err, listingtask.ErrCanonicalSubjectNotReady) {
		t.Fatalf("unready error = %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.ReadCanonicalSubject(canceled, actor("tenant-a", "owner-a", "listingkit_operator"), task.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
}

func newCanonicalSubjectDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: "file:" + t.Name() + "?mode=memory&cache=shared"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func canonicalSubjectTask(taskID, tenantID, userID, productKey string, version uint64) *listingkit.Task {
	return &listingkit.Task{
		ID:                    taskID,
		TenantID:              tenantID,
		UserID:                userID,
		SourceSnapshotVersion: version,
		Status:                core.TaskStatusPending,
		Request: &listingkit.GenerateRequest{
			TenantID:   tenantID,
			UserID:     userID,
			ProductKey: productKey,
			Source: &listingkit.SourceReference{
				Key:      "source-" + productKey,
				Type:     "crawler",
				Platform: "1688",
				ID:       "123",
				URL:      "https://detail.1688.com/offer/123.html",
			},
		},
	}
}

func actor(tenantID, userID, role string) listingtask.Actor {
	return listingtask.Actor{TenantID: tenantID, UserID: userID, Roles: []string{role}}
}
