package store_test

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
)

func TestTaskRepositoryPersistsSourceReference(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	task := &listingkit.Task{
		ID:       "task-source-reference",
		TenantID: "tenant-a",
		UserID:   "user-a",
		Status:   listingkit.TaskStatusPending,
		Request: &listingkit.GenerateRequest{
			TenantID: "tenant-a",
			UserID:   "user-a",
			Source: &listingkit.SourceReference{
				Key:      "crawler:1688:888",
				Type:     "crawler",
				Platform: "1688",
				ID:       "888",
				URL:      "https://detail.1688.com/offer/888.html",
			},
		},
	}

	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Request == nil || got.Request.Source == nil {
		t.Fatalf("stored request source = %+v, want source reference", got.Request)
	}
	if got.Request.Source.Key != "crawler:1688:888" ||
		got.Request.Source.Type != "crawler" ||
		got.Request.Source.Platform != "1688" ||
		got.Request.Source.ID != "888" ||
		got.Request.Source.URL != "https://detail.1688.com/offer/888.html" {
		t.Fatalf("stored source = %+v, want persisted source identity", got.Request.Source)
	}
}
