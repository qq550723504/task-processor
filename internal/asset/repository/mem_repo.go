package repository

import (
	"context"
	"sync"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

type MemRepository struct {
	mu          sync.RWMutex
	inventories map[string]*asset.Inventory
	tasks       map[string][]assetgeneration.Task
}

func NewMemRepository() *MemRepository {
	return &MemRepository{
		inventories: make(map[string]*asset.Inventory),
		tasks:       make(map[string][]assetgeneration.Task),
	}
}

func (r *MemRepository) SaveInventory(ctx context.Context, inventory *asset.Inventory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if inventory == nil {
		return nil
	}
	copied := *inventory
	r.inventories[inventory.Ref.TaskID] = &copied
	return nil
}

func (r *MemRepository) GetInventory(ctx context.Context, ref asset.InventoryRef) (*asset.Inventory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.inventories[ref.TaskID]
	if !ok {
		return nil, ErrInventoryNotFound
	}
	copied := *item
	return &copied, nil
}

func (r *MemRepository) SaveGenerationTasks(ctx context.Context, taskID string, tasks []assetgeneration.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	normalized := normalizeGenerationTasks(taskID, tasks)
	r.tasks[taskID] = append([]assetgeneration.Task(nil), normalized...)
	return nil
}

func (r *MemRepository) ListGenerationTasks(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := r.tasks[taskID]
	if len(items) == 0 {
		return nil, nil
	}
	return append([]assetgeneration.Task(nil), items...), nil
}
