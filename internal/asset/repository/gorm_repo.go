package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) SaveInventory(ctx context.Context, inventory *asset.Inventory) error {
	if inventory == nil {
		return nil
	}
	snapshot := &InventorySnapshot{
		TaskID:     inventory.Ref.TaskID,
		ProductKey: inventory.Ref.ProductKey,
		Payload:    inventory,
	}
	return r.db.WithContext(ctx).Save(snapshot).Error
}

func (r *GormRepository) GetInventory(ctx context.Context, ref asset.InventoryRef) (*asset.Inventory, error) {
	var snapshot InventorySnapshot
	if err := r.db.WithContext(ctx).Where("task_id = ?", ref.TaskID).First(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInventoryNotFound
		}
		return nil, err
	}
	return snapshot.Payload, nil
}

func (r *GormRepository) SaveGenerationTasks(ctx context.Context, taskID string, tasks []assetgeneration.Task) error {
	if taskID == "" {
		return nil
	}
	normalized := normalizeGenerationTasks(taskID, tasks)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", taskID).Delete(&GenerationTaskSnapshot{}).Error; err != nil {
			return err
		}
		for _, item := range normalized {
			snapshot := &GenerationTaskSnapshot{
				ID:       item.ID,
				TaskID:   taskID,
				Platform: item.Platform,
				RecipeID: item.RecipeID,
				Payload:  &item,
			}
			if err := tx.Save(snapshot).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormRepository) ListGenerationTasks(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
	var snapshots []GenerationTaskSnapshot
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("id asc").Find(&snapshots).Error; err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	out := make([]assetgeneration.Task, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.Payload == nil {
			continue
		}
		out = append(out, *snapshot.Payload)
	}
	return out, nil
}
