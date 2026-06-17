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
}
