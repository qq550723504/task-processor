package listingkit

import (
	"testing"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	previewdomain "task-processor/internal/listing/preview"
)

func TestAdaptPreviewDomainResultProjectionPreservesListingKitPreviewExtras(t *testing.T) {
	t.Parallel()

	catalogProduct := &catalog.Product{Title: "Wireless Earbuds"}
	assetBundle := &asset.Bundle{Assets: []asset.Asset{{ID: "asset-1"}}}
	assetInventory := &asset.InventorySummary{TotalRecords: 1}
	queue := &GenerationWorkQueue{Summary: &GenerationWorkQueueSummary{TotalItems: 1}}
	overview := &AssetGenerationOverview{PrimaryAction: "Review Ready Assets"}
	revisionHistory := []ListingKitRevisionRecord{{RevisionID: "rev-1"}}

	projection := adaptPreviewDomainResultProjection(
		previewdomain.ResultProjection{
			NeedsReview: true,
			Attachment: &previewdomain.Attachment{
				CatalogProduct:        catalogProduct,
				AssetBundle:           assetBundle,
				AssetInventorySummary: assetInventory,
			},
			Overview: &previewdomain.Header{
				StatusMessage: "ready",
			},
			RevisionHistoryMeta: &previewdomain.RevisionHistoryMeta{
				TotalRecords:    1,
				ReturnedRecords: 1,
				MaxRecords:      20,
			},
		},
		&listingKitReadProjection{
			PlatformCards:               []ListingKitPlatformCard{{Platform: "shein", Status: "ready"}},
			AssetRenderPreviews:         []AssetRenderPreview{{AssetID: "asset-1"}},
			PlatformAssetRenderPreviews: []PlatformAssetRenderPreviews{{Platform: "shein"}},
			AssetGenerationQueue:        queue,
			AssetGenerationOverview:     overview,
		},
		revisionHistory,
	)

	if !projection.needsReview {
		t.Fatal("needsReview = false, want true")
	}
	if projection.overview == nil || projection.overview.StatusMessage != "ready" || len(projection.overview.PlatformCards) != 1 {
		t.Fatalf("overview = %+v, want status and platform cards", projection.overview)
	}
	if projection.attachment.catalog != catalogProduct || projection.attachment.assets != assetBundle || projection.attachment.assetInventory != assetInventory {
		t.Fatalf("attachment = %+v, want domain attachment references", projection.attachment)
	}
	if len(projection.attachment.assetRenderPreviews) != 1 || projection.attachment.assetRenderPreviews[0].AssetID != "asset-1" {
		t.Fatalf("asset render previews = %+v", projection.attachment.assetRenderPreviews)
	}
	if len(projection.attachment.platformPreviews) != 1 || projection.attachment.platformPreviews[0].Platform != "shein" {
		t.Fatalf("platform previews = %+v", projection.attachment.platformPreviews)
	}
	if projection.attachment.generationQueue != queue || projection.attachment.generationOverview != overview {
		t.Fatalf("generation extras = (%+v, %+v)", projection.attachment.generationQueue, projection.attachment.generationOverview)
	}
	if projection.revisionMeta == nil || projection.revisionMeta.TotalRecords != 1 {
		t.Fatalf("revision meta = %+v, want copied meta", projection.revisionMeta)
	}
	if len(projection.revisionHistory) != 1 || projection.revisionHistory[0].RevisionID != "rev-1" {
		t.Fatalf("revision history = %+v", projection.revisionHistory)
	}
}
