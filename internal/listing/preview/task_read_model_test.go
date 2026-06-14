package preview

import (
	"slices"
	"testing"
	"time"

	"task-processor/internal/catalog"
)

func TestBuildTaskReadModel(t *testing.T) {
	t.Parallel()

	createdAt := time.Now()
	updatedAt := createdAt.Add(2 * time.Minute)
	preview := BuildTaskReadModel(TaskReadModelInput{
		Task: TaskShellInput{
			TaskID:           "task-7",
			Status:           "completed",
			SelectedPlatform: "shein",
			ResultPlatforms:  []string{"shein", "amazon"},
			RequestPlatforms: []string{"temu"},
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		},
		ReadModel: ReadModelInput{
			NeedsReview: true,
			Attachment: &AttachmentInput{
				CatalogProduct: &catalog.Product{Title: "Wireless Earbuds"},
			},
			Overview: &HeaderInput{
				Country:       "US",
				StatusMessage: "ready",
			},
		},
	})
	if preview == nil {
		t.Fatal("preview = nil")
	}
	if preview.TaskID != "task-7" || preview.Status != "completed" {
		t.Fatalf("preview = %+v", preview)
	}
	if !preview.NeedsReview {
		t.Fatal("NeedsReview = false, want true")
	}
	if !slices.Equal(preview.Platforms, []string{"shein", "amazon"}) {
		t.Fatalf("platforms = %#v", preview.Platforms)
	}
	if preview.Attachment == nil || preview.Attachment.CatalogProduct == nil || preview.Attachment.CatalogProduct.Title != "Wireless Earbuds" {
		t.Fatalf("attachment = %+v", preview.Attachment)
	}
	if preview.Overview == nil || preview.Overview.Country != "US" || preview.Overview.StatusMessage != "ready" {
		t.Fatalf("overview = %+v", preview.Overview)
	}
	if preview.CompletedAt == nil || !preview.CompletedAt.Equal(updatedAt) {
		t.Fatalf("completedAt = %+v", preview.CompletedAt)
	}
}
