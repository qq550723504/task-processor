package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
)

const defaultSheinPODImageLookupBackfillBatchSize = 200

type SheinPODImageLookupBackfillSummary struct {
	Processed        int64
	SkippedMalformed int64
	MalformedRows    []SheinPODImageLookupBackfillMalformedRow
}

type SheinPODImageLookupBackfillMalformedRow struct {
	TaskID string
	Field  string
	Reason string
}

type sheinPODImageLookupBackfillTaskID struct {
	ID string
}

type sheinPODImageLookupBackfillTaskRow struct {
	ID                           string
	TenantID                     string
	UserID                       string
	Request                      sql.NullString
	SheinStoreResolutionSnapshot sql.NullString
	Status                       listingkit.TaskStatus
	Result                       sql.NullString
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

func BackfillSheinPODImageLookupIndexes(ctx context.Context, db *gorm.DB, batchSize int) (SheinPODImageLookupBackfillSummary, error) {
	var summary SheinPODImageLookupBackfillSummary
	if db == nil {
		return summary, fmt.Errorf("database is nil")
	}
	if batchSize <= 0 {
		batchSize = defaultSheinPODImageLookupBackfillBatchSize
	}

	cursor := ""
	for {
		var taskIDs []sheinPODImageLookupBackfillTaskID
		query := db.WithContext(ctx).
			Model(&listingkit.Task{}).
			Select("id").
			Order("id ASC").
			Limit(batchSize)
		if cursor != "" {
			query = query.Where("id > ?", cursor)
		}
		if err := query.Find(&taskIDs).Error; err != nil {
			return summary, fmt.Errorf("load listingkit task batch: %w", err)
		}
		if len(taskIDs) == 0 {
			return summary, nil
		}

		var batchProcessed int64
		var batchMalformed []SheinPODImageLookupBackfillMalformedRow
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for i := range taskIDs {
				var row sheinPODImageLookupBackfillTaskRow
				if err := tx.WithContext(ctx).
					Model(&listingkit.Task{}).
					Select(
						"id",
						"tenant_id",
						"user_id",
						"request",
						"shein_store_resolution_snapshot",
						"status",
						"result",
						"created_at",
						"updated_at",
					).
					Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("id = ?", taskIDs[i].ID).
					Take(&row).Error; err != nil {
					return fmt.Errorf("reload listingkit task %q: %w", taskIDs[i].ID, err)
				}
				current, malformed := decodeSheinPODImageLookupBackfillTask(row)
				if malformed != nil {
					batchMalformed = append(batchMalformed, *malformed)
					continue
				}
				if err := syncSheinPODImageLookupIndex(ctx, tx, current); err != nil {
					return err
				}
				batchProcessed++
			}
			return nil
		}); err != nil {
			return summary, fmt.Errorf("sync POD image lookup index batch: %w", err)
		}

		summary.Processed += batchProcessed
		summary.SkippedMalformed += int64(len(batchMalformed))
		summary.MalformedRows = append(summary.MalformedRows, batchMalformed...)
		cursor = taskIDs[len(taskIDs)-1].ID
		if len(taskIDs) < batchSize {
			return summary, nil
		}
	}
}

func decodeSheinPODImageLookupBackfillTask(row sheinPODImageLookupBackfillTaskRow) (*listingkit.Task, *SheinPODImageLookupBackfillMalformedRow) {
	task := &listingkit.Task{
		ID:        row.ID,
		TenantID:  row.TenantID,
		UserID:    row.UserID,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	for _, field := range []struct {
		name   string
		raw    sql.NullString
		target any
	}{
		{name: "request", raw: row.Request, target: &task.Request},
		{name: "shein_store_resolution_snapshot", raw: row.SheinStoreResolutionSnapshot, target: &task.SheinStoreResolutionSnapshot},
		{name: "result", raw: row.Result, target: &task.Result},
	} {
		if !field.raw.Valid {
			continue
		}
		if err := json.Unmarshal([]byte(field.raw.String), field.target); err != nil {
			return nil, &SheinPODImageLookupBackfillMalformedRow{
				TaskID: task.ID,
				Field:  field.name,
				Reason: "invalid_json",
			}
		}
	}
	return task, nil
}
