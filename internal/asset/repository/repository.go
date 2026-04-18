package repository

import (
	"context"
	"errors"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

var ErrInventoryNotFound = errors.New("asset inventory not found")

type Repository interface {
	SaveInventory(ctx context.Context, inventory *asset.Inventory) error
	GetInventory(ctx context.Context, ref asset.InventoryRef) (*asset.Inventory, error)
	SaveGenerationTasks(ctx context.Context, taskID string, tasks []assetgeneration.Task) error
	ListGenerationTasks(ctx context.Context, taskID string) ([]assetgeneration.Task, error)
}
