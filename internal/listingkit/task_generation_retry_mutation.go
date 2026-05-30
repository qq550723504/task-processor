package listingkit

import (
	asset "task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
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
	dispatchTasks := []assetgeneration.Task(nil)
	dispatchAssets := []asset.AssetRecord(nil)
	if dispatchResult != nil {
		dispatchTasks = dispatchResult.Tasks
		dispatchAssets = dispatchResult.Assets
	}

	updatedTasks := mergeGenerationTasks(existingTasks, dispatchTasks)
	if inventory == nil {
		return updatedTasks
	}

	retriedTargets := generationTaskTargets(selectedTasks)
	inventory.Records = replaceGeneratedAssetsForTargets(inventory.Records, retriedTargets, dispatchAssets)
	inventory.Summary = rebuildInventorySummary(inventory)
	return updatedTasks
}
