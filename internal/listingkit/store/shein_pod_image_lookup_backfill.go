package store

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
)

const defaultSheinPODImageLookupBackfillBatchSize = 200

func BackfillSheinPODImageLookupIndexes(ctx context.Context, db *gorm.DB, batchSize int) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database is nil")
	}
	if batchSize <= 0 {
		batchSize = defaultSheinPODImageLookupBackfillBatchSize
	}

	var processed int64
	cursor := ""
	for {
		var tasks []listingkit.Task
		query := db.WithContext(ctx).
			Order("id ASC").
			Limit(batchSize)
		if cursor != "" {
			query = query.Where("id > ?", cursor)
		}
		if err := query.Find(&tasks).Error; err != nil {
			return processed, fmt.Errorf("load listingkit task batch: %w", err)
		}
		if len(tasks) == 0 {
			return processed, nil
		}

		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for i := range tasks {
				var current listingkit.Task
				if err := tx.WithContext(ctx).
					Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("id = ?", tasks[i].ID).
					First(&current).Error; err != nil {
					return fmt.Errorf("reload listingkit task %q: %w", tasks[i].ID, err)
				}
				if err := syncSheinPODImageLookupIndex(ctx, tx, &current); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return processed, fmt.Errorf("sync POD image lookup index batch: %w", err)
		}

		processed += int64(len(tasks))
		cursor = tasks[len(tasks)-1].ID
		if len(tasks) < batchSize {
			return processed, nil
		}
	}
}
