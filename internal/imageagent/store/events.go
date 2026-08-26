package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/imageagent"
)

func (r *gormRepository) AppendEvent(ctx context.Context, event imageagent.RunEvent) error {
	if err := validateEvent(event); err != nil {
		return err
	}
	scope := imageagent.RunScope{TenantID: event.TenantID, RunID: event.RunID}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.findRunForUpdate(ctx, tx, scope); err != nil {
			return err
		}
		nextCursor, err := nextEventCursor(ctx, tx, scope)
		if err != nil {
			return err
		}
		if event.Cursor < nextCursor {
			return imageagent.ErrRevisionConflict
		}
		row := eventRecord{TenantID: event.TenantID, RunID: event.RunID, Type: event.Type, Cursor: event.Cursor, ProjectionVersion: event.ProjectionVersion, Payload: append([]byte(nil), event.Payload...)}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("append image agent event: %w", err)
		}
		return nil
	})
}

func (r *gormRepository) AppendProjectionEvent(ctx context.Context, event imageagent.RunEvent) (imageagent.RunEvent, error) {
	if strings.TrimSpace(event.TenantID) == "" || strings.TrimSpace(event.RunID) == "" || strings.TrimSpace(event.Type) == "" {
		return imageagent.RunEvent{}, fmt.Errorf("event tenant, run, and type are required")
	}
	scope := imageagent.RunScope{TenantID: event.TenantID, RunID: event.RunID}
	stored := event
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.findRunForUpdate(ctx, tx, scope); err != nil {
			return err
		}
		cursor, err := nextEventCursor(ctx, tx, scope)
		if err != nil {
			return err
		}
		stored.Cursor = cursor
		stored.ProjectionVersion = cursor
		return tx.Create(&eventRecord{TenantID: stored.TenantID, RunID: stored.RunID, Type: stored.Type, Cursor: cursor, ProjectionVersion: cursor, Payload: append([]byte(nil), stored.Payload...)}).Error
	})
	if err != nil {
		return imageagent.RunEvent{}, fmt.Errorf("append image agent projection event: %w", err)
	}
	return stored, nil
}

func (r *gormRepository) ListEvents(ctx context.Context, scope imageagent.RunScope, afterCursor int64, limit int) ([]imageagent.RunEvent, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	if _, err := r.findRun(ctx, r.db, scope); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []imageagent.RunEvent{}, nil
	}
	var rows []eventRecord
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND cursor > ?", scope.TenantID, scope.RunID, afterCursor).Order("cursor ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list image agent events: %w", err)
	}
	events := make([]imageagent.RunEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, imageagent.RunEvent{TenantID: row.TenantID, RunID: row.RunID, Type: row.Type, Cursor: row.Cursor, ProjectionVersion: row.ProjectionVersion, Payload: append([]byte(nil), row.Payload...)})
	}
	return events, nil
}

func nextEventCursor(ctx context.Context, db *gorm.DB, scope imageagent.RunScope) (int64, error) {
	var latest eventRecord
	err := db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", scope.TenantID, scope.RunID).Order("cursor DESC").First(&latest).Error
	if err == nil {
		return latest.Cursor + 1, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 1, nil
	}
	return 0, fmt.Errorf("find latest image agent event cursor: %w", err)
}
