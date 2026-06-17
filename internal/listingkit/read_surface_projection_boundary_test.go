package listingkit

import "testing"

func TestReadSurfaceProjectionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("preview_attachment_applies_projection_through_adapter", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_result_attachment.go", "attachListingKitPreviewResult")
		callNames := readNamedFunctionCallNames(t, "preview_result_attachment.go", "attachListingKitPreviewResult")

		assertSourceContainsAll(t, source, []string{
			"projection := buildListingKitPreviewProjection(task, selectedPlatform)",
			"applyListingKitPreviewProjection(preview, projection)",
		})
		assertSourceExcludesAll(t, source, []string{
			"preview.Overview = projection.overview",
			"preview.Catalog = projection.catalog",
			"preview.AssetGenerationOverview = projection.generationOverview",
			"preview.RevisionHistory = projection.revisionHistory",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildListingKitPreviewProjection",
			"applyListingKitPreviewProjection",
		})
	})

	t.Run("export_builder_applies_projection_through_adapter", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "export_builder.go", "buildListingKitExport")
		callNames := readNamedFunctionCallNames(t, "export_builder.go", "buildListingKitExport")

		assertSourceContainsAll(t, source, []string{
			"projection := buildListingKitExportProjection(task.Result, selectedPlatform)",
			"applyListingKitExportProjection(export, projection)",
		})
		assertSourceExcludesAll(t, source, []string{
			"export.CatalogProduct = projection.catalog",
			"export.AssetBundle = projection.assetBundle",
			"export.AssetGenerationOverview = projection.generationOverview",
			"export.Overview = projection.overview",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildListingKitExportProjection",
			"applyListingKitExportProjection",
		})
	})

	t.Run("preview_projection_carries_attachment_fields_as_bundle", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_result_projection.go", "buildListingKitPreviewProjection")
		applySource := readNamedFunctionSource(t, "preview_result_projection.go", "applyListingKitPreviewProjection")
		fileSource := readTaskGenerationSourceFile(t, "preview_result_projection.go")

		assertSourceContainsAll(t, fileSource, []string{
			"type listingKitPreviewProjectionAttachment struct {",
			"attachment      listingKitPreviewProjectionAttachment",
		})
		assertSourceContainsAll(t, source, []string{
			"attachment: listingKitPreviewProjectionAttachment{",
			"catalog:             legacyBase.Catalog",
			"assets:              legacyBase.Assets",
			"assetInventory:      legacyBase.AssetInventory",
			"assetRenderPreviews: readProjection.AssetRenderPreviews",
			"platformPreviews:    readProjection.PlatformAssetRenderPreviews",
			"generationQueue:     readProjection.AssetGenerationQueue",
			"generationOverview:  readProjection.AssetGenerationOverview",
		})
		assertSourceContainsAll(t, applySource, []string{
			"preview.Catalog = projection.attachment.catalog",
			"preview.Assets = projection.attachment.assets",
			"preview.AssetInventory = projection.attachment.assetInventory",
			"preview.AssetRenderPreviews = projection.attachment.assetRenderPreviews",
			"preview.PlatformAssetRenderPreviews = projection.attachment.platformPreviews",
			"preview.AssetGenerationQueue = projection.attachment.generationQueue",
			"preview.AssetGenerationOverview = projection.attachment.generationOverview",
		})
		assertSourceExcludesAll(t, fileSource, []string{
			"preview.Catalog = projection.catalog",
			"preview.Assets = projection.assets",
			"preview.AssetInventory = projection.assetInventory",
			"preview.AssetRenderPreviews = projection.assetRenderPreviews",
			"preview.PlatformAssetRenderPreviews = projection.platformPreviews",
			"preview.AssetGenerationQueue = projection.generationQueue",
			"preview.AssetGenerationOverview = projection.generationOverview",
		})
	})
}
