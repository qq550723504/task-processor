package listingkit

import (
	"testing"
	"time"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildListingKitPreviewFiltersSelectedPlatform(t *testing.T) {
	t.Parallel()

	now := time.Now()
	task := &Task{
		ID:        "task-preview-1",
		Status:    TaskStatusCompleted,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		Request: &GenerateRequest{
			Platforms: []string{"amazon", "shein", "temu"},
		},
		Result: &ListingKitResult{
			TaskID:    "task-preview-1",
			Status:    "completed",
			Platforms: []string{"amazon", "shein", "temu"},
			Country:   "US",
			Language:  "en_US",
			CatalogProduct: &catalog.Product{
				Title: "Wireless Earbuds",
				Brand: "DemoBrand",
			},
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{
					{ID: "asset-main", Kind: asset.KindMainImage, URL: "https://cdn.example.com/main.jpg"},
				},
			},
			Summary: &GenerationSummary{
				SourceType:   "text",
				ImageCount:   2,
				VariantCount: 3,
				NeedsReview:  true,
				Warnings:     []string{"需要确认 SHEIN 销售属性"},
			},
			Shein: &SheinPackage{
				SpuName:      "Wireless Earbuds",
				BrandName:    "DemoBrand",
				CategoryPath: []string{"Electronics", "Headphones"},
				CategoryID:   123,
				ReviewNotes:  []string{"确认主销售属性"},
				Inspection: &SheinInspection{
					NeedsReview: true,
					Summary:     []string{"销售属性仍需人工确认"},
				},
				RequestDraft: &SheinRequestDraft{
					SpuName: "Wireless Earbuds",
				},
				PreviewProduct: &sheinproduct.Product{
					SPUName: "Wireless Earbuds",
				},
			},
		},
	}

	preview, err := buildListingKitPreview(task, "shein")
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}

	if preview.SelectedPlatform != "shein" {
		t.Fatalf("selected platform = %q, want shein", preview.SelectedPlatform)
	}
	if preview.Catalog == nil || preview.Catalog.Title != "Wireless Earbuds" {
		t.Fatalf("catalog = %+v", preview.Catalog)
	}
	if preview.Assets == nil || len(preview.Assets.Assets) != 1 {
		t.Fatalf("assets = %+v", preview.Assets)
	}
	if preview.Shein == nil {
		t.Fatal("expected shein payload")
	}
	if preview.Amazon != nil || preview.Temu != nil || preview.Walmart != nil {
		t.Fatal("expected only selected platform payload")
	}
	if !preview.NeedsReview {
		t.Fatal("expected preview to require review")
	}
	if preview.Overview == nil || len(preview.Overview.PlatformCards) != 1 {
		t.Fatalf("overview cards = %+v, want single shein card", preview.Overview)
	}
	if preview.Shein.PreviewProduct == nil || preview.Shein.PreviewProduct.SPUName != "Wireless Earbuds" {
		t.Fatalf("unexpected shein preview product: %+v", preview.Shein.PreviewProduct)
	}
	if preview.Shein.SubmitReadiness == nil {
		t.Fatal("expected shein submit readiness")
	}
	if preview.Shein.SubmitReadiness.Status != "blocked" {
		t.Fatalf("submit readiness status = %q, want blocked", preview.Shein.SubmitReadiness.Status)
	}
	if preview.Shein.SubmitChecklist == nil {
		t.Fatal("expected shein submit checklist")
	}
	if len(preview.Shein.SubmitChecklist.Required) == 0 {
		t.Fatalf("submit checklist = %+v, want required items", preview.Shein.SubmitChecklist)
	}
	if preview.Shein.RepairCenter == nil || preview.Shein.RepairCenter.Stats == nil || preview.Shein.RepairCenter.Stats.TotalActions == 0 {
		t.Fatalf("repair center = %+v", preview.Shein.RepairCenter)
	}
	if preview.Shein.RepairCenter.PrimaryPlan == nil || preview.Shein.RepairCenter.ApplyQueue == nil || preview.Shein.RepairCenter.Session == nil {
		t.Fatalf("repair center plan = %+v", preview.Shein.RepairCenter)
	}
	if preview.Shein.WorkspaceOverview == nil || preview.Shein.WorkspaceOverview.PrimaryView == "" {
		t.Fatalf("workspace overview = %+v", preview.Shein.WorkspaceOverview)
	}
	if preview.Shein.WorkspaceOverview.ActiveSession == nil {
		t.Fatalf("workspace overview session = %+v", preview.Shein.WorkspaceOverview)
	}
	if preview.Shein.StatusOverview == nil {
		t.Fatal("expected shein status overview")
	}
	if preview.Shein.StatusOverview.Status != "blocked" {
		t.Fatalf("status overview status = %q, want blocked", preview.Shein.StatusOverview.Status)
	}
	if preview.Shein.EditorContext == nil {
		t.Fatal("expected shein editor context")
	}
}

func TestBuildListingKitPreviewRejectsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := buildListingKitPreview(&Task{ID: "task-preview-2"}, "ebay")
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
	if err != ErrUnsupportedPreviewPlatform {
		t.Fatalf("error = %v, want %v", err, ErrUnsupportedPreviewPlatform)
	}
}

func TestBuildListingKitPreviewIncludesRevisionHistoryMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()
	task := &Task{
		ID:        "task-preview-history",
		Status:    TaskStatusCompleted,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		Result: &ListingKitResult{
			TaskID:               "task-preview-history",
			Status:               "completed",
			RevisionHistoryTotal: maxRevisionHistoryRecords + 3,
			RevisionHistory: []ListingKitRevisionRecord{
				{Platform: "shein", UpdatedAt: now.Add(-2 * time.Minute)},
				{Platform: "shein", UpdatedAt: now.Add(-1 * time.Minute)},
			},
		},
	}

	preview, err := buildListingKitPreview(task, "")
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}
	if preview.RevisionHistoryMeta == nil {
		t.Fatal("expected revision history meta")
	}
	if preview.RevisionHistory[0].RevisionID == "" || preview.RevisionHistory[1].RevisionID == "" {
		t.Fatalf("revision history items missing ids: %+v", preview.RevisionHistory)
	}
	if preview.RevisionHistoryMeta.TotalRecords != maxRevisionHistoryRecords+3 {
		t.Fatalf("revision history meta = %+v", preview.RevisionHistoryMeta)
	}
	if preview.RevisionHistoryMeta.ReturnedRecords != 2 {
		t.Fatalf("returned records = %d, want 2", preview.RevisionHistoryMeta.ReturnedRecords)
	}
	if !preview.RevisionHistoryMeta.HasMore {
		t.Fatalf("revision history meta = %+v, want has_more", preview.RevisionHistoryMeta)
	}
}
