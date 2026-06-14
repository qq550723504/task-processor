package listingkit

import (
	"fmt"
	"time"

	previewdomain "task-processor/internal/listing/preview"
)

func buildListingKitExport(task *Task, selectedPlatform string) (*ListingKitExport, error) {
	if task == nil {
		return nil, ErrTaskNotFound
	}

	var ok bool
	selectedPlatform, ok = previewdomain.ValidateSelectedPlatform(selectedPlatform)
	if !ok {
		return nil, ErrUnsupportedPreviewPlatform
	}

	export := &ListingKitExport{
		TaskID:           task.ID,
		SelectedPlatform: selectedPlatform,
		Format:           "json",
		MimeType:         "application/json; charset=utf-8",
		FileName:         buildListingKitExportFileName(task.ID, selectedPlatform),
		GeneratedAt:      time.Now(),
		Platforms:        previewPlatforms(task),
	}

	if task.Result == nil {
		return export, nil
	}

	projection := buildListingKitExportProjection(task.Result, selectedPlatform)
	export.CatalogProduct = projection.catalog
	export.AssetBundle = projection.assetBundle
	export.AssetInventorySummary = projection.assetInventory
	export.AssetRenderPreviews = projection.assetRenderPreviews
	export.PlatformAssetRenderPreviews = projection.platformPreviews
	export.AssetGenerationQueue = projection.generationQueue
	export.AssetGenerationOverview = projection.generationOverview
	export.Overview = projection.overview

	return export, buildExportPlatformSections(task.Result, export, selectedPlatform)
}

func buildListingKitExportMeta(result *ListingKitResult, selectedPlatform string) *ListingKitExportMeta {
	projection := buildListingKitReadProjection(result, selectedPlatform)
	return buildListingKitExportMetaFromReadProjection(projection)
}

func buildListingKitExportFileName(taskID string, selectedPlatform string) string {
	scope := firstNonEmpty(selectedPlatform, "bundle")
	return fmt.Sprintf("listing-kit-%s-%s.json", taskID, scope)
}
