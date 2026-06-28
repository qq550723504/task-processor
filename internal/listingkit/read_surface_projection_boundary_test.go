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

		source := readNamedFunctionSource(t, "preview_result_adapter.go", "buildListingKitPreviewProjection")
		applySource := readNamedFunctionSource(t, "preview_result_adapter.go", "applyListingKitPreviewProjection")
		adapterSource := readTaskGenerationSourceFile(t, "preview_result_adapter.go")

		assertSourceContainsAll(t, adapterSource, []string{
			"func buildListingKitPreviewProjection(",
			"type listingKitPreviewProjectionAttachment struct {",
			"attachment      listingKitPreviewProjectionAttachment",
		})
		assertFileAbsent(t, "preview_result_projection.go")
		assertSourceContainsAll(t, source, []string{
			"domainProjection := buildPreviewDomainResultProjection(base)",
			"return adaptPreviewDomainResultProjection(domainProjection, readProjection, task.Result.RevisionHistory)",
		})
		assertSourceExcludesAll(t, source, []string{
			"attachment: listingKitPreviewProjectionAttachment{",
			"catalog:             adaptPreviewDomainCatalog(domainProjection.Attachment)",
			"assets:              adaptPreviewDomainAssets(domainProjection.Attachment)",
			"assetInventory:      adaptPreviewDomainAssetInventory(domainProjection.Attachment)",
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
	})

	t.Run("export_projection_carries_attachment_fields_as_bundle", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "export_result_projection.go", "buildListingKitExportProjection")
		applySource := readNamedFunctionSource(t, "export_result_projection.go", "applyListingKitExportProjection")
		fileSource := readTaskGenerationSourceFile(t, "export_result_projection.go")

		assertSourceContainsAll(t, fileSource, []string{
			"type listingKitExportProjectionAttachment struct {",
			"attachment listingKitExportProjectionAttachment",
		})
		assertSourceContainsAll(t, source, []string{
			"attachment: listingKitExportProjectionAttachment{",
			"catalog:             attachment.CatalogProduct",
			"assetBundle:         attachment.AssetBundle",
			"assetInventory:      attachment.AssetInventorySummary",
			"assetRenderPreviews: readProjection.AssetRenderPreviews",
			"platformPreviews:    readProjection.PlatformAssetRenderPreviews",
			"generationQueue:     readProjection.AssetGenerationQueue",
			"generationOverview:  readProjection.AssetGenerationOverview",
		})
		assertSourceContainsAll(t, applySource, []string{
			"export.CatalogProduct = projection.attachment.catalog",
			"export.AssetBundle = projection.attachment.assetBundle",
			"export.AssetInventorySummary = projection.attachment.assetInventory",
			"export.AssetRenderPreviews = projection.attachment.assetRenderPreviews",
			"export.PlatformAssetRenderPreviews = projection.attachment.platformPreviews",
			"export.AssetGenerationQueue = projection.attachment.generationQueue",
			"export.AssetGenerationOverview = projection.attachment.generationOverview",
		})
		assertSourceExcludesAll(t, fileSource, []string{
			"export.CatalogProduct = projection.catalog",
			"export.AssetBundle = projection.assetBundle",
			"export.AssetInventorySummary = projection.assetInventory",
			"export.AssetRenderPreviews = projection.assetRenderPreviews",
			"export.PlatformAssetRenderPreviews = projection.platformPreviews",
			"export.AssetGenerationQueue = projection.generationQueue",
			"export.AssetGenerationOverview = projection.generationOverview",
		})
	})
}
