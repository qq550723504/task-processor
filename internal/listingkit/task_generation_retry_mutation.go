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
		return mergeGenerationTasks(existingTasks, nil)
	}

	updatedTasks := mergeGenerationTasks(existingTasks, dispatchResult.Tasks)
	if inventory == nil {
		return updatedTasks
	}

	retriedTargets := listinggeneration.TaskTargets(selectedTasks)
	inventory.Records = listinggeneration.ReplaceGeneratedAssetsForTargets(inventory.Records, retriedTargets, dispatchResult.Assets)
	inventory.Summary = asset.RebuildInventorySummary(inventory)
	return updatedTasks
}
