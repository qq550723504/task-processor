package listingkit

import assetgeneration "task-processor/internal/asset/generation"

type assetGenerationProjection struct {
	Tasks    []assetgeneration.Task
	Summary  *AssetGenerationSummary
	Queue    *GenerationWorkQueue
	Overview *AssetGenerationOverview
}

func buildAssetGenerationProjection(result *ListingKitResult, tasks []assetgeneration.Task) *assetGenerationProjection {
	summary := buildAssetGenerationSummary(tasks)
	clonedTasks := cloneGenerationTasks(tasks)

	queueResult := &ListingKitResult{}
	if result != nil {
		*queueResult = *result
	}
	queueResult.AssetGenerationTasks = cloneGenerationTasks(tasks)
	queueResult.AssetGenerationSummary = summary

	queue := buildGenerationWorkQueue(queueResult)
	return &assetGenerationProjection{
		Tasks:    clonedTasks,
		Summary:  summary,
		Queue:    queue,
		Overview: buildAssetGenerationOverview(queue),
	}
}

func decorateListingKitResultGeneration(result *ListingKitResult, tasks []assetgeneration.Task) {
	if result == nil {
		return
	}
	projection := buildAssetGenerationProjection(result, tasks)
	applyAssetGenerationProjectionToResult(result, projection)
}

func applyAssetGenerationProjectionToResult(result *ListingKitResult, projection *assetGenerationProjection) {
	if result == nil || projection == nil {
		return
	}
	result.AssetGenerationTasks = projection.Tasks
	result.AssetGenerationSummary = projection.Summary
	result.AssetGenerationQueue = projection.Queue
	result.AssetGenerationOverview = projection.Overview
}

func applyAssetGenerationProjectionToPreview(preview *ListingKitPreview, projection *assetGenerationProjection) {
	if preview == nil || projection == nil {
		return
	}
	preview.AssetGenerationTasks = projection.Tasks
	preview.AssetGenerationSummary = projection.Summary
	preview.AssetGenerationQueue = projection.Queue
	preview.AssetGenerationOverview = projection.Overview
}

func applyAssetGenerationProjectionToExport(export *ListingKitExport, projection *assetGenerationProjection) {
	if export == nil || projection == nil {
		return
	}
	export.AssetGenerationTasks = projection.Tasks
	export.AssetGenerationSummary = projection.Summary
	export.AssetGenerationQueue = projection.Queue
	export.AssetGenerationOverview = projection.Overview
}

func withListingKitResultGeneration(result *ListingKitResult, tasks []assetgeneration.Task) *ListingKitResult {
	if result == nil {
		return &ListingKitResult{
			AssetGenerationTasks: cloneGenerationTasks(tasks),
		}
	}
	cloned := *result
	decorateListingKitResultGeneration(&cloned, tasks)
	return &cloned
}
