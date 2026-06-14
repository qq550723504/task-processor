package listingkit

import (
	previewdomain "task-processor/internal/listing/preview"
)

type listingKitReadProjection struct {
	PreviewInput                previewdomain.ReadModelInput
	PlatformCards               []ListingKitPlatformCard
	AssetRenderPreviews         []AssetRenderPreview
	PlatformAssetRenderPreviews []PlatformAssetRenderPreviews
	AssetGenerationQueue        *GenerationWorkQueue
	AssetGenerationOverview     *AssetGenerationOverview
}

func (projection *listingKitReadProjection) previewDomainReadModelInput() previewdomain.ReadModelInput {
	if projection == nil {
		return previewdomain.ReadModelInput{}
	}
	return projection.PreviewInput
}

func buildPreviewDomainRevisionHistoryMetaInput(result *ListingKitResult) *previewdomain.RevisionHistoryMetaInput {
	if result == nil {
		return nil
	}
	total := result.RevisionHistoryTotal
	if total == 0 && len(result.RevisionHistory) > 0 {
		total = len(result.RevisionHistory)
	}
	return &previewdomain.RevisionHistoryMetaInput{
		TotalRecords:    total,
		ReturnedRecords: len(result.RevisionHistory),
		MaxRecords:      maxRevisionHistoryRecords,
	}
}
