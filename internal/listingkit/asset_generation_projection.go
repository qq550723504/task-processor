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
