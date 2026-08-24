package listingkit

import (
	asset "task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	listinggeneration "task-processor/internal/listingkit/generation"
)

type retryGenerationMutationPhase struct{}

func buildRetryGenerationMutationPhase() *retryGenerationMutationPhase {
	return &retryGenerationMutationPhase{}
}

func (p *retryGenerationMutationPhase) run(
	inventory *asset.Inventory,
	existingTasks []assetgeneration.Task,
	selectedTasks []assetgeneration.Task,
	dispatchResult *assetgeneration.Result,
) []assetgeneration.Task {
	if dispatchResult == nil {
		return assetgeneration.MergeTasks(existingTasks, nil)
	}

	updatedTasks := assetgeneration.MergeTasks(existingTasks, dispatchResult.Tasks)
	if inventory == nil {
		return updatedTasks
	}

	retriedTargets := listinggeneration.TaskTargets(completedRetryTasks(dispatchResult.Tasks))
	inventory.Records = listinggeneration.ReplaceGeneratedAssetsForTargets(inventory.Records, retriedTargets, dispatchResult.Assets)
	inventory.Summary = asset.RebuildInventorySummary(inventory)
	return updatedTasks
}

func completedRetryTasks(tasks []assetgeneration.Task) []assetgeneration.Task {
	completed := make([]assetgeneration.Task, 0, len(tasks))
	for _, task := range tasks {
		if task.ExecutionStatus == "completed" {
			completed = append(completed, task)
		}
	}
	return completed
}
