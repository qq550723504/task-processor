package asset

import (
	"testing"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
)

func TestBuildInventoryBuildsPersistentAssetRecords(t *testing.T) {
	t.Parallel()

	bundle := BuildBundle(&canonical.Product{
		Images: []canonical.Image{
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

func TestRebuildInventorySummaryCountsRecordOriginsAndRecipes(t *testing.T) {
	t.Parallel()

	summary := RebuildInventorySummary(&Inventory{Records: []AssetRecord{
		{ID: "source-1", Origin: OriginSource},
		{ID: "generated-1", Origin: OriginGenerated, RecipeID: "shein-main-model"},
		{ID: "derived-1", Origin: OriginDerived},
		{ID: "unknown-1"},
	}})

	if summary == nil {
		t.Fatal("RebuildInventorySummary() = nil, want summary")
	}
	if summary.TotalRecords != 4 || summary.SourceRecords != 1 || summary.GeneratedRecords != 1 || summary.DerivedRecords != 2 || summary.RecipeCount != 1 {
		t.Fatalf("RebuildInventorySummary() = %+v, want origin and recipe counts", summary)
	}
}

func TestRebuildBundleWithRecordsPreservesReviewStateAndCopiesRecordFields(t *testing.T) {
	t.Parallel()

	base := &Bundle{
		Assets: []Asset{{ID: "source-1", Kind: KindSourceImage, URL: "https://example.com/source.jpg"}},
		Review: &ReviewSummary{
			NeedsReview: true,
			Reasons:     []string{"manual_check"},
		},
	}
	metadata := map[string]string{"source_url": "https://example.com/source.jpg"}
	out := RebuildBundleWithRecords(base, []AssetRecord{{
		ID:           "generated-1",
		Kind:         KindSceneImage,
		URL:          "https://cdn.example.com/scene.jpg",
		Role:         "scene",
		RecipeID:     "shein-scene",
		Lineage:      &AssetLineage{SourceAssetIDs: []string{"source-1"}},
		Operations:   []string{"crop"},
		Labels:       []string{"scene"},
		PlatformTags: []string{"shein"},
		Width:        800,
		Height:       1200,
		Metadata:     metadata,
	}})

	if out == nil || len(out.Assets) != 2 {
		t.Fatalf("RebuildBundleWithRecords() = %+v, want base plus generated asset", out)
	}
	if out.Review == nil || !out.Review.NeedsReview {
		t.Fatalf("review = %+v, want preserved review state", out.Review)
	}
	generated := out.Assets[1]
	if generated.ID != "generated-1" || generated.SourceURL != "" || generated.SourceAssetIDs[0] != "source-1" || generated.Metadata["source_url"] != metadata["source_url"] {
		t.Fatalf("generated asset = %+v, want copied record fields without source_url promotion", generated)
	}
	if out.Stats == nil || out.Stats.TotalAssets != 2 || out.Stats.SourceAssets != 1 || out.Stats.GeneratedAssets != 1 {
		t.Fatalf("stats = %+v, want rebuilt stats", out.Stats)
	}
	metadata["source_url"] = "mutated"
	if generated.Metadata["source_url"] == "mutated" {
		t.Fatal("generated metadata should be defensively copied")
	}
}

func TestRebuildBundleFromInventoryPromotesSourceURLMetadata(t *testing.T) {
	t.Parallel()

	out := RebuildBundleFromInventory(&Bundle{
		Quality: &QualitySummary{OverallScore: 0.88},
	}, &Inventory{Records: []AssetRecord{{
		ID:       "generated-1",
		Kind:     KindSceneImage,
		URL:      "https://cdn.example.com/scene.jpg",
		Origin:   OriginGenerated,
		Metadata: map[string]string{"source_url": "https://example.com/source.jpg"},
	}}})

	if out == nil || len(out.Assets) != 1 {
		t.Fatalf("RebuildBundleFromInventory() = %+v, want inventory asset", out)
	}
	if out.Quality == nil || out.Quality.OverallScore != 0.88 {
		t.Fatalf("quality = %+v, want preserved bundle quality", out.Quality)
	}
	if out.Assets[0].SourceURL != "https://example.com/source.jpg" {
		t.Fatalf("source url = %q, want promoted metadata source_url", out.Assets[0].SourceURL)
	}
	if out.Stats == nil || out.Stats.GeneratedAssets != 1 {
		t.Fatalf("stats = %+v, want generated stats", out.Stats)
	}
}
