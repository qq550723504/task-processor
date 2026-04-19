package asset

import (
	"testing"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func TestBuildInventoryBuildsPersistentAssetRecords(t *testing.T) {
	t.Parallel()

	bundle := BuildBundle(&productenrich.CanonicalProduct{
		Images: []productenrich.CanonicalImage{
			{URL: "https://example.com/source-1.jpg", Role: "primary"},
			{URL: "https://example.com/source-2.jpg", Role: "gallery"},
		},
	}, &productimage.ImageProcessResult{
		MainImage:     &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg", SourceURL: "https://example.com/source-1.jpg"},
		WhiteBgImage:  &productimage.ImageAsset{URL: "https://cdn.example.com/white.jpg"},
		SubjectCutout: &productimage.ImageAsset{URL: "https://cdn.example.com/cutout.png"},
		GalleryImages: []productimage.ImageAsset{
			{URL: "https://cdn.example.com/gallery-1.jpg", SourceURL: "https://example.com/source-2.jpg"},
		},
		Review: &productimage.ReviewDecision{
			NeedsReview: true,
			Reasons:     []string{"主图带中文广告"},
		},
	})

	inventory := BuildInventory("task-1", bundle)
	if inventory == nil {
		t.Fatal("expected inventory")
	}
	if inventory.Summary == nil {
		t.Fatal("expected inventory summary")
	}
	if inventory.Summary.TotalRecords != 6 {
		t.Fatalf("summary = %+v, want 6 records", inventory.Summary)
	}
	if inventory.Summary.SourceRecords != 2 {
		t.Fatalf("summary = %+v, want 2 source records", inventory.Summary)
	}
	if inventory.Summary.DerivedRecords != 4 {
		t.Fatalf("summary = %+v, want 4 derived records", inventory.Summary)
	}
	if len(inventory.Records) != 6 {
		t.Fatalf("records = %+v, want 6", inventory.Records)
	}
	if inventory.Records[0].TaskID != "task-1" {
		t.Fatalf("record task id = %q, want task-1", inventory.Records[0].TaskID)
	}
	if inventory.Records[2].Version == nil || inventory.Records[2].Version.Number != 1 {
		t.Fatalf("record version = %+v, want version 1", inventory.Records[2].Version)
	}
	if inventory.Records[2].Lineage == nil || len(inventory.Records[2].Lineage.SourceAssetIDs) == 0 {
		t.Fatalf("record lineage = %+v, want source lineage", inventory.Records[2].Lineage)
	}
	if !inventory.Review.NeedsReview {
		t.Fatalf("review = %+v, want needs review", inventory.Review)
	}
}
