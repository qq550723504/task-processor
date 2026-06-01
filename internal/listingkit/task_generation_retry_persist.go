package listingkit

import (
	"context"

	asset "task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

type retryGenerationPersistenceRepository interface {
	SaveInventory(ctx context.Context, inventory *asset.Inventory) error
	SaveGenerationTasks(ctx context.Context, taskID string, tasks []assetgeneration.Task) error
}

type retryGenerationPersistPhase struct {
	assetRepo retryGenerationPersistenceRepository
}

func buildRetryGenerationPersistPhase(assetRepo retryGenerationPersistenceRepository) *retryGenerationPersistPhase {
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
