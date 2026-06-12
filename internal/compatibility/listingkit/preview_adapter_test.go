package listingkit

import (
	"testing"
	"time"

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
		CreatedAt:        createdAt,
		CompletedAt:      &completedAt,
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
