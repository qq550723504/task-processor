package preview

import (
	"testing"

	"task-processor/internal/catalog"
)

func TestBuildReadModel(t *testing.T) {
	t.Parallel()

	preview := BuildReadModel(ReadModelInput{
		NeedsReview: true,
		Attachment: &AttachmentInput{
			CatalogProduct: &catalog.Product{Title: "Wireless Earbuds"},
		},
		Overview: &HeaderInput{
			Country:       "US",
			StatusMessage: "ready",
		},
		RevisionHistoryMeta: &RevisionHistoryMetaInput{
			TotalRecords:    4,
			ReturnedRecords: 2,
		},
	})
	if preview == nil {
		t.Fatal("preview = nil")
	}
	if !preview.NeedsReview {
		t.Fatal("NeedsReview = false, want true")
	}
	if preview.Attachment == nil || preview.Attachment.CatalogProduct == nil || preview.Attachment.CatalogProduct.Title != "Wireless Earbuds" {
		t.Fatalf("attachment = %+v", preview.Attachment)
	}
	if preview.Overview == nil || preview.Overview.Country != "US" || preview.Overview.StatusMessage != "ready" {
		t.Fatalf("overview = %+v", preview.Overview)
	}
	if preview.RevisionHistoryMeta == nil || preview.RevisionHistoryMeta.TotalRecords != 4 || preview.RevisionHistoryMeta.ReturnedRecords != 2 {
		t.Fatalf("revision history meta = %+v", preview.RevisionHistoryMeta)
	}
}
