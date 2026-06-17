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
			"applyAssetGenerationProjectionToResult(result, projection)",
		})
		assertSourceExcludesAll(t, source, []string{
			"result.AssetGenerationTasks = cloneGenerationTasks(tasks)",
			"result.AssetGenerationSummary = buildAssetGenerationSummary(tasks)",
			"result.AssetGenerationQueue = buildGenerationWorkQueue(result)",
			"result.AssetGenerationOverview = buildAssetGenerationOverview(result.AssetGenerationQueue)",
			"result.AssetGenerationTasks = projection.Tasks",
			"result.AssetGenerationSummary = projection.Summary",
			"result.AssetGenerationQueue = projection.Queue",
			"result.AssetGenerationOverview = projection.Overview",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildAssetGenerationProjection",
			"applyAssetGenerationProjectionToResult",
		})
	})

	t.Run("preview_and_export_decorators_delegate_asset_generation_bundle_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		previewSource := readNamedFunctionSource(t, "task_preview_service.go", "GetTaskPreview")
		previewCalls := readNamedFunctionCallNames(t, "task_preview_service.go", "GetTaskPreview")
		previewFinalizeSource := readNamedFunctionSource(t, "task_preview_service_support.go", "finalizeTaskPreview")
		previewFinalizeCalls := readNamedFunctionCallNames(t, "task_preview_service_support.go", "finalizeTaskPreview")
		exportSource := readNamedFunctionSource(t, "task_export_service.go", "GetTaskExport")
		exportCalls := readNamedFunctionCallNames(t, "task_export_service.go", "GetTaskExport")

		assertSourceContainsAll(t, previewSource, []string{
			"return s.reader.GetTaskPreview(ctx, taskID, platform)",
		})
		assertSourceExcludesAll(t, previewSource, []string{
			"projection := buildAssetGenerationProjection(task.Result, tasks)",
		})
		assertFunctionCallsContainAll(t, previewCalls, []string{
			"GetTaskPreview",
		})

		assertSourceContainsAll(t, previewFinalizeSource, []string{
			"projection := buildAssetGenerationProjection(task.Result, tasks)",
			"applyAssetGenerationProjectionToPreview(preview, projection)",
		})
		assertSourceExcludesAll(t, previewFinalizeSource, []string{
			"preview.AssetGenerationSummary = buildAssetGenerationSummary(tasks)",
			"preview.AssetGenerationTasks = append([]assetgeneration.Task(nil), tasks...)",
			"preview.AssetGenerationQueue = buildGenerationWorkQueue(withListingKitResultGeneration(task.Result, tasks))",
			"preview.AssetGenerationOverview = buildAssetGenerationOverview(preview.AssetGenerationQueue)",
			"preview.AssetGenerationSummary = projection.Summary",
			"preview.AssetGenerationTasks = projection.Tasks",
			"preview.AssetGenerationQueue = projection.Queue",
			"preview.AssetGenerationOverview = projection.Overview",
		})
		assertFunctionCallsContainAll(t, previewFinalizeCalls, []string{
			"buildAssetGenerationProjection",
			"applyAssetGenerationProjectionToPreview",
		})

		assertSourceContainsAll(t, exportSource, []string{
			"projection := buildAssetGenerationProjection(task.Result, tasks)",
			"applyAssetGenerationProjectionToExport(export, projection)",
		})
		assertSourceExcludesAll(t, exportSource, []string{
			"export.AssetGenerationSummary = buildAssetGenerationSummary(tasks)",
			"export.AssetGenerationTasks = append([]assetgeneration.Task(nil), tasks...)",
			"export.AssetGenerationQueue = buildGenerationWorkQueue(withListingKitResultGeneration(task.Result, tasks))",
			"export.AssetGenerationOverview = buildAssetGenerationOverview(export.AssetGenerationQueue)",
			"export.AssetGenerationSummary = projection.Summary",
			"export.AssetGenerationTasks = projection.Tasks",
			"export.AssetGenerationQueue = projection.Queue",
			"export.AssetGenerationOverview = projection.Overview",
		})
		assertFunctionCallsContainAll(t, exportCalls, []string{
			"buildAssetGenerationProjection",
			"applyAssetGenerationProjectionToExport",
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
