package listingkit

import (
	"testing"
	"time"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	legacylistingkit "task-processor/internal/listingkit"
)

func TestAdaptLegacyPreviewShell(t *testing.T) {
	t.Parallel()

	completedAt := time.Now().Add(2 * time.Minute)
	createdAt := completedAt.Add(-5 * time.Minute)
	adapted := AdaptLegacyPreviewShell(&legacylistingkit.ListingKitPreview{
		TaskID:           "task-1",
		Status:           legacylistingkit.TaskStatusCompleted,
		SelectedPlatform: "shein",
		Platforms:        []string{"shein", "amazon"},
		NeedsReview:      true,
		Catalog:          &catalog.Product{Title: "Wireless Earbuds"},
		Assets:           &asset.Bundle{Assets: []asset.Asset{{ID: "asset-1"}}},
		AssetInventory:   &asset.InventorySummary{TotalRecords: 3},
		CreatedAt:        createdAt,
		CompletedAt:      &completedAt,
		RevisionHistoryMeta: &legacylistingkit.ListingKitRevisionHistoryMeta{
			TotalRecords:    8,
			ReturnedRecords: 3,
			HasMore:         true,
			IsTruncated:     true,
			MaxRecords:      20,
		},
		Overview: &legacylistingkit.ListingKitPreviewHeader{
			Country:       "US",
			Language:      "en",
			SourceType:    "amazon",
			ImageCount:    5,
			VariantCount:  2,
			StatusMessage: "ready",
			Warnings:      []string{"warn-1"},
			ReviewReasons: []string{"reason-1"},
			PlatformCards: []legacylistingkit.ListingKitPlatformCard{
				{
					Platform:              "shein",
					Status:                "ready",
					Summary:               "ok",
					NeedsReview:           true,
					PreviewableItems:      3,
					ApprovedSections:      1,
					DeferredSections:      1,
					ReviewPendingSections: 1,
					PrimaryActionKey:      "open",
					PrimaryCTAKind:        "review",
				},
			},
		},
	})
	if adapted == nil {
		t.Fatal("adapted = nil")
	}
	if adapted.TaskID != "task-1" || adapted.Status != string(legacylistingkit.TaskStatusCompleted) {
		t.Fatalf("adapted shell = %+v", adapted)
	}
	if adapted.Overview == nil {
		t.Fatal("adapted overview = nil")
	}
	if adapted.RevisionHistoryMeta == nil || adapted.RevisionHistoryMeta.TotalRecords != 8 {
		t.Fatalf("adapted revision history meta = %+v", adapted.RevisionHistoryMeta)
	}
	if adapted.Attachment == nil || adapted.Attachment.CatalogProduct == nil || adapted.Attachment.CatalogProduct.Title != "Wireless Earbuds" {
		t.Fatalf("adapted attachment = %+v", adapted.Attachment)
	}
	if adapted.Attachment.AssetBundle == nil || len(adapted.Attachment.AssetBundle.Assets) != 1 {
		t.Fatalf("adapted assets = %+v", adapted.Attachment)
	}
	if adapted.Attachment.AssetInventorySummary == nil || adapted.Attachment.AssetInventorySummary.TotalRecords != 3 {
		t.Fatalf("adapted asset inventory = %+v", adapted.Attachment)
	}
	if len(adapted.Overview.PlatformCards) != 1 {
		t.Fatalf("platform cards = %+v", adapted.Overview.PlatformCards)
	}
	card := adapted.Overview.PlatformCards[0]
	if card.Platform != "shein" || card.PrimaryCTAKind != "review" {
		t.Fatalf("adapted card = %+v", card)
	}
}

func TestAdaptLegacyPreviewShellNil(t *testing.T) {
	t.Parallel()

	if adapted := AdaptLegacyPreviewShell(nil); adapted != nil {
		t.Fatalf("adapted = %+v, want nil", adapted)
	}
}
