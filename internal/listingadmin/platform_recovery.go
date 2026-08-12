package listingadmin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/model"
)

const sheinPlatformRecoveryStoreID int64 = 986

// PlatformRecoveryRequest explicitly scopes the one-time platform cleanup.
// Execute is false by default and therefore never writes data.
type PlatformRecoveryRequest struct {
	StoreID       int64
	ExpectedCount int
	Execute       bool
}

// PlatformRecoveryReport records exactly which rows the recovery inspected and
// whether it changed them. It contains no task payloads.
type PlatformRecoveryReport struct {
	SelectedIDs    []int64
	UpdatedIDs     []int64
	ConflictingIDs []int64
	DryRun         bool
}

// RecoverStore986PlatformCohort normalizes only the confirmed pending
// Amazon-to-SHEIN cohort. It never creates work, changes task status, or
// publishes messages; dispatch remains owned by the existing control plane.
func (r *GormImportTaskRepository) RecoverStore986PlatformCohort(ctx context.Context, req PlatformRecoveryRequest) (PlatformRecoveryReport, error) {
	report := PlatformRecoveryReport{DryRun: !req.Execute}
	if r == nil || r.db == nil {
		return report, errors.New("import task repository database is not configured")
	}
	if req.StoreID != sheinPlatformRecoveryStoreID {
		return report, fmt.Errorf("platform recovery is restricted to store_id %d", sheinPlatformRecoveryStoreID)
	}
	if req.ExpectedCount <= 0 {
		return report, errors.New("platform recovery expected_count must be positive")
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var selected []listingProductImportTask
		if err := tx.Table((listingProductImportTask{}).TableName()).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("store_id = ? AND deleted = ?", req.StoreID, 0).
			Where("status = ?", model.TaskStatusPending.Int16()).
			Where("LOWER(TRIM(platform)) = ?", "amazon").
			Where("LOWER(TRIM(source_platform)) = ?", "amazon").
			Where("LOWER(TRIM(target_platform)) = ?", "shein").
			Order("id ASC").
			Find(&selected).Error; err != nil {
			return err
		}
		report.SelectedIDs = importTaskIDs(selected)
		if len(report.SelectedIDs) != req.ExpectedCount {
			return fmt.Errorf("platform recovery selected %d rows, expected %d", len(report.SelectedIDs), req.ExpectedCount)
		}

		conflictingIDs, err := findPlatformRecoveryConflicts(tx, req.StoreID, selected)
		if err != nil {
			return err
		}
		report.ConflictingIDs = conflictingIDs
		if len(conflictingIDs) > 0 {
			return fmt.Errorf("platform recovery found %d conflicting active task rows", len(conflictingIDs))
		}
		if !req.Execute {
			return nil
		}

		result := tx.Table((listingProductImportTask{}).TableName()).
			Where("id IN ?", report.SelectedIDs).
			Updates(map[string]any{
				"platform":        "amazon",
				"source_platform": "amazon",
				"target_platform": "shein",
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(report.SelectedIDs)) {
			return fmt.Errorf("platform recovery updated %d rows, selected %d", result.RowsAffected, len(report.SelectedIDs))
		}
		report.UpdatedIDs = append([]int64(nil), report.SelectedIDs...)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	return report, err
}

func importTaskIDs(rows []listingProductImportTask) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func findPlatformRecoveryConflicts(tx *gorm.DB, storeID int64, selected []listingProductImportTask) ([]int64, error) {
	if len(selected) == 0 {
		return nil, nil
	}
	selectedKeys := make(map[platformRecoveryKey]struct{}, len(selected))
	for _, row := range selected {
		selectedKeys[platformRecoveryKey{ProductID: row.ProductID, Region: row.Region, StoreID: row.StoreID}] = struct{}{}
	}

	var active []listingProductImportTask
	if err := tx.Table((listingProductImportTask{}).TableName()).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("store_id = ? AND deleted = ?", storeID, 0).
		Where("LOWER(TRIM(COALESCE(NULLIF(TRIM(target_platform), ''), platform))) = ?", "shein").
		Order("id ASC").
		Find(&active).Error; err != nil {
		return nil, err
	}

	byKey := make(map[platformRecoveryKey][]int64, len(selectedKeys))
	for _, row := range active {
		key := platformRecoveryKey{ProductID: row.ProductID, Region: row.Region, StoreID: row.StoreID}
		if _, selected := selectedKeys[key]; selected {
			byKey[key] = append(byKey[key], row.ID)
		}
	}
	var conflicts []int64
	for _, ids := range byKey {
		if len(ids) > 1 {
			conflicts = append(conflicts, ids...)
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i] < conflicts[j] })
	return conflicts, nil
}

type platformRecoveryKey struct {
	ProductID string
	Region    string
	StoreID   int64
}

func (r PlatformRecoveryReport) String() string {
	return fmt.Sprintf("dry_run=%t selected_ids=%s updated_ids=%s conflicting_ids=%s", r.DryRun, joinPlatformRecoveryIDs(r.SelectedIDs), joinPlatformRecoveryIDs(r.UpdatedIDs), joinPlatformRecoveryIDs(r.ConflictingIDs))
}

func joinPlatformRecoveryIDs(ids []int64) string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, fmt.Sprintf("%d", id))
	}
	return strings.Join(values, ",")
}
