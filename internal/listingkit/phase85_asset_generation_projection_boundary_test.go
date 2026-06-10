package listingkit

import "testing"

func TestAssetGenerationProjectionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("result_generation_decorator_delegates_bundle_projection_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "asset_generation_projection.go", "decorateListingKitResultGeneration")
		callNames := readNamedFunctionCallNames(t, "asset_generation_projection.go", "decorateListingKitResultGeneration")

		assertSourceContainsAll(t, source, []string{
			"projection := buildAssetGenerationProjection(result, tasks)",
			"result.AssetGenerationTasks = projection.Tasks",
			"result.AssetGenerationSummary = projection.Summary",
			"result.AssetGenerationQueue = projection.Queue",
			"result.AssetGenerationOverview = projection.Overview",
		})
		assertSourceExcludesAll(t, source, []string{
			"result.AssetGenerationTasks = cloneGenerationTasks(tasks)",
			"result.AssetGenerationSummary = buildAssetGenerationSummary(tasks)",
			"result.AssetGenerationQueue = buildGenerationWorkQueue(result)",
			"result.AssetGenerationOverview = buildAssetGenerationOverview(result.AssetGenerationQueue)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildAssetGenerationProjection",
		})
	})

	t.Run("preview_and_export_decorators_delegate_asset_generation_bundle_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		previewSource := readNamedFunctionSource(t, "service_task_preview_facade.go", "GetTaskPreview")
		previewCalls := readNamedFunctionCallNames(t, "service_task_preview_facade.go", "GetTaskPreview")
		exportSource := readNamedFunctionSource(t, "service_task_export.go", "GetTaskExport")
		exportCalls := readNamedFunctionCallNames(t, "service_task_export.go", "GetTaskExport")

		assertSourceContainsAll(t, previewSource, []string{
			"projection := buildAssetGenerationProjection(task.Result, tasks)",
			"preview.AssetGenerationSummary = projection.Summary",
			"preview.AssetGenerationTasks = projection.Tasks",
			"preview.AssetGenerationQueue = projection.Queue",
			"preview.AssetGenerationOverview = projection.Overview",
		})
		assertSourceExcludesAll(t, previewSource, []string{
			"preview.AssetGenerationSummary = buildAssetGenerationSummary(tasks)",
			"preview.AssetGenerationTasks = append([]assetgeneration.Task(nil), tasks...)",
			"preview.AssetGenerationQueue = buildGenerationWorkQueue(withListingKitResultGeneration(task.Result, tasks))",
			"preview.AssetGenerationOverview = buildAssetGenerationOverview(preview.AssetGenerationQueue)",
		})
		assertFunctionCallsContainAll(t, previewCalls, []string{
			"buildAssetGenerationProjection",
		})

		assertSourceContainsAll(t, exportSource, []string{
			"projection := buildAssetGenerationProjection(task.Result, tasks)",
			"export.AssetGenerationSummary = projection.Summary",
			"export.AssetGenerationTasks = projection.Tasks",
			"export.AssetGenerationQueue = projection.Queue",
			"export.AssetGenerationOverview = projection.Overview",
		})
		assertSourceExcludesAll(t, exportSource, []string{
			"export.AssetGenerationSummary = buildAssetGenerationSummary(tasks)",
			"export.AssetGenerationTasks = append([]assetgeneration.Task(nil), tasks...)",
			"export.AssetGenerationQueue = buildGenerationWorkQueue(withListingKitResultGeneration(task.Result, tasks))",
			"export.AssetGenerationOverview = buildAssetGenerationOverview(export.AssetGenerationQueue)",
		})
		assertFunctionCallsContainAll(t, exportCalls, []string{
			"buildAssetGenerationProjection",
		})
	})

	t.Run("shared_projection_seam_owns_tasks_summary_queue_and_overview_contract", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "asset_generation_projection.go", "buildAssetGenerationProjection")
		callNames := readNamedFunctionCallNames(t, "asset_generation_projection.go", "buildAssetGenerationProjection")

		assertSourceContainsAll(t, source, []string{
			"summary := buildAssetGenerationSummary(tasks)",
			"clonedTasks := cloneGenerationTasks(tasks)",
			"queueResult.AssetGenerationTasks = cloneGenerationTasks(tasks)",
			"queueResult.AssetGenerationSummary = summary",
			"queue := buildGenerationWorkQueue(queueResult)",
			"Overview: buildAssetGenerationOverview(queue)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildAssetGenerationSummary",
			"cloneGenerationTasks",
			"buildGenerationWorkQueue",
			"buildAssetGenerationOverview",
		})
	})
}
