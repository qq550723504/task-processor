package listingkit

import assetgeneration "task-processor/internal/asset/generation"

type platformAssetDispatchTaskMergePhase struct{}

func buildPlatformAssetDispatchTaskMergePhase() *platformAssetDispatchTaskMergePhase {
	return &platformAssetDispatchTaskMergePhase{}
}

func (p *platformAssetDispatchTaskMergePhase) run(
	generationTasks []assetgeneration.Task,
	dispatchTasks []assetgeneration.Task,
) []assetgeneration.Task {
	if p == nil || len(dispatchTasks) == 0 {
		return generationTasks
	}
	return mergeGenerationTasks(generationTasks, dispatchTasks)
}
