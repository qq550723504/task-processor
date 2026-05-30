package listingkit

import (
	"context"

	asset "task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
)

type retryGenerationPersistPhase struct {
	assetRepo assetrepo.Repository
}

func buildRetryGenerationPersistPhase(assetRepo assetrepo.Repository) *retryGenerationPersistPhase {
	return &retryGenerationPersistPhase{assetRepo: assetRepo}
}

func (p *retryGenerationPersistPhase) run(ctx context.Context, taskID string, inventory *asset.Inventory, updatedTasks []assetgeneration.Task) error {
	if err := p.assetRepo.SaveInventory(ctx, inventory); err != nil {
		return err
	}
	if err := p.assetRepo.SaveGenerationTasks(ctx, taskID, updatedTasks); err != nil {
		return err
	}
	return nil
}
