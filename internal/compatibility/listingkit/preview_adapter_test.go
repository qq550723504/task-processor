package listingkit

import (
	"testing"
	"time"

	legacylistingkit "task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

func TestAdaptLegacyPreviewShell(t *testing.T) {
	t.Parallel()

	completedAt := time.Now().Add(2 * time.Minute)
	createdAt := completedAt.Add(-5 * time.Minute)
	adapted := AdaptLegacyPreviewShell(&legacylistingkit.ListingKitPreview{
		TaskID:           "task-1",
		Status:           core.TaskStatusCompleted,
		SelectedPlatform: "shein",
		Platforms:        []string{"shein", "amazon"},
		NeedsReview:      true,
		Catalog:          &catalog.ProductSnapshot{Title: "Wireless Earbuds"},
		ApprovedAssetInventory: &productasset.ApprovedAssetInventory{
			Scope:  productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"},
			Assets: []productasset.ApprovedAsset{{ID: "asset-1", Role: productasset.RoleMain, URL: "https://example.test/main.jpg"}},
		},
		CreatedAt:   createdAt,
		CompletedAt: &completedAt,
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
				},
			},
		},
	})
	if adapted == nil {
		t.Fatal("adapted = nil")
	}
	if adapted.TaskID != "task-1" || adapted.Status != string(core.TaskStatusCompleted) {
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
	if adapted.Attachment.ApprovedAssetInventory == nil || len(adapted.Attachment.ApprovedAssetInventory.Assets) != 1 {
		t.Fatalf("adapted approved asset inventory = %+v", adapted.Attachment)
	}
	if len(adapted.Overview.PlatformCards) != 1 {
		t.Fatalf("platform cards = %+v", adapted.Overview.PlatformCards)
	}
	card := adapted.Overview.PlatformCards[0]
	if card.Platform != "shein" {
		t.Fatalf("adapted card = %+v", card)
	}
}

func TestAdaptLegacyPreviewShellNil(t *testing.T) {
	t.Parallel()

	if adapted := AdaptLegacyPreviewShell(nil); adapted != nil {
		t.Fatalf("adapted = %+v, want nil", adapted)
	}
}
