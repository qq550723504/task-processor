package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
)

type platformAssetDispatchPersistPhase struct {
	assetRepository assetrepo.Repository
}

func buildPlatformAssetDispatchPersistPhase(s *service) *platformAssetDispatchPersistPhase {
	return &platformAssetDispatchPersistPhase{assetRepository: resolveWorkflowAssetRepository(s)}
}

func (p *platformAssetDispatchPersistPhase) run(
	ctx context.Context,
	task *Task,
	final *ListingKitResult,
	persistedGenerationTasks []assetgeneration.Task,
) []assetgeneration.Task {
	decorateListingKitResultGeneration(final, persistedGenerationTasks)
	if p == nil || p.assetRepository == nil || len(persistedGenerationTasks) == 0 {
		return persistedGenerationTasks
	}
	if err := p.assetRepository.SaveGenerationTasks(ctx, task.ID, persistedGenerationTasks); err != nil {
		appendWarning(final, "asset generation task persistence failed: "+err.Error())
		newWorkflowRecorder(final).AddIssue(
			WorkflowIssueSeverityWarning,
			"asset_generation_platform",
			"asset_generation_task_persistence_failed",
			"Asset generation task persistence failed",
			err.Error(),
		)
	}
	return persistedGenerationTasks
}
