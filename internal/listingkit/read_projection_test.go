package listingkit

import (
	"testing"

	"task-processor/internal/catalog"
	previewdomain "task-processor/internal/listing/preview"
)

func TestBuildListingKitReadProjectionCombinesOverviewAndAttachment(t *testing.T) {
	t.Parallel()

	projection := buildListingKitReadProjection(&ListingKitResult{
		Country:        "US",
		Language:       "en_US",
		CatalogProduct: &catalog.Product{Title: "Wireless Earbuds"},
		ReviewReasons:  []string{"reason"},
		Summary:        &GenerationSummary{NeedsReview: true, SourceType: "text"},
		AssetRenderPreviews: []AssetRenderPreview{
			{AssetID: "asset-1"},
		},
	}, "")
	if projection == nil {
		t.Fatal("projection = nil")
	}
	if !projection.PreviewInput.NeedsReview {
		t.Fatal("PreviewInput.NeedsReview = false, want true")
	}
	if projection.PreviewInput.Overview == nil || projection.PreviewInput.Overview.Country != "US" || projection.PreviewInput.Overview.SourceType != "text" {
		t.Fatalf("Overview = %+v", projection.PreviewInput.Overview)
	}
	if projection.PreviewInput.Attachment == nil || projection.PreviewInput.Attachment.CatalogProduct == nil {
		t.Fatalf("Attachment = %+v", projection.PreviewInput.Attachment)
	}
	if len(projection.AssetRenderPreviews) != 1 {
		t.Fatalf("AssetRenderPreviews = %+v", projection.AssetRenderPreviews)
	}
}

func TestBuildProjectionConsumersFromReadProjection(t *testing.T) {
	t.Parallel()

	projection := buildListingKitReadProjection(&ListingKitResult{
		Country:        "US",
		Language:       "en_US",
		CatalogProduct: &catalog.Product{Title: "Wireless Earbuds"},
		ReviewReasons:  []string{"reason"},
		Summary: &GenerationSummary{
			NeedsReview:  true,
			SourceType:   "text",
			ImageCount:   2,
			VariantCount: 3,
		},
	}, "")

	header := buildPreviewHeaderFromReadProjection(projection)
	if header == nil || header.Country != "US" || header.ImageCount != 2 {
		t.Fatalf("header = %+v", header)
	}

	meta := buildListingKitExportMetaFromReadProjection(projection)
	if meta == nil || meta.Language != "en_US" || meta.VariantCount != 3 {
		t.Fatalf("meta = %+v", meta)
	}

	domainProjection := buildListingKitPreviewDomainProjectionFromReadProjection(projection)
	if domainProjection == nil || !domainProjection.NeedsReview {
		t.Fatalf("domain projection = %+v", domainProjection)
	}
	if domainProjection.Overview == nil || domainProjection.Overview.SourceType != "text" {
		t.Fatalf("domain projection overview = %+v", domainProjection.Overview)
	}
}

func TestReadProjectionOwnsPreviewDomainReadModelInput(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		RevisionHistoryTotal: 3,
		RevisionHistory: []ListingKitRevisionRecord{
			{RevisionID: "rev-1"},
			{RevisionID: "rev-2"},
		},
	}
	projection := &listingKitReadProjection{
		PreviewInput: previewdomain.ReadModelInput{
			NeedsReview: true,
			Overview: &previewdomain.HeaderInput{
				Country:       "US",
				Language:      "en_US",
				SourceType:    "text",
				ImageCount:    2,
				VariantCount:  3,
				ReviewReasons: []string{"reason"},
				PlatformCards: []previewdomain.PlatformCard{{Platform: "shein", Status: "blocked"}},
			},
			Attachment: &previewdomain.AttachmentInput{
				CatalogProduct: &catalog.Product{Title: "Wireless Earbuds"},
			},
		},
		PlatformCards: []ListingKitPlatformCard{{Platform: "shein", Status: "blocked"}},
	}
	projection.PreviewInput.RevisionHistoryMeta = buildPreviewDomainRevisionHistoryMetaInput(result)

	input := projection.previewDomainReadModelInput()
	if !input.NeedsReview {
		t.Fatal("NeedsReview = false, want true")
	}
	if input.Overview == nil || input.Overview.Country != "US" || len(input.Overview.PlatformCards) != 1 {
		t.Fatalf("overview input = %+v", input.Overview)
	}
	if input.Attachment == nil || input.Attachment.CatalogProduct == nil {
		t.Fatalf("attachment input = %+v", input.Attachment)
	}
	if input.RevisionHistoryMeta == nil || input.RevisionHistoryMeta.TotalRecords != 3 || input.RevisionHistoryMeta.ReturnedRecords != 2 {
		t.Fatalf("revision history input = %+v", input.RevisionHistoryMeta)
	}
}
