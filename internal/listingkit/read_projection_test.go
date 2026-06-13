package listingkit

import "testing"

func TestBuildListingKitReadProjectionCombinesOverviewAndAttachment(t *testing.T) {
	t.Parallel()

	projection := buildListingKitReadProjection(&ListingKitResult{
		Country:        "US",
		Language:       "en_US",
		CatalogProduct: effectiveCatalogProduct(&ListingKitResult{}),
		ReviewReasons:  []string{"reason"},
		Summary:        &GenerationSummary{NeedsReview: true, SourceType: "text"},
		AssetRenderPreviews: []AssetRenderPreview{
			{AssetID: "asset-1"},
		},
	}, "")
	if projection == nil {
		t.Fatal("projection = nil")
	}
	if !projection.NeedsReview {
		t.Fatal("NeedsReview = false, want true")
	}
	if projection.Overview == nil || projection.Overview.Country != "US" || projection.Overview.SourceType != "text" {
		t.Fatalf("Overview = %+v", projection.Overview)
	}
	if projection.Attachment == nil || len(projection.Attachment.AssetRenderPreviews) != 1 {
		t.Fatalf("Attachment = %+v", projection.Attachment)
	}
}

func TestBuildProjectionConsumersFromReadProjection(t *testing.T) {
	t.Parallel()

	projection := buildListingKitReadProjection(&ListingKitResult{
		Country:        "US",
		Language:       "en_US",
		CatalogProduct: effectiveCatalogProduct(&ListingKitResult{}),
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

	domainProjection := buildListingKitPreviewDomainProjectionFromReadProjection(&ListingKitResult{}, projection)
	if domainProjection == nil || !domainProjection.NeedsReview {
		t.Fatalf("domain projection = %+v", domainProjection)
	}
	if domainProjection.Overview == nil || domainProjection.Overview.SourceType != "text" {
		t.Fatalf("domain projection overview = %+v", domainProjection.Overview)
	}
}
